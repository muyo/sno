// +build !amd64

package internal

const (
	// The encoding is a custom base32 variant stemming from base32hex.
	// The alphabet is 2 contiguous ASCII ranges: `50..57` (digits) and `97..120` (lowercase letters).
	// A canonically encoded ID can be validated with a regexp of `[2-9a-x]{16}`.
	enc = "23456789abcdefghijklmnopqrstuvwx"
)

var (
	// Decoding LUT.
	dec [256]byte

	// Dummy flag to be set by the appropriate build (used by tests).
	hasVectorSupport bool
)

func init() {
	for i := 0; i < len(dec); i++ {
		dec[i] = 0xFF
	}

	for i := 0; i < len(enc); i++ {
		dec[enc[i]] = byte(i)
	}
}

func Encode(src *[10]byte) (dst [16]byte) {
	dst[15] = enc[src[9]&0x1F]
	dst[14] = enc[(src[9]>>5|src[8]<<3)&0x1F]
	dst[13] = enc[src[8]>>2&0x1F]
	dst[12] = enc[(src[8]>>7|src[7]<<1)&0x1F]
	dst[11] = enc[(src[7]>>4|src[6]<<4)&0x1F]
	dst[10] = enc[src[6]>>1&0x1F]
	dst[9] = enc[(src[6]>>6|src[5]<<2)&0x1F]
	dst[8] = enc[src[5]>>3]

	dst[7] = enc[src[4]&0x1F]
	dst[6] = enc[(src[4]>>5|src[3]<<3)&0x1F]
	dst[5] = enc[src[3]>>2&0x1F]
	dst[4] = enc[(src[3]>>7|src[2]<<1)&0x1F]
	dst[3] = enc[(src[2]>>4|src[1]<<4)&0x1F]
	dst[2] = enc[src[1]>>1&0x1F]
	dst[1] = enc[(src[1]>>6|src[0]<<2)&0x1F]
	dst[0] = enc[src[0]>>3]

	return
}

func Decode(src []byte) (dst [10]byte) {
	_ = src[15] // BCE hint.

	dst[9] = dec[src[14]]<<5 | dec[src[15]]
	dst[8] = dec[src[12]]<<7 | dec[src[13]]<<2 | dec[src[14]]>>3
	dst[7] = dec[src[11]]<<4 | dec[src[12]]>>1
	dst[6] = dec[src[9]]<<6 | dec[src[10]]<<1 | dec[src[11]]>>4
	dst[5] = dec[src[8]]<<3 | dec[src[9]]>>2

	dst[4] = dec[src[6]]<<5 | dec[src[7]]
	dst[3] = dec[src[4]]<<7 | dec[src[5]]<<2 | dec[src[6]]>>3
	dst[2] = dec[src[3]]<<4 | dec[src[4]]>>1
	dst[1] = dec[src[1]]<<6 | dec[src[2]]<<1 | dec[src[3]]>>4
	dst[0] = dec[src[0]]<<3 | dec[src[1]]>>2

	return
}
