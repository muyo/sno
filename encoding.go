// +build !amd64

package sno

const (
	// The encoding is a custom base32 variant stemming from base32hex.
	// The alphabet is 2 contiguous ASCII ranges: `50..57` (digits) and `97..120` (lowercase letters).
	// A canonically encoded ID can be validated with a regexp of `[2-9a-x]{16}`.
	encoding = "23456789abcdefghijklmnopqrstuvwx"
)

var (
	// Decoding LUT.
	decoding [256]byte

	// Dummy flag to be set by the appropriate build (used by tests).
	hasVectorSupport bool
)

func init() {
	for i := 0; i < len(decoding); i++ {
		decoding[i] = 0xFF
	}

	for i := 0; i < len(encoding); i++ {
		decoding[encoding[i]] = byte(i)
	}
}

func encode(src *ID) (dst [SizeEncoded]byte) {
	dst[14] = encoding[(src[9]>>5|src[8]<<3)&0x1F]
	dst[13] = encoding[src[8]>>2&0x1F]
	dst[12] = encoding[(src[8]>>7|src[7]<<1)&0x1F]
	dst[11] = encoding[(src[7]>>4|src[6]<<4)&0x1F]
	dst[10] = encoding[src[6]>>1&0x1F]
	dst[9] = encoding[(src[6]>>6|src[5]<<2)&0x1F]
	dst[8] = encoding[src[5]>>3]

	dst[7] = encoding[src[4]&0x1F]
	dst[6] = encoding[(src[4]>>5|src[3]<<3)&0x1F]
	dst[5] = encoding[src[3]>>2&0x1F]
	dst[4] = encoding[(src[3]>>7|src[2]<<1)&0x1F]
	dst[3] = encoding[(src[2]>>4|src[1]<<4)&0x1F]
	dst[2] = encoding[src[1]>>1&0x1F]
	dst[1] = encoding[(src[1]>>6|src[0]<<2)&0x1F]
	dst[0] = encoding[src[0]>>3]

	return
}

func decode(src []byte) (dst ID) {
	_ = src[15] // BCE hint.

	dst[9] = decoding[src[14]]<<5 | decoding[src[15]]
	dst[8] = decoding[src[12]]<<7 | decoding[src[13]]<<2 | decoding[src[14]]>>3
	dst[7] = decoding[src[11]]<<4 | decoding[src[12]]>>1
	dst[6] = decoding[src[9]]<<6 | decoding[src[10]]<<1 | decoding[src[11]]>>4
	dst[5] = decoding[src[8]]<<3 | decoding[src[9]]>>2

	dst[4] = decoding[src[6]]<<5 | decoding[src[7]]
	dst[3] = decoding[src[4]]<<7 | decoding[src[5]]<<2 | decoding[src[6]]>>3
	dst[2] = decoding[src[3]]<<4 | decoding[src[4]]>>1
	dst[1] = decoding[src[1]]<<6 | decoding[src[2]]<<1 | decoding[src[3]]>>4
	dst[0] = decoding[src[0]]<<3 | decoding[src[1]]>>2

	return
}
