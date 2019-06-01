package sno

import (
	"bytes"
	"testing"
)

// TODO(alcore) Needs deterministic cases for, amongst others:
// - Generation with overflowing the sequence pool;
// - Monotonic order guarantees when not overflowing;
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

	// Must not fail.
	if err != nil {
		t.Error(err)
		return
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
		if !bytes.Equal(p1[:], p2[:]) {
			t.Errorf("%d: partitions differ despite using the same generator, %d vs %d", i, p1[:], p2[:])
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
