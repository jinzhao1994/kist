package domain

import "testing"

func TestNewTagFoldsItsName(t *testing.T) {
	id := NewTagID()
	tag, err := NewTag(id, nil, "Passport")
	if err != nil {
		t.Fatalf("NewTag: %v", err)
	}
	if tag.ParentID != nil {
		t.Error("a root tag should have no parent")
	}
	if tag.NameFold != Fold("Passport") {
		t.Errorf("NameFold = %q, want %q", tag.NameFold, Fold("Passport"))
	}
}

func TestNewTagWithParent(t *testing.T) {
	parent := NewTagID()
	tag, err := NewTag(NewTagID(), &parent, "护照")
	if err != nil {
		t.Fatalf("NewTag: %v", err)
	}
	if tag.ParentID == nil || *tag.ParentID != parent {
		t.Errorf("ParentID = %v, want %v", tag.ParentID, parent)
	}
}

func TestNewTagRejectsBadNames(t *testing.T) {
	if _, err := NewTag(NewTagID(), nil, " "); err == nil {
		t.Error("NewTag with a blank name = nil error, want a rejection")
	}
}
