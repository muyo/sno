package sno

import (
	"bytes"
	"testing"
)

func TestEncoding_Encode(t *testing.T) {
	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := []byte("brpk4q72xwf2m63l")
	actual := encode(&src)

	if !bytes.Equal(actual[:], expected) {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}

	runWithoutVectors(func() {
		actual = encode(&src)
		if !bytes.Equal(actual[:], expected) {
			t.Errorf("expected [%s], got [%s]", expected, actual)
		}
	})
}

func TestEncoding_Decode(t *testing.T) {
	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	actual := decode([]byte("brpk4q72xwf2m63l"))

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}

	runWithoutVectors(func() {
		actual = decode([]byte("brpk4q72xwf2m63l"))
		if actual != expected {
			t.Errorf("expected [%s], got [%s]", expected, actual)
		}
	})
}

func runWithoutVectors(t func()) {
	if !hasVectorSupport {
		return
	}

	hasVectorSupport = false
	t()
	hasVectorSupport = true
}
