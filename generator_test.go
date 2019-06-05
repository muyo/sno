package sno

import (
	"testing"
)

// TODO(alcore) Needs deterministic time source to cover, amongst others:
// - Monotonic order guarantees when not overflowing.
// - Simulated clock regression and multi-regression.
// - Restoration from snapshot taken prior to a regression.
func TestGenerator_NewNoOverflow(t *testing.T) {
	var (
		seqPool = uint16(MaxSequence / 2)
		// Scaled to not exceed bounds, otherwise we run into the seqOverflow race and order - which we
		// test for in here - becomes non-deterministic.
		sampleSize = int(seqPool)
		g, err     = NewGenerator(&GeneratorSnapshot{
			Partition:   Partition{255, 255},
			SequenceMin: seqPool,
			SequenceMax: 2*seqPool - 1,
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

		p1, p2 := curID.Partition(), prevID.Partition()
		if p1 != p2 {
			t.Errorf("%d: partitions differ despite using the same generator, %d vs %d", i, p1, p2)
		}
	}
}

func TestGenerator_NewOverflows(t *testing.T) {
	var (
		seqPool      = 1024
		seqOverflows = 4
		seqMin       = uint16(seqPool)
		seqMax       = uint16(2*seqPool - 1)
		sampleSize   = int(seqPool * seqOverflows)
		g, err       = NewGenerator(&GeneratorSnapshot{
			Partition:   Partition{255, 255},
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

	timeDistMap := make(map[int64]int)

	for i := 0; i < sampleSize; i++ {
		timeDistMap[ids[i].Timestamp()]++

		seq := ids[i].Sequence()
		if seq > seqMax {
			t.Errorf("%d: sequence overflowing max boundary; max [%d], got [%d]", i, seqMin, seq)
		}

		if seq < seqMin {
			t.Errorf("%d: sequence underflowing min boundary; min [%d], got [%d]", i, seqMin, seq)
		}
	}

	for tf, c := range timeDistMap {
		if c > seqPool {
			t.Errorf("count of IDs in the given timeframe exceeds pool; timestamp [%d], pool [%d], count [%d]", tf, seqPool, c)
		}
	}
}

func TestGenerator_Uniqueness(t *testing.T) {
	var (
		collisions int
		setSize    = 10 * MaxSequence
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
