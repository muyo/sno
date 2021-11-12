//go:build go1.17
// +build go1.17

package internal

import "time"

// Snotime returns the current wall clock time reported by the OS as adjusted to our internal epoch.
func Snotime() uint64 {
	return uint64(time.Now().UnixNano()-epochNsec) / timeUnit
}
