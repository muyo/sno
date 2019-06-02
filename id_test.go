package sno

import (
	"bytes"
	"sync/atomic"
	"testing"
	"time"
)

func TestID_Time(t *testing.T) {
	tn := time.Now()
	id := New(255)

	// As we prune the fraction, actual cmp needs to be adjusted. This *may* also fail
	// in the rare condition that a new timeframe started between time.Now() and New()
	// since we're not using a deterministic time source currently.
	expected := tn.UnixNano() / TimeUnit
	actual := id.Time().UnixNano() / TimeUnit

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestID_Timestamp(t *testing.T) {
	tn := time.Now()
	id := New(255)

	expected := tn.UnixNano() / TimeUnit * timeUnitStep // Drop precision for the comparison.
	actual := id.Timestamp()

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestID_Meta(t *testing.T) {
	var expected byte = 255
	id := New(expected)
	actual := id.Meta()

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestID_Partition(t *testing.T) {
	expected := generator.partition
	actual := generator.New(255).Partition()

	if actual != expected {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestID_Sequence(t *testing.T) {
	expected := atomic.LoadUint32(generator.seq) + 1
	actual := generator.New(255).Sequence()

	if actual != uint16(expected) {
		t.Errorf("expected [%v], got [%v]", expected, actual)
	}
}

func TestID_String(t *testing.T) {
	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := "brpk4q72xwf2m63l"
	actual := src.String()

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestID_MarshalText(t *testing.T) {
	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := []byte("brpk4q72xwf2m63l")

	actual, err := src.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(actual, expected) != 0 {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestID_UnmarshalText_Valid(t *testing.T) {
	actual := ID{}
	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}

	if err := actual.UnmarshalText([]byte("brpk4q72xwf2m63l")); err != nil {
		t.Fatal(err)
	}

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestID_UnmarshalText_Invalid(t *testing.T) {
	id := ID{}
	err := id.UnmarshalText([]byte("012brpk4q72xwf2m63l1245453gfdgxz"))

	if err != errInvalidID {
		t.Errorf("expected [%s], got [%s]", errInvalidID, err)
	}
}

func TestID_MarshalJSON_Valid(t *testing.T) {
	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := []byte("\"brpk4q72xwf2m63l\"")

	actual, err := src.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(actual, expected) != 0 {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestID_MarshalJSON_Null(t *testing.T) {
	src := ID{}
	expected := []byte("null")
	actual, err := src.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(actual, expected) != 0 {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestID_UnmarshalJSON_Valid(t *testing.T) {
	actual := ID{}
	expected := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}

	if err := actual.UnmarshalJSON([]byte("\"brpk4q72xwf2m63l\"")); err != nil {
		t.Fatal(err)
	}

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestID_UnmarshalJSON_Invalid(t *testing.T) {
	id := ID{}
	err := id.UnmarshalJSON([]byte("\"012brpk4q72xwf2m63l1245453gfdgxz\""))

	if err != errInvalidID {
		t.Errorf("expected [%s], got [%s]", errInvalidID, err)
	}
}

func TestID_UnmarshalJSON_Null(t *testing.T) {
	actual := ID{}
	expected := ID{}

	if err := actual.UnmarshalJSON([]byte("null")); err != nil {
		t.Fatal(err)
	}

	if actual != expected {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}
}

func TestID_IsZero(t *testing.T) {
	cases := []struct {
		name string
		id   ID
		want bool
	}{
		{
			id:   New(255),
			want: false,
		},
		{
			id:   ID{},
			want: true,
		},
	}

	for _, c := range cases {
		if actual, expected := c.id.IsZero(), c.want; actual != expected {
			t.Errorf("expected [%v], got [%v]", expected, actual)
		}
	}
}
