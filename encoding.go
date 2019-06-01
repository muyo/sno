package sno

const (
	encoding = "23456789abcdefghijklmnopqrstuvwx"
)

var (
	// Decoding LUT. We declare this as a pointer to avoid allocating a 256-byte
	// all-zero table that we might not need at all (vide init.go).
	dec *[256]byte

	// encode and decode are the actual codec functions used on a package level.
	// On platforms that have vectorized codecs those will be assigned to the ASM
	// implementations. Everywhere else init.go will assign encodeScalar and decodeScalar
	// respectively as fallbacks. Function pointers have a slight overhead but not notable
	// enough to care in this case.
	encode func(src *ID) [SizeEncoded]byte
	decode func(dst *ID, src []byte)
)

func encodeScalar(src *ID) [SizeEncoded]byte {
	var dst [SizeEncoded]byte

	dst[15] = encoding[src[9]&0x1F]
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

	return dst
}

func decodeScalar(dst *ID, src []byte) {
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
}
