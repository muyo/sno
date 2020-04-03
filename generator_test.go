// +build !bench

package sno

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGenerator_NewNoOverflow(t *testing.T) {
	var (
		part    = Partition{255, 255}
		seqPool = uint16(MaxSequence / 2)
		seqMin  = seqPool
		seqMax  = 2*seqPool - 1

		// Scaled to not exceed bounds, otherwise we run into the seqOverflow race and order - which we
		// test for in here - becomes non-deterministic.
		sampleSize = int(seqPool)
		g, err     = NewGenerator(&GeneratorSnapshot{
			Partition:   part,
			SequenceMin: seqMin,
			SequenceMax: seqMax,
		}, nil)
	)

	if err != nil {
		t.Fatal(err)
	}

	ids := make([]ID, sampleSize)
	for i := 0; i < sampleSize; i++ {
		ids[i] = g.New(byte(i))
	}

	for i := 1; i < sampleSize; i++ {
		curID, prevID := ids[i], ids[i-1]

		seq := ids[i].Sequence()
		if seq > seqMax {
			t.Errorf("%d: sequence overflowing max boundary; max [%d], got [%d]", i, seqMin, seq)
		}

		if seq < seqMin {
			t.Errorf("%d: sequence underflowing min boundary; min [%d], got [%d]", i, seqMin, seq)
		}

		// We're expecting the time to increment and never more than by one time unit, since
		// we generated them in sequence.
		timeDiff := curID.Timestamp() - prevID.Timestamp()

		// Check if drift got applied in this edge case.
		if timeDiff < 0 && curID[4]&1 == 0 {
			t.Error("timestamp of next ID lower than previous and no tick-tock applied")
		}

		if timeDiff > TimeUnit {
			t.Error("timestamp diff between IDs is higher than by one time unit")
		}

		if prevID.Partition() != part {
			t.Errorf("%d: partition differs from generator's partition; expected [%d], got [%d]", i, part, prevID.Partition())
		}
	}
}

func TestGenerator_NewOverflows(t *testing.T) {
	var (
		part         = Partition{255, 255}
		seqPool      = 512
		seqOverflows = 16
		seqMin       = uint16(seqPool)
		seqMax       = uint16(2*seqPool - 1)
		sampleSize   = int(seqPool * seqOverflows)

		c       = make(chan *SequenceOverflowNotification)
		cc      = make(chan struct{})
		notesHi = new(int64)

		g, err = NewGenerator(&GeneratorSnapshot{
			Partition:   part,
			SequenceMin: seqMin,
			SequenceMax: seqMax,
		}, c)
	)

	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			select {
			case note := <-c:
				if note.Count > 0 {
					atomic.AddInt64(notesHi, 1)
				}
			case <-cc:
				return
			}
		}
	}()

	ids := make([]ID, sampleSize)
	for i := 0; i < sampleSize; i++ {
		ids[i] = g.New(byte(i))
	}

	close(cc)

	// TODO(alcore) The non-blocking writes are far from reliable. The notifications need a rework with
	// deep profiling.
	if atomic.LoadInt64(notesHi) < int64(seqOverflows)/2 {
		t.Errorf("expected at least [%d] overflow notification, got [%d]", seqOverflows/2, atomic.LoadInt64(notesHi))
	}

	timeDist := make(map[int64]int)

	for i := 0; i < sampleSize; i++ {
		id := ids[i]
		timeDist[id.Timestamp()]++

		seq := id.Sequence()
		if seq > seqMax {
			t.Errorf("%d: sequence overflowing max boundary; max [%d], got [%d]", i, seqMin, seq)
		}

		if seq < seqMin {
			t.Errorf("%d: sequence underflowing min boundary; min [%d], got [%d]", i, seqMin, seq)
		}

		if id.Partition() != part {
			t.Errorf("%d: partition differs from generator's partition; expected [%d], got [%d]", i, part, id.Partition())
		}
	}

	for tf, c := range timeDist {
		if c > seqPool {
			t.Errorf("count of IDs in the given timeframe exceeds pool; timestamp [%d], pool [%d], count [%d]", tf, seqPool, c)
		}
	}
}

func TestGenerator_NewTickTocks(t *testing.T) {
	g, ids := testGeneratorNewTickTocksSetup(t)
	t.Run("Tick", testGeneratorNewTickTocksTick(g, ids))
	t.Run("SafetySlumber", testGeneratorNewTickTocksSafetySlumber(g, ids))
	t.Run("Tock", testGeneratorNewTickTocksTock(g, ids))
	t.Run("Race", testGeneratorNewTickTocksRace(g, ids))
}

func testGeneratorNewTickTocksSetup(t *testing.T) (*Generator, []ID) {
	var (
		seqPool = 8096
		g, err  = NewGenerator(&GeneratorSnapshot{
			Partition:   Partition{255, 255},
			SequenceMin: uint16(seqPool),
			SequenceMax: uint16(2*seqPool - 1),
		}, nil)
	)
	if err != nil {
		t.Fatal(err)
	}

	return g, make([]ID, g.Cap())
}

func testGeneratorNewTickTocksTick(g *Generator, ids []ID) func(*testing.T) {
	return func(t *testing.T) {
		// First batch follows normal time progression.
		for i := 0; i < 512; i++ {
			ids[i] = g.New(255)
		}

		wall := snotime()
		atomic.StoreUint64(staticWallNow, wall-TimeUnit)

		// Swap out the time source. Next batch is supposed to set a drift, have their tick-tock bit
		// set to 1, and wallSafe on the generator must be set accordingly.
		snotime = staticTime

		if atomic.LoadUint32(&g.drifts) != 0 {
			t.Errorf("expected [0] drifts recorded, got [%d]", atomic.LoadUint32(&g.drifts))
		}

		if atomic.LoadUint64(&g.wallSafe) != 0 {
			t.Errorf("expected wallSafe to be [0], is [%d]", atomic.LoadUint64(&g.wallSafe))
		}

		for j := 512; j < 1024; j++ {
			ids[j] = g.New(255)
		}

		if atomic.LoadUint32(&g.drifts) != 1 {
			t.Errorf("expected [1] drift recorded, got [%d]", atomic.LoadUint32(&g.drifts))
		}

		if atomic.LoadUint64(&g.wallSafe) == atomic.LoadUint64(staticWallNow) {
			t.Errorf("expected wallSafe to be [%d], was [%d]", atomic.LoadUint64(staticWallNow), atomic.LoadUint64(&g.wallSafe))
		}

		for i := 0; i < 512; i++ {
			if ids[i][4]&1 != 0 {
				t.Errorf("%d: expected tick-tock bit to not be set, was set", i)
			}
		}

		for j := 512; j < 1024; j++ {
			if ids[j][4]&1 != 1 {
				t.Errorf("%d: expected tick-tock bit to be set, was not", j)
			}
		}

		snotime = snotimeReal
	}
}

func testGeneratorNewTickTocksSafetySlumber(g *Generator, ids []ID) func(*testing.T) {
	return func(t *testing.T) {
		// Multi-regression, checking on a single goroutine.
		atomic.AddUint64(staticWallNow, ^uint64(TimeUnit-1))

		// Use a clock where the first call will return the static clock times
		// but subsequent calls will return higher times. Since we didn't adjust the mono clock
		// at all insofar, it's currently 1 TimeUnit (first drift) behind wallSafe, which got set
		// during the initial drift. This is the time the next generation call(s) are supposed
		// to sleep, as we are simulating a multi-regression (into an unsafe past where can't
		// tick-tock again until reaching wallSafe).
		snotime = staticIncTime

		mono1 := monotime()
		id := g.New(255)
		if id[4]&1 != 1 {
			t.Errorf("expected tick-tock bit to be set, was not")
		}
		mono2 := monotime()

		monoDiff := mono2 - mono1

		// We had 2 regressions by 1 TimeUnit each, so sleep duration should've been roughly
		// the same since time was static (got incremented only after the sleep).
		if monoDiff < 2*TimeUnit {
			t.Errorf("expected to sleep for at least [%f]ns, took [%d] instead", 2*TimeUnit, monoDiff)
		} else if monoDiff > 3*TimeUnit {
			t.Errorf("expected to sleep for no more than [%f]ns, took [%d] instead", 3*TimeUnit, monoDiff)
		}

		if atomic.LoadUint32(&g.drifts) != 1 {
			t.Errorf("expected [1] drift recorded, got [%d]", atomic.LoadUint32(&g.drifts))
		}

		snotime = snotimeReal
	}
}

func testGeneratorNewTickTocksTock(g *Generator, ids []ID) func(*testing.T) {
	return func(t *testing.T) {
		// At this point we are going to simulate another drift, somewhere in the 'far' future,
		// with parallel load.
		snotime = staticTime
		atomic.AddUint64(staticWallNow, 100*TimeUnit)

		g.New(255) // Updates wallHi

		// Regress again. Not adjusting mono clock - calls below are supposed to simply drift - drift
		// count is supposed to end at 2 (since we're still using the same generator) and tick-tock
		// bit is supposed to be unset.
		atomic.AddUint64(staticWallNow, ^uint64(2*TimeUnit-1))

		var (
			batchCount = 4
			batchSize  = g.Cap() / batchCount
			wg         sync.WaitGroup
		)

		wg.Add(batchCount)

		for i := 0; i < batchCount; i++ {
			go func(mul int) {
				for i := mul * batchSize; i < mul*batchSize+batchSize; i++ {
					ids[i] = g.New(255)
				}
				wg.Done()
			}(i)
		}

		wg.Wait()

		if atomic.LoadUint32(&g.drifts) != 2 {
			t.Errorf("expected [2] drifts recorded, got [%d]", atomic.LoadUint32(&g.drifts))
		}

		for i := 0; i < g.Cap(); i++ {
			if ids[i][4]&1 != 0 {
				t.Errorf("%d: expected tick-tock bit to not be set, was set", i)
			}
		}

		snotime = snotimeReal
	}
}

func testGeneratorNewTickTocksRace(g *Generator, ids []ID) func(*testing.T) {
	return func(*testing.T) {
		snotime = staticTime

		atomic.AddUint64(staticWallNow, 100*TimeUnit)
		g.New(255)
		atomic.AddUint64(staticWallNow, ^uint64(TimeUnit-1))

		var (
			wgOuter sync.WaitGroup
			wgInner sync.WaitGroup
		)
		wgOuter.Add(1000)

		wgInner.Add(1000)
		for i := 0; i < 1000; i++ {
			go func() {
				wgInner.Done()
				wgInner.Wait()
				for i := 0; i < 2; i++ {
					_ = g.New(byte(i))
				}
				wgOuter.Done()
			}()
		}
		wgOuter.Wait()

		snotime = snotimeReal
	}
}

func TestGenerator_NewGeneratorRestoreRegressions(t *testing.T) {
	// First one we simply check that the times get applied at all. We get rid of the time
	// added while simulating the last drift.
	g, err := NewGenerator(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Reset the static clock.
	wall := snotime()
	snotime = staticTime
	atomic.StoreUint64(staticWallNow, wall)

	// Simulate a regression.
	g.New(255)
	atomic.AddUint64(staticWallNow, ^uint64(TimeUnit-1))
	g.New(255)

	snapshot := g.Snapshot()

	g, err = NewGenerator(&snapshot, nil)
	if err != nil {
		t.Fatal(err)
	}

	if uint64(snapshot.WallSafe) != atomic.LoadUint64(&g.wallSafe) {
		t.Errorf("expected [%d], got [%d]", snapshot.WallSafe, atomic.LoadUint64(&g.wallSafe))
	}

	if uint64(snapshot.WallHi) != atomic.LoadUint64(&g.wallHi) {
		t.Errorf("expected [%d], got [%d]", snapshot.WallHi, atomic.LoadUint64(&g.wallHi))
	}

	// Second test, with a snapshot taken "in the future" (relative to current wall clock time).
	wall = snotimeReal()
	atomic.StoreUint64(staticWallNow, wall+100*TimeUnit)

	// Simulate another regression. Takes place in the future - we are going to take a snapshot
	// and create a generator using that snapshot, where the generator will use snotime (current time)
	// as comparison and is supposed to handle this as if it is in the past relative to the snapshot.
	g.New(255)
	atomic.AddUint64(staticWallNow, ^uint64(TimeUnit-1))
	g.New(255)

	snotime = snotimeReal

	snapshot = g.Snapshot()

	g, err = NewGenerator(&snapshot, nil)
	if err != nil {
		t.Fatal(err)
	}

	if uint64(snapshot.WallSafe) != atomic.LoadUint64(&g.wallSafe) {
		t.Errorf("expected [%d], got [%d]", snapshot.WallSafe, atomic.LoadUint64(&g.wallSafe))
	}

	if atomic.LoadUint64(&g.wallHi) != wall {
		t.Errorf("expected [%d], got [%d]", wall, atomic.LoadUint64(&g.wallHi))
	}
}

func TestGenerator_NewWithTimeOverflows(t *testing.T) {
	var (
		part         = Partition{255, 255}
		seqPool      = 12
		seqOverflows = 4
		seqMin       = uint16(seqPool)
		seqMax       = uint16(2*seqPool - 1)
		sampleSize   = int(seqPool * seqOverflows)

		g, err = NewGenerator(&GeneratorSnapshot{
			Partition:   part,
			SequenceMin: seqMin,
			SequenceMax: seqMax,
		}, nil)
	)

	if err != nil {
		t.Fatal(err)
	}

	tn := time.Now()
	pool := g.Cap()

	ids := make([]ID, sampleSize)
	for i := 0; i < sampleSize; i++ {
		ids[i] = g.NewWithTime(byte(i), tn)
	}

	timeDist := make(map[int64]int)

	for i, s := 0, 0; i < sampleSize; i, s = i+1, s+1 {
		id := ids[i]
		timeDist[id.Timestamp()]++

		seq := id.Sequence()
		if seq > seqMax {
			t.Errorf("%d: sequence overflowing max boundary; max [%d], got [%d]", i, seqMin, seq)
		}

		if seq < seqMin {
			t.Errorf("%d: sequence underflowing min boundary; min [%d], got [%d]", i, seqMin, seq)
		}

		// When we overflow with NewWithTime, the static sequence is supposed to roll over silently.
		if s == pool {
			s = 0
		} else if i > 0 && seq-ids[i-1].Sequence() != 1 {
			t.Errorf("%d: expected sequence to increment by 1, got [%d]", i, seq-ids[i-1].Sequence())
		}

		expectedSeq := uint16(s) + seqMin
		if seq != expectedSeq {
			t.Errorf("%d: expected sequence [%d], got [%d]", i, expectedSeq, seq)
		}

		if id.Partition() != part {
			t.Errorf("%d: partition differs from generator's partition; expected [%d], got [%d]", i, part, id.Partition())
		}
	}

	if len(timeDist) > 1 {
		t.Error("IDs generated with the same time ended up with different timestamps")
	}

	// Race test.
	var wg sync.WaitGroup
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			for i := 0; i < sampleSize; i++ {
				_ = g.NewWithTime(byte(i), tn)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestGenerator_Uniqueness(t *testing.T) {
	var (
		collisions int
		setSize    = 4 * MaxSequence
	)

	ids := make(map[ID]struct{}, setSize)

	for i := 1; i < setSize; i++ {
		id := generator.New(255)
		if _, found := ids[id]; found {
			collisions++
		} else {
			ids[id] = struct{}{}
		}
	}

	if collisions > 0 {
		t.Errorf("generated %d colliding IDs in a set of %d", collisions, setSize)
	}
}

func TestGenerator_Partition(t *testing.T) {
	expected := Partition{'A', 255}
	g, err := NewGenerator(&GeneratorSnapshot{
		Partition: expected,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	actual := g.Partition()
	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestGenerator_SequenceBounds(t *testing.T) {
	min := uint16(1024)
	max := uint16(2047)
	g, err := NewGenerator(&GeneratorSnapshot{
		SequenceMin: min,
		SequenceMax: max,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if actual, expected := g.SequenceMin(), min; actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}

	if actual, expected := g.SequenceMax(), max; actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}

	if actual, expected := g.Cap(), int(max-min)+1; actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}

	if actual, expected := g.Len(), 0; actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}

	for i := 0; i < 5; i++ {
		g.New(255)
	}

	if actual, expected := g.Len(), 5; actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}

	g, err = NewGenerator(&GeneratorSnapshot{
		SequenceMin: 8,
		SequenceMax: 16,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate an overflow. All IDs over Cap() must be generated in a subsequent timeframe
	// meaning Len will reflect the count in the last frame.
	// TODO(alcore) This *can* occasionally fail as we are not using a deterministic time source,
	// meaning first batch can get split up if time changes during the test and then end up
	// spilling into the Len() we test for.
	for i := 0; i < g.Cap()+7; i++ {
		g.New(255)
	}

	if actual, expected := g.Len(), 7; actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}

	g, err = NewGenerator(&GeneratorSnapshot{
		SequenceMin: 8,
		SequenceMax: 16,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < g.Cap(); i++ {
		g.New(255)
	}

	if actual, expected := g.Len(), g.Cap(); actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}
}

func TestGenerator_Sequence_Single(t *testing.T) {
	g, err := NewGenerator(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	expected0 := uint32(0)
	expected1 := expected0
	expected2 := expected1 + 1
	actual0 := g.Sequence()
	_ = g.New(255)
	actual1 := g.Sequence()
	_ = g.New(255)
	actual2 := g.Sequence()

	if actual0 != expected0 {
		t.Errorf("expected [%d], got [%d]", expected0, actual0)
	}
	if actual1 != expected1 {
		t.Errorf("expected [%d], got [%d]", expected1, actual1)
	}
	if actual2 != expected2 {
		t.Errorf("expected [%d], got [%d]", expected2, actual2)
	}
}

func TestGenerator_Sequence_Batch(t *testing.T) {
	g, err := NewGenerator(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := uint32(9)
	for i := 0; i <= int(expected); i++ {
		_ = g.New(255)
	}

	actual := g.Sequence()
	if actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}
}

func TestGenerator_FromSnapshot_Sequence(t *testing.T) {
	seq := uint32(1024)
	g, err := NewGenerator(&GeneratorSnapshot{
		SequenceMin: uint16(seq),
		Sequence:    seq,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	expected1 := seq
	expected2 := seq + 1
	_ = g.New(255)
	actual1 := g.Sequence()
	_ = g.New(255)
	actual2 := g.Sequence()

	if actual1 != expected1 {
		t.Errorf("expected [%d], got [%d]", expected1, actual1)
	}
	if actual2 != expected2 {
		t.Errorf("expected [%d], got [%d]", expected2, actual2)
	}
}

func TestGenerator_FromSnapshot_Pool_Defaults(t *testing.T) {
	t.Parallel()

	g, err := NewGenerator(&GeneratorSnapshot{
		SequenceMin: 0,
		SequenceMax: 0,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if g.SequenceMin() != 0 {
		t.Errorf("expected [%d], got [%d]", 0, g.SequenceMin())
	}

	if g.SequenceMax() != MaxSequence {
		t.Errorf("expected [%d], got [%d]", MaxSequence, g.SequenceMax())
	}

	// Max as default when min is given.
	g, err = NewGenerator(&GeneratorSnapshot{
		SequenceMin: 2048,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if g.SequenceMin() != 2048 {
		t.Errorf("expected [%d], got [%d]", 2048, g.SequenceMin())
	}

	if g.SequenceMax() != MaxSequence {
		t.Errorf("expected [%d], got [%d]", MaxSequence, g.SequenceMax())
	}
}

func TestGenerator_FromSnapshot_Pool_BoundsOrder(t *testing.T) {
	t.Parallel()

	g, err := NewGenerator(&GeneratorSnapshot{
		SequenceMin: 2048,
		SequenceMax: 1024,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if g.SequenceMin() != 1024 {
		t.Errorf("expected [%d], got [%d]", 1024, g.SequenceMin())
	}

	if g.SequenceMax() != 2048 {
		t.Errorf("expected [%d], got [%d]", 2048, g.SequenceMax())
	}
}

func TestGenerator_FromSnapshot_Pool_None(t *testing.T) {
	t.Parallel()

	bound := uint16(2048)
	_, err := NewGenerator(&GeneratorSnapshot{
		SequenceMin: bound,
		SequenceMax: bound,
	}, nil)
	if err == nil {
		t.Errorf("expected error, got none")
		return
	}

	verr, ok := err.(*InvalidSequenceBoundsError)
	if !ok {
		t.Errorf("expected error type [%T], got [%T]", &InvalidSequenceBoundsError{}, err)
		return
	}

	if verr.Msg != errSequenceBoundsIdenticalMsg {
		t.Errorf("expected error msg [%s], got [%s]", errSequenceBoundsIdenticalMsg, verr.Msg)
	}

	if verr.Min != bound {
		t.Errorf("expected [%d], got [%d]", bound, verr.Min)
	}

	if verr.Max != bound {
		t.Errorf("expected [%d], got [%d]", bound, verr.Max)
	}

	expectedMsg := fmt.Sprintf(errInvalidSequenceBoundsFmt, errSequenceBoundsIdenticalMsg, bound, 0, bound, 1)
	if verr.Error() != expectedMsg {
		t.Errorf("expected error msg [%s], got [%s]", expectedMsg, verr.Error())
	}
}

func TestGenerator_FromSnapshot_Pool_Size(t *testing.T) {
	t.Parallel()

	seqMin := uint16(0)
	seqMax := seqMin + minSequencePoolSize - 1
	_, err := NewGenerator(&GeneratorSnapshot{
		SequenceMin: seqMin,
		SequenceMax: seqMax,
	}, nil)
	if err == nil {
		t.Errorf("expected error, got none")
		return
	}

	verr, ok := err.(*InvalidSequenceBoundsError)
	if !ok {
		t.Errorf("expected error type [%T], got [%T]", &InvalidSequenceBoundsError{}, err)
		return
	}

	if verr.Msg != errSequencePoolTooSmallMsg {
		t.Errorf("expected error msg [%s], got [%s]", errSequencePoolTooSmallMsg, verr.Msg)
	}

	if verr.Min != seqMin {
		t.Errorf("expected [%d], got [%d]", seqMin, verr.Min)
	}

	if verr.Max != seqMax {
		t.Errorf("expected [%d], got [%d]", seqMax, verr.Max)
	}

	expectedMsg := fmt.Sprintf(errInvalidSequenceBoundsFmt, errSequencePoolTooSmallMsg, seqMin, 0, seqMax, seqMax-seqMin+1)
	if verr.Error() != expectedMsg {
		t.Errorf("expected error msg [%s], got [%s]", expectedMsg, verr.Error())
	}
}

func TestGenerator_FromSnapshot_Underflow(t *testing.T) {
	t.Parallel()

	seqMin := uint16(2048)
	seq := uint32(seqMin - 1)
	_, err := NewGenerator(&GeneratorSnapshot{
		SequenceMin: seqMin,
		Sequence:    seq,
	}, nil)
	if err == nil {
		t.Errorf("expected error, got none")
		return
	}

	verr, ok := err.(*InvalidSequenceBoundsError)
	if !ok {
		t.Errorf("expected error type [%T], got [%T]", &InvalidSequenceBoundsError{}, err)
		return
	}

	if verr.Msg != errSequenceUnderflowsBound {
		t.Errorf("expected error msg [%s], got [%s]", errSequenceUnderflowsBound, verr.Msg)
	}

	if verr.Min != seqMin {
		t.Errorf("expected [%d], got [%d]", seqMin, verr.Min)
	}

	if verr.Cur != seq {
		t.Errorf("expected [%d], got [%d]", seq, verr.Cur)
	}

	expectedMsg := fmt.Sprintf(errInvalidSequenceBoundsFmt, errSequenceUnderflowsBound, seqMin, seq, MaxSequence, MaxSequence-seqMin+1)
	if verr.Error() != expectedMsg {
		t.Errorf("expected error msg [%s], got [%s]", expectedMsg, verr.Error())
	}
}

func TestGenerator_Snapshot(t *testing.T) {
	var (
		part   = Partition{128, 255}
		seqMin = uint16(1024)
		seqMax = uint16(2047)
		seq    = uint32(1024)
	)

	snap := &GeneratorSnapshot{
		Partition:   part,
		SequenceMin: seqMin,
		SequenceMax: seqMax,
		Sequence:    seq,
	}

	g, err := NewGenerator(snap, nil)
	if err != nil {
		t.Fatal(err)
	}

	actual := g.Snapshot()
	if actual.Sequence != seq {
		t.Errorf("expected [%d], got [%d]", seq, actual.Sequence)
	}

	atomic.AddUint32(&g.drifts, 1)
	wallNow := snotime()
	g.New(255) // First call will catch a zero wallHi and reset the sequence, while we want to measure an incr.
	g.New(255)
	actual = g.Snapshot()

	if uint64(actual.Now) != wallNow {
		t.Errorf("expected [%d], got [%d]", wallNow, actual.Now)
	}

	if uint64(actual.WallHi) != wallNow {
		t.Errorf("expected [%d], got [%d]", wallNow, actual.WallHi)
	}

	if actual.Drifts != 1 {
		t.Errorf("expected [%d], got [%d]", 1, actual.Drifts)
	}

	if actual.Sequence != seq+1 {
		t.Errorf("expected [%d], got [%d]", seq+1, actual.Sequence)
	}

	if actual.Partition != part {
		t.Errorf("expected [%s], got [%s]", part, actual.Partition)
	}

	if actual.SequenceMin != seqMin {
		t.Errorf("expected [%d], got [%d]", seqMin, actual.SequenceMin)
	}

	if actual.SequenceMax != seqMax {
		t.Errorf("expected [%d], got [%d]", seqMax, actual.SequenceMax)
	}
}
