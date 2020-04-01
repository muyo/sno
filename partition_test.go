package sno

import (
	"sync/atomic"
	"testing"
)

func TestPartition_Public_Conversions(t *testing.T) {
	t.Run("AsUint16", func(t *testing.T) {
		src := Partition{255, 255}
		expected := uint16(maxPartition)
		actual := src.AsUint16()

		if actual != expected {
			t.Errorf("expected [%d], got [%d]", expected, actual)
		}
	})

	t.Run("PutUint16", func(t *testing.T) {
		expected := Partition{255, 255}
		actual := Partition{}
		actual.PutUint16(maxPartition)

		if actual != expected {
			t.Errorf("expected [%s], got [%s]", expected, actual)
		}
	})
}

func TestPartition_Internal_Conversions(t *testing.T) {
	public := Partition{255, 255}
	internal := uint32(maxPartition) << 16

	t.Run("to-internal", func(t *testing.T) {
		expected := internal
		actual := partitionToInternalRepr(public)

		if actual != expected {
			t.Errorf("expected [%d], got [%d]", expected, actual)
		}
	})

	t.Run("to-public", func(t *testing.T) {
		expected := public
		actual := partitionToPublicRepr(internal)

		if actual != expected {
			t.Errorf("expected [%d], got [%d]", expected, actual)
		}
	})
}

func TestPartition_Internal_Generation(t *testing.T) {
	t.Run("monotonic-increments", func(t *testing.T) {
		// Reset global count (leaving seed as is).
		atomic.StoreUint32(&partitions, 0)

		var prevPartition = uint32(seed) << 16

		for i := 0; i < 100; i++ {
			p, err := genPartition()
			if err != nil {
				t.Fatal(err)
			}

			// Note: genPartition() shifts to make space for the sequence,
			// so we can't simply check for an increment of 1 within the resulting
			// uint32. The below is a tiny bit faster than converting back
			// to an uint16.
			if p-prevPartition != 1<<16 {
				t.Errorf("expected [%d], got [%d]", prevPartition+1<<16, p)
				break
			}

			prevPartition = p
		}
	})

	t.Run("pool-exhaustion", func(t *testing.T) {
		// Reset global count (leaving seed as is).
		atomic.StoreUint32(&partitions, 0)

		for i := 0; i < 2*maxPartition; i++ {
			_, err := genPartition()

			if err != nil {
				verr, ok := err.(*PartitionPoolExhaustedError)
				if !ok {
					t.Fatalf("expected error type [%T], got [%T]", &PartitionPoolExhaustedError{}, err)
					return
				}

				if i < maxPartition {
					t.Fatalf("expected errors no sooner than after [%d] iterations, got to [%d]", maxPartition, i)
					return
				}

				errMsgActual := verr.Error()
				errMsgExpected := errPartitionPoolExhaustedMsg

				if errMsgActual != errMsgExpected {
					t.Fatalf("expected error msg [%s], got [%s]", errMsgExpected, errMsgActual)
				}
			}

			if i >= maxPartition {
				if err == nil {
					t.Fatalf("expected constant errors after [%d] iterations, got no error at [%d]", maxPartition, i)
					return
				}
			}
		}
	})

	// Clean up.
	atomic.StoreUint32(&partitions, 0)
}
