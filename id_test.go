package sno

import (
	"bytes"
	"reflect"
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

	id = NewWithTime(255, tn)
	actual = id.Time().UnixNano() / TimeUnit

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

	id = NewWithTime(255, tn)
	actual = id.Timestamp()

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

func TestID_Bytes(t *testing.T) {
	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := make([]byte, SizeBinary)
	copy(expected, src[:])

	actual := src.Bytes()
	if !bytes.Equal(actual, expected) {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}

	actual[SizeBinary-1]++
	if bytes.Equal(expected, actual) {
		t.Error("returned a reference to underlying array")
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

	if err != errInvalidDataSize {
		t.Errorf("expected error [%s], got [%s]", errInvalidDataSize, err)
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

	if err != errInvalidDataSize {
		t.Errorf("expected error [%s], got [%s]", errInvalidDataSize, err)
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
	for _, c := range []struct {
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
	} {
		if actual, expected := c.id.IsZero(), c.want; actual != expected {
			t.Errorf("expected [%v], got [%v]", expected, actual)
		}
	}
}

func TestID_Compare(t *testing.T) {
	a := New(100)
	l := a
	l[5]++
	e := a
	b := a
	b[5]--

	if actual := a.Compare(l); actual != -1 {
		t.Errorf("expected [-1], got [%d]", actual)
	}

	if actual := a.Compare(e); actual != 0 {
		t.Errorf("expected [0], got [%d]", actual)
	}

	if actual := a.Compare(b); actual != 1 {
		t.Errorf("expected [1], got [%d]", actual)
	}
}

func TestID_Value(t *testing.T) {
	src := ID{78, 111, 33, 96, 160, 255, 154, 10, 16, 51}
	expected := make([]byte, SizeBinary)
	copy(expected, src[:])

	v, err := src.Value()
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	actual, ok := v.([]byte)
	if !ok {
		t.Errorf("expected type [%T], got [%T]", expected, actual)
	}

	if !bytes.Equal(actual, expected) {
		t.Errorf("expected [%s], got [%s]", expected, actual)
	}

	actual[SizeBinary-1]++
	if bytes.Equal(expected, actual) {
		t.Error("returned a reference to underlying array")
	}
}

func TestID_Scan(t *testing.T) {
	id := New(255)

	for _, c := range []struct {
		name string
		in   interface{}
		out  ID
		err  error
	}{
		{"nil", nil, ID{}, nil},
		{"bytes-valid", id[:], id, nil},
		{"bytes-invalid", make([]byte, 3), zero, errInvalidDataSize},
		{"bytes-zero", []byte{}, zero, nil},
		{"string-valid", id.String(), id, nil},
		{"string-invalid", "123", zero, errInvalidDataSize},
		{"string-zero", "", zero, nil},
		{"invalid", 69, ID{}, &InvalidTypeError{}},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var out ID
			err := out.Scan(c.in)

			if actual, expected := out, c.out; actual != expected {
				t.Errorf("expected [%s], got [%s]", expected, actual)
			}

			if err != nil && c.err == nil {
				t.Errorf("got unexpected error: %s", err)
			} else if actual, expected := reflect.TypeOf(err), reflect.TypeOf(c.err); actual != expected {
				t.Errorf("expected error type [%s], got [%s]", expected, actual)
			}
		})
	}
}
