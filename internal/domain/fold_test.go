package domain

import "testing"

func TestFoldIsCaseInsensitive(t *testing.T) {
	if Fold("Passport") != Fold("passport") {
		t.Error("case must not survive folding")
	}
	if Fold("PASSPORT") != Fold("Passport") {
		t.Error("case must not survive folding")
	}
}

func TestFoldNormalizesUnicode(t *testing.T) {
	// macOS hands out NFD, Linux keeps whatever bytes it was given. Two
	// spellings of the same character must fold to one (spec 3.4).
	nfc := "caf\u00e9"  // é is U+00E9
	nfd := "cafe\u0301" // e followed by combining acute
	if nfc == nfd {
		t.Fatal("test inputs are byte-identical; the case under test is missing")
	}
	if Fold(nfc) != Fold(nfd) {
		t.Errorf("Fold(NFC)=%q != Fold(NFD)=%q", Fold(nfc), Fold(nfd))
	}
}

func TestFoldIsIdempotent(t *testing.T) {
	// The query side folds the pattern with the same function that built the
	// column. If folding twice differed from folding once, a folded value
	// re-entering the pipeline would stop matching itself.
	for _, s := range []string{"Passport", "café", "护照", "ÅNGSTRÖM", ""} {
		if once, twice := Fold(s), Fold(Fold(s)); once != twice {
			t.Errorf("Fold(%q) = %q but Fold(Fold(%q)) = %q", s, once, s, twice)
		}
	}
}

func TestFoldLeavesCJKAlone(t *testing.T) {
	// Chinese has no case and does not decompose; folding must be a no-op
	// rather than mangling it.
	if got := Fold("护照"); got != "护照" {
		t.Errorf("Fold(\"护照\") = %q, want it unchanged", got)
	}
}

func TestFoldPreservesInnerSpacing(t *testing.T) {
	// Fold normalizes representation, it does not clean up input. Rejecting
	// leading/trailing whitespace in names is validation's job (spec 1.4's
	// ghost-directory problem) and must not be smuggled in here, or "a b" and
	// "ab" would become the same tag.
	if Fold("a b") == Fold("ab") {
		t.Error("folding must not collapse spaces")
	}
	if Fold(" x ") == Fold("x") {
		t.Error("folding must not trim; that is validation's job")
	}
}
