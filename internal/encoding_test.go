package internal

import (
	"bytes"
	"testing"
)

func testEncoding(t *testing.T) {
	runEncodingWithFallback("encode", t, testEncodingEncode)
	runEncodingWithFallback("decode", t, testEncodingDecode)
}

var encdec = [...]struct {
	enc [10]byte
	dec string
}{
	{
		[10]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"2222222222222222",
	},
	{
		[10]byte{78, 111, 33, 96, 160, 255, 154, 10, 16, 51},
		"brpk4q72xwf2m63l",
	},
	{
		[10]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		"xxxxxxxxxxxxxxxx",
	},
}

func testEncodingEncode(t *testing.T) {
	for _, c := range encdec {
		var (
			actual   = Encode(&c.enc)
			expected = []byte(c.dec)
		)

		if !bytes.Equal(actual[:], expected) {
			t.Errorf("expected [%s], got [%s]", expected, actual)
		}
	}
}

func testEncodingDecode(t *testing.T) {
	for _, c := range encdec {
		var (
			actual   = Decode([]byte(c.dec))
			expected = c.enc
		)

		if actual != expected {
			t.Errorf("expected [%v], got [%v]", expected, actual)
		}
	}
}

func runEncodingWithFallback(name string, t *testing.T, f func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		var actualVectorSupport = hasVectorSupport
		if actualVectorSupport {
			t.Run("vectorized", f)
		}

		hasVectorSupport = false
		t.Run("fallback", f)
		hasVectorSupport = actualVectorSupport
	})
}
