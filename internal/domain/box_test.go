package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewBoxFoldsItsName(t *testing.T) {
	id := NewBoxID()
	b, err := NewBox(id, "Yunnan 2019", "the long way round")
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	if b.ID != id {
		t.Errorf("ID = %v, want %v", b.ID, id)
	}
	if b.NameFold != Fold("Yunnan 2019") {
		t.Errorf("NameFold = %q, want %q", b.NameFold, Fold("Yunnan 2019"))
	}
}

func TestNewBoxRejectsBadNames(t *testing.T) {
	for _, name := range []string{
		"",
		"   ",
		" leading",
		"trailing ",
		"trailing.",
	} {
		if _, err := NewBox(NewBoxID(), name, ""); err == nil {
			t.Errorf("NewBox(%q) = nil error, want a rejection", name)
		}
	}
}

func TestBoxDatesAreOptional(t *testing.T) {
	b, err := NewBox(NewBoxID(), "undated", "")
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	if !b.StartAt.IsZero() || !b.EndAt.IsZero() {
		t.Error("a fresh box should have no dates set")
	}
}

func TestValidateName(t *testing.T) {
	ok := []string{
		"婚礼",
		"a",
		"IMG_1234",
		"a b",           // inner spaces are fine
		"file.name.txt", // inner dots are fine
	}
	for _, n := range ok {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", n, err)
		}
	}

	bad := []struct {
		name string
		want error
	}{
		{"", ErrEmptyName},
		{"  ", ErrEmptyName}, // whitespace-only is empty
		{" x", ErrNameHasEdgeSpace},
		{"x ", ErrNameHasEdgeSpace},
		{"x\t", ErrNameHasEdgeSpace},
		{"x.", ErrNameTrailingDot},
	}
	for _, tt := range bad {
		err := ValidateName(tt.name)
		if err == nil {
			t.Errorf("ValidateName(%q) = nil, want %v", tt.name, tt.want)
			continue
		}
		if !errors.Is(err, tt.want) {
			t.Errorf("ValidateName(%q) = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestValidateNameRejectsOverlyLongNames(t *testing.T) {
	// The limit is on runes, not bytes: a 255-character Chinese name is
	// 765 bytes and would still be a perfectly reasonable name, while a
	// byte limit would cut it off at 85 characters (spec 3.4 keeps DB names
	// clean and lets the projection layer deal with filesystem limits).
	if err := ValidateName(strings.Repeat("好", 255)); err != nil {
		t.Errorf("a 255-rune name was rejected: %v", err)
	}
	if err := ValidateName(strings.Repeat("好", 256)); !errors.Is(err, ErrNameTooLong) {
		t.Errorf("a 256-rune name gave %v, want ErrNameTooLong", err)
	}
}
