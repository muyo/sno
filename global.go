package sno

import (
	"time"
)

var (
	generator *Generator
	zero      ID
)

// Zero returns the zero value of an ID.
func Zero() ID {
	return zero
}

// New generates a new ID using the current system time for its timestamp.
func New(meta byte) ID {
	return generator.New(meta)
}

// NewWithTime generates a new ID using the given time for the timestamp.
//
// IDs generated with user-specified timestamps are exempt from the tick-tock mechanism (but retain
// the same data layout). Managing potential collisions in their case is left to the user. This utility
// is primarily meant to enable porting of old IDs to sno and assumed to be ran before an ID scheme goes
// online.
func NewWithTime(meta byte, t time.Time) ID {
	return generator.NewWithTime(meta, t)
}

// FromBinaryBytes takes a byte slice and copies its contents into an ID, returning the bytes as an ID.
// The slice must have a length of 10.
func FromBinaryBytes(src []byte) (ID, error) {
	if len(src) != SizeBinary {
		return zero, errInvalidID
	}

	var id ID
	copy(id[:], src)
	return id, nil
}

// FromEncodedString decodes the provided, canonically base32-encoded string representation of an ID
// into a its binary representation and returns it.
// The string must have a length of 16.
func FromEncodedString(src string) (ID, error) {
	// Microbenchmarking note: while return FromEncodedBytes([]byte(src)) would do,
	// the call would not get inlined (Go 1.12) and we effectively go from 3ns to 6ns per op just for that.
	// That little bit of duplication is fine given the functions are trivial.
	if len(src) != SizeEncoded {
		return zero, errInvalidID
	}

	var id ID
	decode(&id, []byte(src))
	return id, nil
}

// FromEncodedBytes decodes the provided, canonically base32-encoded byte slice representation of an ID
// into a its binary representation and returns it.
// The slice must have a length of 10.
func FromEncodedBytes(src []byte) (ID, error) {
	if len(src) != SizeEncoded {
		return zero, errInvalidID
	}

	var id ID
	decode(&id, src)
	return id, nil
}
