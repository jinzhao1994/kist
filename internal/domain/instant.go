package domain

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"time"
)

// Instant is a moment that really happened, held as unix nanoseconds.
//
// It is deliberately not a LocalDate. A box's start_at is a fuzzy interval a
// person declared — "that trip was some time in August 2019" — while a photo's
// capture time is an instant, and forcing them into one type would be mixing
// two different ideas rather than unifying a representation (spec 1.5.2).
//
// Nanoseconds, one unit for every instant in the library. How much precision a
// source really has stays a property of that source: EXIF lands on a whole
// second, a filesystem mtime keeps what the filesystem recorded.
//
// Instants are not unique. Within one, the keyset tiebreaker (id, i.e. scan
// order, i.e. sorted file name) is what orders a burst.
//
// The zero value means "not known" and stores as NULL. That is a real state,
// not a missing one — a PDF, a scan and a voice memo have no capture time, and
// spec 4.5.1 puts them last in both directions.
type Instant struct {
	ns    int64
	valid bool
}

// NewInstant builds one from unix nanoseconds. Negative values are ordinary: a
// photograph scanned from 1960 is exactly what an archive is for. The
// representable range is 1678–2262, which covers photography; anything older is
// a box date, which is a LocalDate and unbounded.
func NewInstant(ns int64) Instant { return Instant{ns: ns, valid: true} }

// NewInstantFromTime builds one from a time.Time, which is the usual source.
func NewInstantFromTime(t time.Time) Instant { return NewInstant(t.UnixNano()) }

func (i Instant) IsZero() bool { return !i.valid }

// UnixNano returns the nanoseconds and whether there are any.
func (i Instant) UnixNano() (int64, bool) { return i.ns, i.valid }

// Time renders it in loc. The caller supplies the zone because this type does
// not know one — and because reading time.Local here would make the same
// library display differently on two machines (spec 4.7).
func (i Instant) Time(loc *time.Location) (time.Time, bool) {
	if !i.valid {
		return time.Time{}, false
	}
	return time.Unix(0, i.ns).In(loc), true
}

// String is the decimal nanoseconds, or "" when unset. This is what a keyset
// cursor carries (spec 4.5), so it has to round-trip through ParseInstant.
func (i Instant) String() string {
	if !i.valid {
		return ""
	}
	return strconv.FormatInt(i.ns, 10)
}

// ParseInstant reads String back. The empty string is the unset value rather
// than an error, because that is the form a cursor carries for a file with no
// capture time.
func ParseInstant(s string) (Instant, error) {
	if s == "" {
		return Instant{}, nil
	}
	ns, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Instant{}, fmt.Errorf("parse instant %q: %w", s, err)
	}
	return NewInstant(ns), nil
}

func (i Instant) Value() (driver.Value, error) {
	if !i.valid {
		return nil, nil
	}
	return i.ns, nil
}

func (i *Instant) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*i = Instant{}
		return nil
	case int64:
		*i = NewInstant(v)
		return nil
	case float64:
		// SQLite hands back whatever was stored, and a REAL here would mean
		// somebody wrote a float into an INTEGER column. Refuse rather than
		// truncate: silently losing the low digits of a timestamp is the kind
		// of wrong that never gets noticed.
		return fmt.Errorf("scan instant: got a float (%v); captured_at must hold whole nanoseconds", v)
	default:
		return fmt.Errorf("scan instant: cannot read %T", src)
	}
}

func (i Instant) MarshalText() ([]byte, error) { return []byte(i.String()), nil }

func (i *Instant) UnmarshalText(b []byte) error {
	parsed, err := ParseInstant(string(b))
	if err != nil {
		return err
	}
	*i = parsed
	return nil
}
