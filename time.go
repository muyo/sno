// +build !test

package sno

import "github.com/muyo/sno/internal"

// snotime returns the current wall clock time reported by the OS as adjusted to our internal epoch.
//
// It is a thin wrapper over actual implementations provided separately by os/arch dependent code.
//
// Note: tests use a different implementation of snotime() which is dynamically dispatched
// and does not necessarily call internal.Snotime().
func snotime() uint64 {
	return internal.Snotime()
}
