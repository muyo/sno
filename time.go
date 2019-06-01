package sno

import (
	_ "unsafe" // required to use //go:linkname
)

const (
	// Epoch is the offset to the Unix epoch, in seconds, that ID timestamps are embedded with.
	// Canonically this includes a sign offset.
	// 1262304000 is 2010-01-01 00:00:00 UTC
	// 2147483647 is the sign offset
	Epoch     = 1262304000 - 2147483647
	epochMsec = Epoch * 1e3
	epochNsec = Epoch * 1e9

	// TimeUnit is the time unit timestamps are embedded with - 4msec, handled as nanoseconds internally.
	TimeUnit = 4e6

	// MaxTimestamp is the max number of time units that can be embedded in an ID's timestamp.
	// Corresponds to 2067-09-25 19:01:42.548 UTC in our custom epoch.
	MaxTimestamp = 1<<39 - 1

	// MaxSequence is the max sequence number supported by generators.
	// As bounds can be set individually - this is the upper cap.
	MaxSequence = 1<<16 - 1

	// Arbitrary min pool size of 16 per time unit (that is 4000 per sec).
	minSequencePoolSize = 16

	tickRate    = TimeUnit / tickRateDiv
	tickRateDiv = 4
)

//go:linkname now time.now
func now() (sec int64, nsec int32, mono int64)

// nanotime returns the current wall clock time reported by the OS as adjusted to our internal epoch
// and the current monotonic clock reading verbatim as reported by the OS.
func nanotime() (wall int64, mono int64) {
	var (
		wallNowSec, wallNowNsec, monoNow = now()
		wallnow                          = wallNowSec*1e9 + int64(wallNowNsec) - epochNsec
	)

	return wallnow / TimeUnit, monoNow
}
