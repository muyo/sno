//go:build !(windows && amd64) && !(linux && amd64 && go1.17)
// +build !windows !amd64
// +build !linux !amd64 !go1.17

package internal

import _ "unsafe" // Required for go:linkname

// ostime returns the current wall clock time reported by the OS.
//
// The function is linked against runtime.walltime() directly, which is only available since the
// introduction of faketime in Go 1.14 (which is the version sno depends on at minimum). This being
// linked to an internal function instead of a semi-stable one like time.now() is somewhat brittle,
// but the rationale is explained below.
//
// POSIXy arch/OS combinations use some form of clock_gettime with CLOCK_REALTIME, either through
// a syscall, libc call (Darwin) or vDSO (Linux).
// These calls are relatively slow, even using vDSO. Not using time.Now() allows us to bypass getting
// the monotonic clock readings which is a separate invocation of the underlying kernel facility and
// roughly doubles the execution time.
//
// As a result, doing sno.New(0).Time() tends to be actually faster on those platforms than time.Now(),
// despite an entire ID being generated alongside. That is, if you're fine with the precision reduced to 4ms.
//
// On Windows/amd64 we use an even more efficient implementation which allows us to also bypass
// some unnecessary unit conversions, which isn't as trivially possible on POSIXy systems (as their
// kernels keep track of time and provide secs and fractional secs instead of a singular higher
// resolution source).
//
// See https://lore.kernel.org/linux-arm-kernel/20190621095252.32307-1-vincenzo.frascino@arm.com
// to get an overview of the perf numbers involved on Linux-based distros.
//
//go:linkname ostime runtime.walltime
func ostime() (sec int64, nsec int32)

// Snotime returns the current wall clock time reported by the OS as adjusted to our internal epoch.
func Snotime() uint64 {
	wallSec, wallNsec := ostime()

	return (uint64(wallSec)*1e9 + uint64(wallNsec) - epochNsec) / timeUnit
}
