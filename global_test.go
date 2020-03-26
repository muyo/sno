package sno

import (
	"reflect"
	"testing"
)

func TestGlobal_Init(t *testing.T) {
	t.Run("sane", func(t *testing.T) {
		defer func() {
			if err := recover(); err != nil {
				t.Fatal("expected init to not panic")
			}
		}()

		// Must never panic.
		doInit()
	})

	t.Run("panics", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("expected init to panic")
			}

			if _, ok := err.(*PartitionPoolExhaustedError); !ok {
				t.Errorf("expected panic with type [%T], got [%T]", &PartitionPoolExhaustedError{}, err)
				return
			}
		}()

		// Theoretically impossible to happen but ensure that we cover all "potential" cases
		// where the global generator could fail to get constructed and we need to panic.
		//
		// At present only one branch even has an error return, so we simulate that... impossibility
		// by trying to create more Generators without snapshots than we have a Partition pool for.
		// Note that we are invoking doInit() instead of NewGenerator() directly.
		for i := 0; i < 2*maxPartition; i++ {
			doInit()
		}
	})
}

func TestGlobal_FromEncodedString_Valid(t *testing.T) {
	src := "brpk4q72xwf2m63l"
	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}

	actual, err := FromEncodedString(src)
	if err != nil {
		t.Fatal(err)
	}

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestGlobal_FromEncodedString_Invalid(t *testing.T) {
	_, err := FromEncodedString("012brpk4q72xwf2m63l1245453gfdgxz")
	if err != errInvalidDataSize {
		t.Errorf("expected error [%s], got [%s]", errInvalidDataSize, err)
	}

	if err != nil && err.Error() != errInvalidDataSizeMsg {
		t.Errorf("expected error [%s], got [%s]", errInvalidDataSizeMsg, err.Error())
	}
}

func TestGlobal_FromEncodedBytes_Valid(t *testing.T) {
	src := []byte("brpk4q72xwf2m63l")
	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}

	actual, err := FromEncodedBytes(src)
	if err != nil {
		t.Fatal(err)
	}

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestGlobal_FromEncodedBytes_Invalid(t *testing.T) {
	_, err := FromEncodedBytes([]byte("012brpk4q72xwf2m63l1245453gfdgxz"))
	if err != errInvalidDataSize {
		t.Errorf("expected error [%s], got [%s]", errInvalidDataSize, err)
	}

	if err != nil && err.Error() != errInvalidDataSizeMsg {
		t.Errorf("expected error [%s], got [%s]", errInvalidDataSizeMsg, err.Error())
	}
}

func TestGlobal_FromBinaryBytes_Valid(t *testing.T) {
	src := []byte{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}

	actual, err := FromBinaryBytes(src)
	if err != nil {
		t.Fatal(err)
	}

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestGlobal_FromBinaryBytes_Invariant(t *testing.T) {
	expected := New(255)
	actual, err := FromBinaryBytes(expected[:])
	if err != nil {
		t.Fatal(err)
	}

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestGlobal_FromBinaryBytes_Invalid(t *testing.T) {
	for _, c := range []struct {
		n       int
		invalid bool
	}{
		{4, true},
		{8, true},
		{10, false},
		{12, true},
		{16, true},
	} {
		b := make([]byte, c.n)
		_, err := FromBinaryBytes(b)

		if actual, expected := err != nil, c.invalid; actual != expected {
			t.Errorf("expected error [%v], got [%v]", expected, actual)
		}
	}
}

func TestGlobal_Collection(t *testing.T) {
	var ids = []ID{{1}, {2}, {3}, {4}, {5}, {6}}

	t.Run("len", makeCollectionLenTest(ids))
	t.Run("less", makeCollectionLessTest(ids))
	t.Run("swap", makeCollectionSwapTest(ids))
	t.Run("sort", makeCollectionSortTest(ids))
}

func makeCollectionLenTest(ids []ID) func(t *testing.T) {
	n := len(ids)
	return func(t *testing.T) {
		if actual, expected := collection([]ID{}).Len(), 0; actual != expected {
			t.Errorf("Len() %v, want %v", expected, actual)
		}

		if actual, expected := collection(ids).Len(), n; actual != expected {
			t.Errorf("expected [%v], got [%v]", expected, actual)
		}
	}
}

func makeCollectionLessTest(ids []ID) func(t *testing.T) {
	return func(t *testing.T) {
		c := collection(ids)
		if c.Less(0, 0) {
			t.Errorf("expected [false], got [true]")
		}

		if !c.Less(0, 1) {
			t.Errorf("expected [true], got [false]")
		}

		if !c.Less(1, 2) {
			t.Errorf("expected [true], got [false]")
		}
	}
}

func makeCollectionSwapTest(ids []ID) func(t *testing.T) {
	return func(t *testing.T) {
		b := make([]ID, len(ids))
		copy(b, ids)

		c := collection(b)
		c.Swap(1, 2)
		if actual, expected := c[1], ids[2]; actual != expected {
			t.Errorf("expected [%v], got [%v]", expected, actual)
		}
		if actual, expected := c[2], ids[1]; actual != expected {
			t.Errorf("expected [%v], got [%v]", expected, actual)
		}
		c.Swap(3, 3)
		if actual, expected := c[3], ids[3]; actual != expected {
			t.Errorf("expected [%v], got [%v]", expected, actual)
		}
	}
}

func makeCollectionSortTest(ids []ID) func(t *testing.T) {
	return func(t *testing.T) {
		src := make([]ID, len(ids))
		copy(src, ids)

		// Input IDs are sorted, so a comparison will do the trick.
		src[2], src[1] = src[1], src[2]
		src[4], src[3] = src[3], src[4]

		Sort(src)

		if actual, expected := src, ids; !reflect.DeepEqual(actual, expected) {
			t.Errorf("expected [%v], got [%v]", expected, actual)
		}
	}
}

func TestGlobal_Zero(t *testing.T) {
	if actual := Zero(); actual != (ID{}) {
		t.Error("Zero() not equal to ID{}")
	}
}

func TestGlobal_Zero_IsZero(t *testing.T) {
	if !Zero().IsZero() {
		t.Error("Zero().IsZero() is not true")
	}
}
