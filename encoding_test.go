package sno

import "testing"

func init() {
	// Decoding LUT might not be initialized depending on the platform the tests are being run on.
	if dec == nil {
		dec = &[256]byte{}

		for i := 0; i < len(dec); i++ {
			dec[i] = 0xFF
		}

		for i := 0; i < len(encoding); i++ {
			dec[encoding[i]] = byte(i)
		}
	}
}

func TestEncoding_EncodeScalar(t *testing.T) {
	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := [SizeEncoded]byte{}
	copy(expected[:], "brpk4q72xwf2m63l")

	actual := encodeScalar(&src)

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestEncoding_DecodeScalar(t *testing.T) {
	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	actual := ID{}

	decodeScalar(&actual, []byte("brpk4q72xwf2m63l"))

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}
