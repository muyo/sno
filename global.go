package sno

import (
	"sort"
	"time"
	"unsafe"

	"github.com/muyo/sno/internal"
)

var (
	generator *Generator
	zero      ID
)

func init() {
	doInit()
}

func doInit() {
	g, err := NewGenerator(nil, nil)
	if err != nil {
		panic(err)
	}

	generator = g
}

// New uses the package-level generator to generate a new ID using the current system
// time for its timestamp.
func New(meta byte) ID {
	return generator.New(meta)
}

// NewWithTime uses the package-level generator to generate a new ID using the given time
// for the timestamp.
//
// IDs generated using this method are subject to several caveats.
// See generator.NewWithTime() for their documentation.
func NewWithTime(meta byte, t time.Time) ID {
	return generator.NewWithTime(meta, t)
}

// FromBinaryBytes takes a byte slice and copies its contents into an ID, returning the bytes as an ID.
//
// The slice must have a length of 10. Returns a InvalidDataSizeError if it does not.
func FromBinaryBytes(src []byte) (id ID, err error) {
	return id, id.UnmarshalBinary(src)
}

// FromEncodedBytes decodes a canonically base32-encoded byte slice representation of an ID
// into its binary representation and returns it.
//
// The slice must have a length of 16. Returns a InvalidDataSizeError if it does not.
func FromEncodedBytes(src []byte) (id ID, err error) {
	return id, id.UnmarshalText(src)
}

// FromEncodedString decodes a canonically base32-encoded string representation of an ID
// into its binary representation and returns it.
//
// The string must have a length of 16. Returns a InvalidDataSizeError if it does not.
func FromEncodedString(src string) (id ID, err error) {
	if len(src) != SizeEncoded {
		return zero, &InvalidDataSizeError{Size: len(src)}
	}

	// We only read in the data pointer (and input is read-only), so this does the job.
	return internal.Decode(*(*[]byte)(unsafe.Pointer(&src))), nil
}

type collection []ID

func (ids collection) Len() int           { return len(ids) }
func (ids collection) Less(i, j int) bool { return ids[i].Compare(ids[j]) < 0 }
func (ids collection) Swap(i, j int)      { ids[i], ids[j] = ids[j], ids[i] }

// Sort performs an in-place lexicographic sort of a slice of sno IDs.
func Sort(s []ID) {
	sort.Sort(collection(s))
}

// Zero returns the zero value of an ID, which is 10 zero bytes and equivalent to:
//
//	id := sno.ID{}
// ... e.g. ...
//	id := sno.ID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
func Zero() ID {
	return zero
}
