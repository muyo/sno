package sno

import (
	"bytes"
	"testing"
)

func TestEncoding(t *testing.T) {
	runEncodingWithFallback("encode", t, testEncodingEncode)
	runEncodingWithFallback("decode", t, testEncodingDecode)
}

func testEncodingEncode(t *testing.T) {
	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := []byte("brpk4q72xwf2m63l")
	actual := encode(&src)
	if !bytes.Equal(actual[:], expected) {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func testEncodingDecode(t *testing.T) {
	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	actual := decode([]byte("brpk4q72xwf2m63l"))
	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func runEncodingWithFallback(name string, t *testing.T, f func(t *testing.T)) {
	if hasVectorSupport {
		t.Run(name+"-vectorized", f)
	} else {
		t.Run(name+"-fallback", f)
		return
	}

	hasVectorSupport = false
	t.Run(name+"-fallback", f)
	hasVectorSupport = true
}
