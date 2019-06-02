package sno

import (
	"testing"
	"time"
)

func TestTime_Nanotime(t *testing.T) {
	actual, _ := nanotime()
	expected := (epochNsec + time.Now().UnixNano()) / TimeUnit

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}
