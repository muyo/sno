package sno

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"time"
	"unsafe"

	"github.com/muyo/sno/internal"
)

const (
	// SizeBinary is the length of an ID in its binary array representation.
	SizeBinary = 10

	// SizeEncoded is the length of an ID in its canonical base-32 encoded representation.
	SizeEncoded = 16

	// Epoch is the offset to the Unix epoch, in seconds, that ID timestamps are embedded with.
	// Corresponds to 2010-01-01 00:00:00 UTC.
	Epoch     = 1262304000
	epochNsec = Epoch * 1e9

	// TimeUnit is the time unit timestamps are embedded with - 4msec, as expressed in nanoseconds.
	TimeUnit = 4e6

	// MaxTimestamp is the max number of time units that can be embedded in an ID's timestamp.
	// Corresponds to 2079-09-07 15:47:35.548 UTC in our custom epoch.
	MaxTimestamp = 1<<39 - 1

	// MaxPartition is the max Partition number when represented as a uint16.
	// It equals max uint16 (65535) and is the equivalent of Partition{255, 255}.
	MaxPartition = 1<<16 - 1

	// MaxSequence is the max sequence number supported by generators. As bounds can be set
	// individually - this is the upper cap and equals max uint16 (65535).
	MaxSequence = 1<<16 - 1
)

// ID is the binary representation of a sno ID.
//
// It is comprised of 10 bytes in 2 blocks of 40 bits, with its components stored in big-endian order.
//
// The timestamp:
//	39 bits - unsigned milliseconds since epoch with a 4msec resolution
//	  1 bit - the tick-tock toggle
//
// The payload:
//	 8 bits - metabyte
//	16 bits - partition
//	16 bits - sequence
//
type ID [SizeBinary]byte

// Time returns the timestamp of the ID as a time.Time struct.
func (id ID) Time() time.Time {
	var (
		units = int64(binary.BigEndian.Uint64(id[:]) >> 25)
		s     = units/250 + Epoch
		ns    = (units % 250) * TimeUnit
	)

	return time.Unix(s, ns)
}

// Timestamp returns the timestamp of the ID as nanoseconds relative to the Unix epoch.
func (id ID) Timestamp() int64 {
	return int64(binary.BigEndian.Uint64(id[:])>>25)*TimeUnit + epochNsec
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

// String implements fmt.Stringer by returning the base32-encoded representation of the ID
// as a string.
func (id ID) String() string {
	enc := internal.Encode((*[10]byte)(&id))
	dst := enc[:]

	return *(*string)(unsafe.Pointer(&dst))
}

// Bytes returns the ID as a byte slice.
func (id ID) Bytes() []byte {
	return id[:]
}

// MarshalBinary implements encoding.BinaryMarshaler by returning the ID as a byte slice.
func (id ID) MarshalBinary() ([]byte, error) {
	return id[:], nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler by copying src into the receiver.
func (id *ID) UnmarshalBinary(src []byte) error {
	if len(src) != SizeBinary {
		return &InvalidDataSizeError{Size: len(src)}
	}

	copy(id[:], src)

	return nil
}

// MarshalText implements encoding.TextMarshaler by returning the base32-encoded representation
// of the ID as a byte slice.
func (id ID) MarshalText() ([]byte, error) {
	b := internal.Encode((*[10]byte)(&id))

	return b[:], nil
}

// UnmarshalText implements encoding.TextUnmarshaler by decoding a base32-encoded representation
// of the ID from src into the receiver.
func (id *ID) UnmarshalText(src []byte) error {
	if len(src) != SizeEncoded {
		return &InvalidDataSizeError{Size: len(src)}
	}

	*id = internal.Decode(src)

	return nil
}

// MarshalJSON implements encoding.json.Marshaler by returning the base32-encoded and quoted
// representation of the ID as a byte slice.
//
// If the ID is a zero value, MarshalJSON will return a byte slice containing 'null' (unquoted) instead.
//
// Note that ID's are byte arrays and Go's std (un)marshaler is unable to distinguish
// the zero values of custom structs as "empty", so the 'omitempty' tag has the same caveats
// as, for example, time.Time.
//
// See https://github.com/golang/go/issues/11939 for tracking purposes as changes are being
// discussed.
func (id ID) MarshalJSON() ([]byte, error) {
	if id == zero {
		return []byte("null"), nil
	}

	dst := []byte("\"                \"")
	enc := internal.Encode((*[10]byte)(&id))
	copy(dst[1:], enc[:])

	return dst, nil
}

// UnmarshalJSON implements encoding.json.Unmarshaler by decoding a base32-encoded and quoted
// representation of an ID from src into the receiver.
//
// If the byte slice is an unquoted 'null', the receiving ID will instead be set
// to a zero ID.
func (id *ID) UnmarshalJSON(src []byte) error {
	n := len(src)
	if n != SizeEncoded+2 {
		if n == 4 && src[0] == 'n' && src[1] == 'u' && src[2] == 'l' && src[3] == 'l' {
			*id = zero
			return nil
		}

		return &InvalidDataSizeError{Size: n}
	}

	*id = internal.Decode(src[1 : n-1])

	return nil
}

// Compare returns an integer comparing this and that ID lexicographically.
//
// Returns:
// 	 0 - if this and that are equal,
// 	-1 - if this is smaller than that,
// 	 1 - if this is greater than that.
//
// Note that IDs are byte arrays - if all you need is to check for equality, a simple...
//	if thisID == thatID {...}
// ... will do the trick.
func (id ID) Compare(that ID) int {
	return bytes.Compare(id[:], that[:])
}

// Value implements the sql.driver.Valuer interface by returning the ID as a byte slice.
// If you'd rather receive a string, wrapping an ID is a possible solution...
//
//	// stringedID wraps a sno ID to provide a driver.Valuer implementation which
//	// returns strings.
//	type stringedID sno.ID
//
//	func (id stringedID) Value() (driver.Value, error) {
//		return sno.ID(id).String(), nil
//	}
//
//	// ... and use it via:
// 	db.Exec(..., stringedID(id))
func (id ID) Value() (driver.Value, error) {
	return id.MarshalBinary()
}

// Scan implements the sql.Scanner interface by attempting to convert the given value
// into an ID.
//
// When given a byte slice:
//	- with a length of SizeBinary (10), its contents will be copied into ID.
//	- with a length of 0, ID will be set to a zero ID.
//	- with any other length, sets ID to a zero ID and returns InvalidDataSizeError.
//
// When given a string:
//	- with a length of SizeEncoded (16), its contents will be decoded into ID.
//	- with a length of 0, ID will be set to a zero ID.
//	- with any other length, sets ID to a zero ID and returns InvalidDataSizeError.
//
// When given nil, ID will be set to a zero ID.
//
// When given any other type, returns a InvalidTypeError.
func (id *ID) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		switch len(v) {
		case SizeBinary:
			copy(id[:], v)
		case 0:
			*id = zero
		default:
			*id = zero
			return &InvalidDataSizeError{Size: len(v)}
		}

	case string:
		switch len(v) {
		case SizeEncoded:
			*id = internal.Decode(*(*[]byte)(unsafe.Pointer(&v)))
		case 0:
			*id = zero
		default:
			*id = zero
			return &InvalidDataSizeError{Size: len(v)}
		}

	case nil:
		*id = zero

	default:
		return &InvalidTypeError{Value: value}
	}

	return nil
}
