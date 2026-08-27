package domain

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"time"
)

// Instant is a moment that really happened, held as unix seconds.
//
// It is deliberately not a LocalDate. A box's start_at is a fuzzy interval a
// person declared — "that trip was some time in August 2019" — while a photo's
// capture time is an instant, and forcing them into one type would be mixing
// two different ideas rather than unifying a representation (spec 1.5.2).
//
// Seconds, not nanoseconds: EXIF resolves to the second, and within one second
// the keyset tiebreaker (id, i.e. scan order, i.e. sorted file name) already
// puts a burst in the right order.
//
// The zero value means "not known" and stores as NULL. That is a real state,
// not a missing one — a PDF, a scan and a voice memo have no capture time, and
// spec 4.5.1 puts them last in both directions.
type Instant struct {
	sec   int64
	valid bool
}

// NewInstant builds one from unix seconds. Negative values are ordinary: a
// photograph scanned from 1960 is exactly what an archive is for.
func NewInstant(sec int64) Instant { return Instant{sec: sec, valid: true} }

func (i Instant) IsZero() bool { return !i.valid }

// Unix returns the seconds and whether there are any.
func (i Instant) Unix() (int64, bool) { return i.sec, i.valid }

// Time renders it in loc. The caller supplies the zone because this type does
// not know one — and because reading time.Local here would make the same
// library display differently on two machines (spec 4.7).
func (i Instant) Time(loc *time.Location) (time.Time, bool) {
	if !i.valid {
		return time.Time{}, false
	}
	return time.Unix(i.sec, 0).In(loc), true
}

// String is the decimal seconds, or "" when unset. This is what a keyset
// cursor carries (spec 4.5), so it has to round-trip through ParseInstant.
func (i Instant) String() string {
	if !i.valid {
		return ""
	}
	return strconv.FormatInt(i.sec, 10)
}

// ParseInstant reads String back. The empty string is the unset value rather
// than an error, because that is the form a cursor carries for a file with no
// capture time.
func ParseInstant(s string) (Instant, error) {
	if s == "" {
		return Instant{}, nil
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Instant{}, fmt.Errorf("parse instant %q: %w", s, err)
	}
	return NewInstant(sec), nil
}

func (i Instant) Value() (driver.Value, error) {
	if !i.valid {
		return nil, nil
	}
	return i.sec, nil
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
		// truncate: silently losing the fractional part of a timestamp is the
		// kind of wrong that never gets noticed.
		return fmt.Errorf("scan instant: got a float (%v); captured_at must hold whole seconds", v)
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
