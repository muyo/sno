package sno

import (
	_ "unsafe" // required to use //go:linkname
)

const (
	// Epoch is the offset to the Unix epoch, in seconds, that ID timestamps are embedded with.
	// 1262304000 is 2010-01-01 00:00:00 UTC
	Epoch     = 1262304000
	epochNsec = Epoch * 1e9

	// TimeUnit is the time unit timestamps are embedded with - 4msec.
	TimeUnit = 4e6

	// MaxTimestamp is the max number of time units that can be embedded in an ID's timestamp.
	// Corresponds to 2079-09-07 15:47:35.548 UTC in our custom epoch.
	MaxTimestamp = 1<<39 - 1

	// MaxSequence is the max sequence number supported by generators.
	// As bounds can be set individually - this is the upper cap.
	MaxSequence = 1<<16 - 1

	// Arbitrary min pool size of 4 per time unit (that is 1000 per sec).
	minSequencePoolSize = 4

	tickRate    = TimeUnit / tickRateDiv
	tickRateDiv = 4
)

//go:linkname now time.now
func now() (sec int64, nsec int32, mono int64)

// nanotime returns the current wall clock time reported by the OS as adjusted to our internal epoch.
func nanotime() uint64 {
	wallSec, wallNsec, _ := now()

	return (uint64(wallSec)*1e9 + uint64(wallNsec) - epochNsec) / TimeUnit
}
