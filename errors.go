package sno

import "fmt"

var (
	errInvalidID = &InvalidIDError{}
)

type InvalidIDError struct{}

func (e *InvalidIDError) Error() string { return "ID is invalid or encoded in unknown format" }

// InvalidGeneratorBoundsError gets returned when a Generator gets seeded with sequence boundaries
// which are invalid, e.g. the pool is too small or the current sequence overflows the bounds.
type InvalidGeneratorBoundsError struct {
	Cur uint16
	Min uint16
	Max uint16
	msg string
}

func (e *InvalidGeneratorBoundsError) Error() string {
	return fmt.Sprintf("%s; current: %d, min: %d, max: %d, pool: %d", e.msg, e.Cur, e.Min, e.Max, e.Max-e.Min)
}
