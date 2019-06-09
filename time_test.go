package sno

import (
	"testing"
	"time"
)

func TestTime_Nanotime(t *testing.T) {
	actual := nanotime()
	expected := (time.Now().UnixNano() - epochNsec) / TimeUnit

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}
