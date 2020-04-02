// +build !windows !amd64
// +build test

package sno

import _ "unsafe" // required to use //go:linkname

//go:linkname ostime runtime.walltime1
func ostime() (sec int64, nsec int32)

// Keep this in sync with the '!test' implementation.
func snotimeReal() uint64 {
	wallSec, wallNsec := ostime()

	return (uint64(wallSec)*1e9 + uint64(wallNsec) - epochNsec) / TimeUnit
}
