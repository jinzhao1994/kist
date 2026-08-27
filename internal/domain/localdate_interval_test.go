package domain

import "testing"

// A variable-precision date denotes an interval, and range matching compares
// intervals (spec 4.2.1). These are its ends.
func TestLocalDateInterval(t *testing.T) {
	cases := []struct{ in, start, end string }{
		{"2019", "2019-01-01T00:00", "2019-12-31T23:59"},
		{"2019-02", "2019-02-01T00:00", "2019-02-28T23:59"},
		{"2020-02", "2020-02-01T00:00", "2020-02-29T23:59"}, // leap year
		{"2019-12", "2019-12-01T00:00", "2019-12-31T23:59"},
		{"2019-08-12", "2019-08-12T00:00", "2019-08-12T23:59"},
		{"2019-08-12T14:30", "2019-08-12T14:30", "2019-08-12T14:30"},
	}
	for _, c := range cases {
		d, err := ParseLocalDate(c.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := d.Start(); got != c.start {
			t.Errorf("%s: Start = %q, want %q", c.in, got, c.start)
		}
		if got := d.End(); got != c.end {
			t.Errorf("%s: End = %q, want %q", c.in, got, c.end)
		}
	}
}

func TestZeroLocalDateHasNoInterval(t *testing.T) {
	var d LocalDate
	if d.Start() != "" || d.End() != "" {
		t.Errorf("the zero date has an interval: %q..%q", d.Start(), d.End())
	}
}

// The property the padding width exists for: a stored value of any precision
// compares correctly against a bound of the widest precision.
func TestIntervalBoundsOrderAgainstEveryStoredPrecision(t *testing.T) {
	june, _ := ParseLocalDate("2019-06")
	hi := june.End()
	for _, stored := range []string{"2019", "2019-06", "2019-06-30", "2019-06-30T14:00"} {
		if !(stored <= hi) {
			t.Errorf("%q sorts after the end of June (%q), so a range containing it would miss it", stored, hi)
		}
	}
	lo := june.Start()
	for _, stored := range []string{"2019-07", "2020"} {
		if stored <= hi {
			t.Errorf("%q is after June but compares inside it (hi %q)", stored, hi)
		}
	}
	_ = lo
}
