// +build !windows
// +build !amd64

package sno

import (
	_ "unsafe" // required to use //go:linkname
)

//go:linkname now time.now
func now() (sec int64, nsec int32, mono int64)

// snotime returns the current wall clock time reported by the OS as adjusted to our internal epoch.
func snotime() uint64 {
	wallSec, wallNsec, _ := now()

	return (uint64(wallSec)*1e9 + uint64(wallNsec) - epochNsec) / TimeUnit
}
