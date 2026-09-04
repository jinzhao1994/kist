package domain

import (
	"errors"
	"testing"
)

func TestNewFileFolds(t *testing.T) {
	f, err := NewFile(NewFileID(), NewFolderID(), "IMG_0001.JPG", "sha256:abcd")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if f.NameFold != "img_0001.jpg" {
		t.Errorf("NameFold = %q, want %q", f.NameFold, "img_0001.jpg")
	}
	// Timestamps belong to the op that creates the file, and only the write
	// path knows that op's ts (spec 2.3.1).
	if !f.CreatedAt.IsZero() || !f.UpdatedAt.IsZero() {
		t.Error("NewFile stamped timestamps; they must come from ops.ts")
	}
	if !f.CapturedAt.IsZero() {
		t.Error("NewFile set CapturedAt; EXIF extraction is M3")
	}
}

// A file name that came off a disk is not automatically a legal kist name:
// trailing dots and edge whitespace are dropped or rejected by Windows, so a
// library holding them would not project the same way everywhere (spec 3.4).
// Import turns this into a per-file failure rather than aborting the batch
// (spec 3.7.1 ③), which only works if the check is here.
func TestNewFileRejectsUnprojectableNames(t *testing.T) {
	for _, name := range []string{"", "  ", " leading.jpg", "trailing.jpg ", "dot."} {
		if _, err := NewFile(NewFileID(), NewFolderID(), name, "sha256:abcd"); err == nil {
			t.Errorf("NewFile(%q) was accepted", name)
		}
	}
}

func TestNewFileRequiresBlobKey(t *testing.T) {
	_, err := NewFile(NewFileID(), NewFolderID(), "a.jpg", "")
	if !errors.Is(err, ErrNoBlobKey) {
		t.Errorf("err = %v, want ErrNoBlobKey", err)
	}
}

func TestNewFolderIsNotARoot(t *testing.T) {
	parent := NewFolderID()
	f, err := NewFolder(NewFolderID(), parent, "2019云南")
	if err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	if f.ParentID == nil || *f.ParentID != parent {
		t.Errorf("ParentID = %v, want %v", f.ParentID, parent)
	}
	if f.NameFold != Fold("2019云南") {
		t.Errorf("NameFold = %q", f.NameFold)
	}
	// The root is the one folder with no name; a named folder must never be
	// mistaken for one, because the schema allows exactly one root per box.
	if f.Name == "" {
		t.Error("NewFolder produced a nameless folder")
	}
}

func TestNewFolderRejectsEmptyName(t *testing.T) {
	if _, err := NewFolder(NewFolderID(), NewFolderID(), ""); err == nil {
		t.Error("NewFolder accepted an empty name; that is the root's shape")
	}
}

// spec 3.4: the database stores NFC, always. Names that arrive from a
// filesystem are the reason — macOS hands back NFD — and without normalizing
// here the same café.jpg imported on two machines becomes two rows whose
// name differs and whose name_fold does not.
func TestConstructorsStoreNFC(t *testing.T) {
	nfd := "cafe\u0301.jpg" // e + combining acute, which is what macOS hands back
	nfc := "caf\u00e9.jpg"  // precomposed

	f, err := NewFile(NewFileID(), NewFolderID(), nfd, "sha256:aa")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != nfc {
		t.Errorf("File.Name = %q, want the NFC form %q", f.Name, nfc)
	}

	folder, err := NewFolder(NewFolderID(), NewFolderID(), nfd)
	if err != nil {
		t.Fatal(err)
	}
	if folder.Name != nfc {
		t.Errorf("Folder.Name = %q, want %q", folder.Name, nfc)
	}

	b, err := NewBox(NewBoxID(), NewFolderID(), nfd, nfd)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != nfc || b.Description != nfc {
		t.Errorf("Box = (%q, %q), want both %q", b.Name, b.Description, nfc)
	}

	tag, err := NewTag(NewTagID(), nil, nfd)
	if err != nil {
		t.Fatal(err)
	}
	if tag.Name != nfc {
		t.Errorf("Tag.Name = %q, want %q", tag.Name, nfc)
	}
}
