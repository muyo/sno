package sno

import (
	"testing"
)

func TestEncoding_AMD64_EncodeVector(t *testing.T) {
	if !cpuHasVectorSupport() {
		t.Skip("Skipping test (CPU lacks instruction set)")
	}

	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := [SizeEncoded]byte{}
	copy(expected[:], "brpk4q72xwf2m63l")

	actual := encodeVector(&src)

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestEncoding_AMD64_DecodeVector(t *testing.T) {
	if !cpuHasVectorSupport() {
		t.Skip("Skipping test (CPU lacks instruction set)")
	}

	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	actual := ID{}

	decodeVector(&actual, []byte("brpk4q72xwf2m63l"))

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}
