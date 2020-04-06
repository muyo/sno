package internal

import (
	"testing"
	"time"
)

func testSnotime(t *testing.T) {
	// Covers all arch/os combinations since they are expected to provide the snotime() function
	// to the rest of the package.
	//
	// Strictly speaking this test can be flaky if the time.Now() call happens to cross
	// the boundary between different TimeUnits, but that would just be really bad luck.
	actual := Snotime()
	expected := uint64(time.Now().UnixNano()-epochNsec) / timeUnit

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}
