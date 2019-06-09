package sno

import (
	"testing"
)

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
		b := make([]byte, c.n, c.n)
		_, err := FromBinaryBytes(b)

		if actual, expected := err != nil, c.invalid; actual != expected {
			t.Errorf("expected error [%v], got [%v]", expected, actual)
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
