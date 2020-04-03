// +build !test

package sno

// snotime returns the current wall clock time reported by the OS as adjusted to our internal epoch.
//
// It is a thin wrapper over snotimeReal which is provided separately by os/arch dependent code.
//
// Note: tests use a different implementation of snotime() which is dynamically dispatched
// and does not necessarily call snotimeReal.
func snotime() uint64 {
	return snotimeReal()
}
