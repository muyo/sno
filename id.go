package sno

import (
	"time"
	"unsafe"
)

const (
	SizeBinary  = 10
	SizeEncoded = 16
)

// ID is the binary representation of an ID.
type ID [SizeBinary]byte

// Time returns the timestamp of the ID as a time.Time struct.
func (id ID) Time() time.Time {
	msecs := int64(id[0]) << 31
	msecs |= int64(id[1]) << 23
	msecs |= int64(id[2]) << 15
	msecs |= int64(id[3]) << 7
	msecs |= int64(id[4]) >> 1

	s := msecs/250 - Epoch
	ns := (msecs % 250) * TimeUnit

	return time.Unix(s, ns)
}

// Timestamp returns the timestamp of the ID as nanoseconds since its epoch.
func (id ID) Timestamp() int64 {
	msecs := int64(id[0]) << 31
	msecs |= int64(id[1]) << 23
	msecs |= int64(id[2]) << 15
	msecs |= int64(id[3]) << 7
	msecs |= int64(id[4]) >> 1

	return msecs*4 - epochMsec
	// The entirety of the above could be done with some simple asm, but the func would stop
	// being inlineable. It's 6 instructions vs 17 in the above.
	//    MOVQ   id+0(FP), AX
	//    BSWAPQ AX
	//    SHRQ   $25, AX 			// Gets rid of the 3 lower bytes and the tick-tock bit.
	//    MOVQ   $885179647000, BX 	// epochMsec -885 179 647 000
	//    LEAQ   (BX)(AX*4), BX
	//    MOVQ   BX, ret+16(FP)
	//    RET
}

// Meta returns the metabyte of the ID.
func (id ID) Meta() byte {
	return id[5]
}

// Partition returns the partition of the ID.
func (id ID) Partition() Partition {
	return Partition{id[6], id[7]}
}

// Sequence returns the sequence of the ID.
func (id ID) Sequence() uint16 {
	return uint16(id[8])<<8 | uint16(id[9])
}

// IsZero checks whether the ID is a zero value.
func (id ID) IsZero() bool {
	return id == zero
}

// String returns the base32-encoded representation of the ID as a string.
// It implements the std fmt Stringer interface.
func (id ID) String() string {
	enc := encode(&id)
	b := enc[:]

	return *(*string)(unsafe.Pointer(&b))
}

// MarshalText returns the base32-encoded representation of the ID as a byte slice.
// It implements the std encoding/text TextMarshaler interface.
func (id *ID) MarshalText() ([]byte, error) {
	b := encode(id)

	return b[:], nil
}

// UnmarshalText decodes a base32-encoded representation of an ID contained in the given byte slice
// into the receiver.
// It implements the std encoding TextUnmarshaler interface.
func (id *ID) UnmarshalText(src []byte) error {
	if len(src) != SizeEncoded {
		return errInvalidID
	}

	decode(id, src)

	return nil
}

// MarshalJSON returns the base32-encoded and quoted representation of the ID as a byte slice.
// If the ID is a zero value, MarshalJSON will return a byte slice of 'null' (unquoted) instead.
// It implements the std encoding/json Marshaler interface.
//
// Note that ID's are byte arrays and Go's std (un)marshaler is unable to distinguish
// the zero values of custom structs as "empty", so the 'omitempty' flag has the same caveats
// as, for example, time.Time.
//
// See https://github.com/golang/go/issues/11939 for tracking purposes as changes are being
// discussed.
func (id *ID) MarshalJSON() ([]byte, error) {
	if *id == zero {
		return []byte("null"), nil
	}

	dst := make([]byte, SizeEncoded+2)
	enc := encode(id)

	copy(dst[1:SizeEncoded+1], enc[:])
	dst[0], dst[SizeEncoded+1] = '"', '"'

	return dst, nil
}

// UnmarshalJSON decodes a base32 encoded and quoted representation of an ID in the given
// byte slice into the receiver. If the byte slice contains an unquoted 'null', the receiving ID
// will instead be set to the zero value of an ID.
//
// It implements the std encoding/json Unmarshaler interface.
func (id *ID) UnmarshalJSON(src []byte) error {
	n := len(src)
	if n != SizeEncoded+2 && n != 4 {
		return errInvalidID
	}

	_ = src[3] // BCE hint.
	if src[0] == 'n' && src[1] == 'u' && src[2] == 'l' && src[3] == 'l' {
		*id = zero
		return nil
	}

	decode(id, src[1:n-1])

	return nil
}
