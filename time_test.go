package sno

import (
	"sync/atomic"
	"testing"
	"time"
	_ "unsafe" // required to use //go:linkname
)

// monotime provides real monotonic clock readings to several tests.
//go:linkname monotime runtime.nanotime
func monotime() int64

var (
	staticInc     = new(uint64)
	staticWallNow = func() *uint64 {
		wall := snotime()
		return &wall
	}()
)

// staticTime provides tests with a fake time source which returns a fixed time on each call.
// The time returned can be changed by directly (atomically) mutating the underlying variable.
func staticTime() uint64 {
	return atomic.LoadUint64(staticWallNow)
}

// staticIncTime provides tests with a fake time source which returns a time based on a fixed time
// monotonically increasing by 1 TimeUnit on each call.
func staticIncTime() uint64 {
	wall := atomic.LoadUint64(staticWallNow) + atomic.LoadUint64(staticInc)*TimeUnit

	atomic.AddUint64(staticInc, 1)

	return wall
}

func TestTime_Snotime(t *testing.T) {
	// Covers all arch/os combinations since they are expected to provide the snotime() function
	// to the rest of the package.
	actual := snotime()
	expected := uint64(time.Now().UnixNano()-epochNsec) / TimeUnit

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}
