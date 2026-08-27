package query

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func fileWith(t *testing.T, captured, name string) domain.File {
	t.Helper()
	f := domain.File{ID: domain.NewFileID(), Name: name, NameFold: domain.Fold(name)}
	if captured != "" {
		f.CapturedAt = instant(t, captured)
	}
	return f
}

func allSortSpecs() []SortSpec {
	var out []SortSpec
	for k := SortKey(0); k < numSortKeys; k++ {
		out = append(out, SortSpec{Key: k}, SortSpec{Key: k, Asc: true})
	}
	return out
}

func TestCursorRoundTrips(t *testing.T) {
	for _, s := range allSortSpecs() {
		f := fileWith(t, "2019-08-12", "IMG_1.jpg")
		key, id, err := DecodeCursor(s, EncodeCursor(s, f))
		if err != nil {
			t.Fatalf("%s: %v", s.ID(), err)
		}
		if id != f.ID {
			t.Errorf("%s: id = %s, want %s", s.ID(), id, f.ID)
		}
		if key != s.Value(f) {
			t.Errorf("%s: key = %q, want %q", s.ID(), key, s.Value(f))
		}
	}
}

// The whole reason the ordering travels inside the cursor. Paging under one
// order and then continuing under another must fail — the alternative is a
// page that looks perfectly normal and is silently wrong, which nobody ever
// notices (spec 4.2.1).
func TestCursorFromAnotherSortIsRefused(t *testing.T) {
	f := fileWith(t, "2019-08-12", "IMG_1.jpg")
	made := SortSpec{Key: SortCaptured}
	c := EncodeCursor(made, f)

	for _, other := range allSortSpecs() {
		if other == made {
			continue
		}
		if _, _, err := DecodeCursor(other, c); !errors.Is(err, ErrCursorSortMismatch) {
			t.Errorf("a %s cursor was accepted for %s (err %v)", made.ID(), other.ID(), err)
		}
	}
}

// A file with no capture time still needs a usable cursor: it is where the
// nulls block begins, and paging has to be able to stop inside it (spec 4.5.1).
func TestCursorForAFileWithNoCaptureTime(t *testing.T) {
	f := fileWith(t, "", "scan.pdf")
	s := SortSpec{Key: SortCaptured}
	key, id, err := DecodeCursor(s, EncodeCursor(s, f))
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		t.Errorf("key = %q, want empty", key)
	}
	if id != f.ID {
		t.Error("the id did not survive")
	}
}

// Rubbish in must be an error, not a panic and not a silent first page.
func TestDecodeRejectsRubbish(t *testing.T) {
	for _, c := range []string{"not base64!!", "", "AAAA", "Zm9v"} {
		if _, _, err := DecodeCursor(SortSpec{Key: SortCaptured}, c); err == nil {
			t.Errorf("%q was accepted as a cursor", c)
		}
	}
}

// The name key is the folded name, so paging agrees with the ordering. A
// cursor holding the raw name would compare against a folded column and skip
// or repeat rows whose case differs (spec 4.8).
func TestNameCursorCarriesTheFoldedName(t *testing.T) {
	f := fileWith(t, "", "IMG_1.jpg")
	if got := (SortSpec{Key: SortName}).Value(f); got != f.NameFold {
		t.Errorf("name sort value = %q, want the folded name %q", got, f.NameFold)
	}
	if f.NameFold == f.Name {
		t.Fatal("the fixture does not actually exercise folding")
	}
}
