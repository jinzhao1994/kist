package query

import (
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

func date(t *testing.T, s string) domain.LocalDate {
	t.Helper()
	d, err := domain.ParseLocalDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// instant renders a human-readable date as the unix second a file's capture
// time actually holds (spec 1.5.2). UTC is arbitrary and fixed: these cases
// care about round-tripping and ordering, not about which zone.
func instant(t *testing.T, s string) domain.Instant {
	t.Helper()
	d := date(t, s)
	start, err := time.Parse("2006-01-02T15:04", d.Start())
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return domain.NewInstant(start.UnixNano())
}

// The zero Query is "everything, newest first". A query type whose zero value
// meant nothing would make every caller write boilerplate before it could ask
// the simplest question there is — and one of them would eventually forget.
func TestZeroQueryIsEverythingNewestFirst(t *testing.T) {
	var q Query
	if err := q.Validate(); err != nil {
		t.Fatalf("the zero query is invalid: %v", err)
	}
	if q.Sort.Key != SortCaptured || q.Sort.Asc {
		t.Errorf("default sort = %+v, want captured descending", q.Sort)
	}
	if q.PageSize() != DefaultLimit {
		t.Errorf("default page size = %d, want %d", q.PageSize(), DefaultLimit)
	}
}

// A limit has to be bounded somewhere, or one request asks for a hundred
// thousand rows and the answer arrives as a memory spike.
func TestValidateBoundsTheLimit(t *testing.T) {
	if err := (Query{Limit: MaxLimit + 1}).Validate(); err == nil {
		t.Errorf("a limit of %d was accepted", MaxLimit+1)
	}
	if err := (Query{Limit: -1}).Validate(); err == nil {
		t.Error("a negative limit was accepted")
	}
	if err := (Query{Limit: MaxLimit}).Validate(); err != nil {
		t.Errorf("the maximum limit was rejected: %v", err)
	}
}

// A range ending before it starts is a mistake, and returning zero rows for it
// looks exactly like "you have nothing from then".
func TestValidateRejectsABackwardsRange(t *testing.T) {
	if err := (Query{Time: &DateRange{From: date(t, "2019-06"), To: date(t, "2019-03")}}).Validate(); err == nil {
		t.Error("a range ending before it starts was accepted")
	}
	// Overlapping ends of different precision are fine: "2019" to "2019-03" is
	// a real question, and comparing the interval ends is what makes it one.
	if err := (Query{Time: &DateRange{From: date(t, "2019"), To: date(t, "2019-03")}}).Validate(); err != nil {
		t.Errorf("a coarse start before a fine end was rejected: %v", err)
	}
	// And a half-open range is always fine.
	if err := (Query{Time: &DateRange{From: date(t, "2019")}}).Validate(); err != nil {
		t.Errorf("an open-ended range was rejected: %v", err)
	}
}

// An unknown sort key must be refused rather than silently treated as the
// default: a caller asking for something we do not have wants to hear so.
func TestValidateRejectsAnUnknownSortKey(t *testing.T) {
	if err := (Query{Sort: SortSpec{Key: SortKey(99)}}).Validate(); err == nil {
		t.Error("an unknown sort key was accepted")
	}
	if err := (Query{Sort: SortSpec{Key: SortKey(-1)}}).Validate(); err == nil {
		t.Error("a negative sort key was accepted")
	}
}
