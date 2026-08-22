package domain

import (
	"sort"
	"testing"
)

func TestParseLocalDatePrecision(t *testing.T) {
	tests := []struct {
		in   string
		want Precision
	}{
		{"2019", PrecYear},
		{"2019-08", PrecMonth},
		{"2019-08-12", PrecDay},
		{"2019-08-12T14:30", PrecMinute},
	}
	for _, tt := range tests {
		d, err := ParseLocalDate(tt.in)
		if err != nil {
			t.Errorf("ParseLocalDate(%q): %v", tt.in, err)
			continue
		}
		if d.Precision() != tt.want {
			t.Errorf("ParseLocalDate(%q).Precision() = %v, want %v", tt.in, d.Precision(), tt.want)
		}
		if d.String() != tt.in {
			t.Errorf("ParseLocalDate(%q).String() = %q, want the input back", tt.in, d.String())
		}
	}
}

func TestLocalDateRejectsInvalid(t *testing.T) {
	for _, s := range []string{
		"2019-13",             // month 13
		"2019-02-30",          // no such day
		"2019-08-12T25:00",    // hour 25
		"2019-8",              // not zero-padded
		"20190812",            // no separators
		"2019-08-12T14:30:00", // seconds: not a precision we support
		"2019-08-12 14:30",    // space instead of T
		"",                    // use the zero value for "unset", not an empty parse
	} {
		if _, err := ParseLocalDate(s); err == nil {
			t.Errorf("ParseLocalDate(%q) = nil error, want a rejection", s)
		}
	}
}

// The whole variable-precision scheme rests on this property (spec 1.3): the
// lexicographic order of the ISO-8601 prefixes IS chronological order, so no
// database needs special comparison code.
func TestLexicographicOrderIsChronological(t *testing.T) {
	chronological := []string{
		"2018",
		"2019",
		"2019-08",
		"2019-08-12",
		"2019-08-12T09:05",
		"2019-08-12T14:30",
		"2019-09",
		"2020",
	}

	shuffled := []string{
		"2019-08-12T14:30", "2018", "2020", "2019-08",
		"2019-08-12T09:05", "2019", "2019-09", "2019-08-12",
	}
	sort.Strings(shuffled)

	for i := range chronological {
		if shuffled[i] != chronological[i] {
			t.Fatalf("sorted[%d] = %q, want %q\nsorted: %v", i, shuffled[i], chronological[i], shuffled)
		}
	}
}

func TestDirPrefix(t *testing.T) {
	// spec 3.4: the projection layer absorbs platform differences. ':' is
	// illegal in Windows filenames, so minute precision drops the ISO form.
	tests := []struct{ in, want string }{
		{"2019", "2019"},
		{"2019-08", "2019-08"},
		{"2019-08-12", "2019-08-12"},
		{"2019-08-12T14:30", "2019-08-12_1430"},
	}
	for _, tt := range tests {
		d, err := ParseLocalDate(tt.in)
		if err != nil {
			t.Fatalf("ParseLocalDate(%q): %v", tt.in, err)
		}
		if got := d.DirPrefix(); got != tt.want {
			t.Errorf("ParseLocalDate(%q).DirPrefix() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestZeroLocalDate(t *testing.T) {
	var d LocalDate
	if !d.IsZero() {
		t.Error("zero value should report IsZero")
	}
	if d.Precision() != PrecNone {
		t.Errorf("zero value Precision() = %v, want PrecNone", d.Precision())
	}
	v, err := d.Value()
	if err != nil {
		t.Fatalf("Value(): %v", err)
	}
	if v != nil {
		t.Errorf("Value() = %#v, want nil so it stores as NULL", v)
	}
}
