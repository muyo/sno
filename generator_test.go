package sno

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestPartition_PutUint16(t *testing.T) {
	expected := Partition{255, 255}
	actual := Partition{}
	actual.PutUint16(65535)

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestPartition_AsUint16(t *testing.T) {
	src := Partition{255, 255}
	expected := uint16(65535)
	actual := src.AsUint16()

	if actual != expected {
		t.Errorf("expected [%d], got [%d]", expected, actual)
	}
}

// TODO(alcore) Needs deterministic time source to cover, amongst others:
// - Monotonic order guarantees when not overflowing.
// - Simulated clock regression and multi-regression.
// - Restoration from snapshot taken prior to a regression.
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

func TestGenerator_Sequence_FromSnapshot(t *testing.T) {
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

	atomic.AddUint32(g.drifts, 1)
	wallNow, _ := nanotime()
	g.New(255) // First call will catch a zero wallHi and reset the sequence, while we want to measure an incr.
	g.New(255)
	actual = g.Snapshot()

	if actual.Now != wallNow {
		t.Errorf("expected [%d], got [%d]", wallNow, actual.Now)
	}

	if actual.WallHi != wallNow {
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
