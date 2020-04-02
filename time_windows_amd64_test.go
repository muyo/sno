// +build test

package sno

//go:noescape
func ostime() uint64

// Keep this in sync with the '!test' implementation.
func snotimeReal() uint64 {
	return ostime() / 4e4
}
