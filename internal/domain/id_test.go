package domain

import (
	"database/sql/driver"
	"testing"
)

func TestIDRoundTrip(t *testing.T) {
	id := NewID()

	parsed, err := ParseID(id.String())
	if err != nil {
		t.Fatalf("ParseID(%q): %v", id.String(), err)
	}
	if parsed != id {
		t.Errorf("round trip gave %v, want %v", parsed, id)
	}
}

func TestIDIsTimeOrdered(t *testing.T) {
	// UUIDv7 puts a millisecond timestamp in the high bits, which is why the
	// scheme was chosen (spec 3.12): index locality. Byte order must therefore
	// track creation order.
	first := NewID()
	var last ID
	for i := 0; i < 1000; i++ {
		last = NewID()
	}
	if string(first[:]) >= string(last[:]) {
		t.Errorf("%v was generated first but does not sort before %v", first, last)
	}
}

func TestIDUnique(t *testing.T) {
	seen := make(map[ID]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := NewID()
		if seen[id] {
			t.Fatalf("NewID returned %v twice after %d draws", id, i)
		}
		seen[id] = true
	}
}

func TestParseIDRejectsGarbage(t *testing.T) {
	for _, s := range []string{
		"",
		"not-a-uuid",
		"0198c1f4-0000-7000-8000-00000000000",   // one char short
		"0198c1f4-0000-7000-8000-0000000000000", // one char long
	} {
		if _, err := ParseID(s); err == nil {
			t.Errorf("ParseID(%q) = nil error, want a rejection", s)
		}
	}
}

func TestEntityIDsAreDistinctTypes(t *testing.T) {
	// This is really a compile-time assertion; the runtime part just checks
	// that the embedded value survives the wrapping.
	b := NewBoxID()
	f := NewFileID()

	if b.IsZero() || f.IsZero() {
		t.Fatal("constructors returned zero IDs")
	}
	if b.String() == f.String() {
		t.Error("two constructors produced the same ID")
	}

	// Method promotion: driver.Valuer and sql.Scanner must reach through the
	// embedded ID, or every query has to unwrap by hand.
	var _ driver.Valuer = b
	var v, err = b.Value()
	if err != nil {
		t.Fatalf("Value(): %v", err)
	}
	raw, ok := v.([]byte)
	if !ok || len(raw) != 16 {
		t.Fatalf("Value() = %#v, want a 16-byte blob", v)
	}

	var got BoxID
	if err := got.Scan(raw); err != nil {
		t.Fatalf("Scan(): %v", err)
	}
	if got != b {
		t.Errorf("Scan round trip gave %v, want %v", got, b)
	}
}
