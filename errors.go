package sno

import "fmt"

const (
	errInvalidDataSizeMsg         = "sno: unrecognized data size"
	errInvalidTypeFmt             = "sno: unrecognized data type: %T"
	errInvalidSequenceBoundsFmt   = "sno: %s; min: %d, sequence: %d, max: %d, pool: %d"
	errSequenceBoundsIdenticalMsg = "sno: sequence bounds are identical - need a sequence pool with a capacity of at least 4"
	errSequenceUnderflowsBound    = "sno: current sequence underflows the given lower bound"
	errSequencePoolTooSmallMsg    = "sno: generators require a sequence pool with a capacity of at least 4"
	errPartitionPoolExhaustedMsg  = "sno: process exceeded maximum number of possible defaults-configured generators"
)

// InvalidDataSizeError gets returned when attempting to unmarshal or decode an ID from data that
// is not nil and not of a size of: SizeBinary, SizeEncoded nor 0.
type InvalidDataSizeError struct {
	Size int
}

func (e *InvalidDataSizeError) Error() string { return errInvalidDataSizeMsg }

// InvalidTypeError gets returned when attempting to scan a value that is neither...
//	- a string
//	- a byte slice
//	- nil
// ... into an ID via ID.Scan().
type InvalidTypeError struct {
	Value interface{}
}

func (e *InvalidTypeError) Error() string {
	return fmt.Sprintf(errInvalidTypeFmt, e.Value)
}

// InvalidSequenceBoundsError gets returned when a Generator gets seeded with sequence boundaries
// which are invalid, e.g. the pool is too small or the current sequence overflows the bounds.
type InvalidSequenceBoundsError struct {
	Cur uint32
	Min uint16
	Max uint16
	Msg string
}

func (e *InvalidSequenceBoundsError) Error() string {
	return fmt.Sprintf(errInvalidSequenceBoundsFmt, e.Msg, e.Min, e.Cur, e.Max, e.Max-e.Min+1)
}

// PartitionPoolExhaustedError gets returned when attempting to create more than MaxPartition (65535)
// Generators using the default configuration (eg. without snapshots).
//
// Should you ever run into this, please consult the docs on the genPartition() internal function.
type PartitionPoolExhaustedError struct{}

func (e *PartitionPoolExhaustedError) Error() string { return errPartitionPoolExhaustedMsg }
