package domain

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// Precision says how much of a LocalDate is actually known. It is derived from
// the string, never stored alongside it: a second copy of the same fact is a
// second thing that can be wrong (spec 1.3).
type Precision uint8

const (
	PrecNone Precision = iota
	PrecYear
	PrecMonth
	PrecDay
	PrecMinute
)

func (p Precision) String() string {
	switch p {
	case PrecYear:
		return "year"
	case PrecMonth:
		return "month"
	case PrecDay:
		return "day"
	case PrecMinute:
		return "minute"
	default:
		return "none"
	}
}

// LocalDate is a wall-clock date of variable precision, held as an ISO-8601
// prefix: "2019", "2019-08", "2019-08-12", "2019-08-12T14:30".
//
// No timezone, deliberately. It describes human time — "August 12th in
// Yunnan" is that day, and converting to UTC would shorten the trip
// (spec 1.3). Machine timestamps (created_at, updated_at) are a different
// thing entirely and are stored as UTC.
//
// The zero value means "not set" and stores as NULL.
type LocalDate struct {
	s string
}

// layouts maps the length of an accepted prefix to its time layout. Length is
// the discriminator because each precision has exactly one width.
var layouts = map[int]string{
	4:  "2006",
	7:  "2006-01",
	10: "2006-01-02",
	16: "2006-01-02T15:04",
}

// ParseLocalDate accepts exactly the four ISO-8601 prefixes above. It rejects
// everything else, including seconds — widening the set later is easy, but a
// value that slipped in under a looser parser cannot be narrowed again.
func ParseLocalDate(s string) (LocalDate, error) {
	layout, ok := layouts[len(s)]
	if !ok {
		return LocalDate{}, fmt.Errorf("parse local date %q: not one of the accepted forms 2006, 2006-01, 2006-01-02, 2006-01-02T15:04", s)
	}
	if _, err := time.Parse(layout, s); err != nil {
		return LocalDate{}, fmt.Errorf("parse local date %q: %w", s, err)
	}
	return LocalDate{s: s}, nil
}

func (d LocalDate) String() string { return d.s }

func (d LocalDate) IsZero() bool { return d.s == "" }

func (d LocalDate) Precision() Precision {
	switch len(d.s) {
	case 4:
		return PrecYear
	case 7:
		return PrecMonth
	case 10:
		return PrecDay
	case 16:
		return PrecMinute
	default:
		return PrecNone
	}
}

// DirPrefix renders the date for a directory name (spec 3.4). Minute precision
// becomes "2019-08-12_1430": ':' cannot appear in a Windows filename, so the
// ISO form would be uncreatable there. The database keeps the ISO form.
func (d LocalDate) DirPrefix() string {
	if d.Precision() != PrecMinute {
		return d.s
	}
	return strings.NewReplacer("T", "_", ":", "").Replace(d.s)
}

func (d LocalDate) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.s, nil
}

func (d *LocalDate) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = LocalDate{}
		return nil
	case string:
		parsed, err := ParseLocalDate(v)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		return d.Scan(string(v))
	default:
		return fmt.Errorf("scan local date: cannot read %T", src)
	}
}

func (d LocalDate) MarshalText() ([]byte, error) { return []byte(d.s), nil }

func (d *LocalDate) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*d = LocalDate{}
		return nil
	}
	parsed, err := ParseLocalDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Start and End are the interval a variable-precision date denotes.
//
// A LocalDate is not an instant, it is however much was known: "2019" means
// the whole of 2019, "2019-08" the whole of August (spec 1.3). Range matching
// therefore compares intervals, not points (spec 4.2.1), and these are the two
// ends of one.
//
// Both pad out to minute precision, the widest form there is, and that width
// is load-bearing rather than tidy. Values in the database are prefixes of
// varying width, and prefix strings compare lexicographically: an End of
// "2019-06-30" would sort before the stored value "2019-06-30T14:00", so a
// file captured that afternoon would fall outside a range that plainly
// contains it.
func (d LocalDate) Start() string {
	switch d.Precision() {
	case PrecMinute:
		return d.s
	case PrecDay:
		return d.s + "T00:00"
	case PrecMonth:
		return d.s + "-01T00:00"
	case PrecYear:
		return d.s + "-01-01T00:00"
	}
	return ""
}

// End is the last minute of the interval this date denotes.
func (d LocalDate) End() string {
	switch d.Precision() {
	case PrecMinute:
		return d.s
	case PrecDay:
		return d.s + "T23:59"
	case PrecMonth:
		return fmt.Sprintf("%s-%02dT23:59", d.s, daysIn(d.s))
	case PrecYear:
		return d.s + "-12-31T23:59"
	}
	return ""
}

// daysIn returns the length of the month named by a "2006-01" prefix.
//
// It is computed rather than looked up so that February is right in leap
// years — a table of month lengths is the kind of thing that is correct for
// three years and then is not.
func daysIn(month string) int {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return 28 // unreachable: the value already parsed as a LocalDate
	}
	return t.AddDate(0, 1, -1).Day()
}
