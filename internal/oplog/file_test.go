package oplog

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

// seedBox creates a box and its root folder, the shape files need to exist in.
func seedBox(t *testing.T, tx *fakeTx) (domain.BoxID, domain.FolderID) {
	t.Helper()
	p, raw := createBoxPayload(t, "云南")
	if err := apply(t, tx, ActionCreateBox, raw); err != nil {
		t.Fatal(err)
	}
	return p.BoxID, p.RootFolderID
}

// importFixture builds a payload that creates one subfolder and two files.
func importFixture(box domain.BoxID, root domain.FolderID) ImportFilesPayload {
	sub := domain.NewFolderID()
	return ImportFilesPayload{
		BoxID:   box,
		Folders: []ImportedFolder{{FolderID: sub, ParentID: root, Name: "day1"}},
		Files: []ImportedFile{
			{FileID: domain.NewFileID(), FolderID: root, Name: "封面.jpg", BlobKey: "sha256:aa", Sha256: "aa", Size: 10},
			{FileID: domain.NewFileID(), FolderID: sub, Name: "IMG_1.jpg", BlobKey: "sha256:bb", Sha256: "bb", Size: 20},
		},
	}
}

func applyPayload(t *testing.T, tx Tx, action Action, p any) error {
	t.Helper()
	raw, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return apply(t, tx, action, raw)
}

func mustApply(t *testing.T, tx Tx, action Action, p any) {
	t.Helper()
	if err := applyPayload(t, tx, action, p); err != nil {
		t.Fatalf("apply %s: %v", action, err)
	}
}

func TestImportFilesApply(t *testing.T) {
	tx := newFakeTx(opTime)
	box, root := seedBox(t, tx)
	p := importFixture(box, root)

	mustApply(t, tx, ActionImportFiles, p)

	if got := len(tx.files); got != 2 {
		t.Errorf("%d files, want 2", got)
	}
	if got := len(tx.blobs); got != 2 {
		t.Errorf("%d blobs, want 2 — Apply must maintain the blobs index", got)
	}
	f, ok, _ := tx.GetFile(p.Files[1].FileID)
	if !ok {
		t.Fatal("the second file was not created")
	}
	if f.FolderID != p.Folders[0].FolderID {
		t.Error("the second file did not land in the folder this op created")
	}
	if !f.CreatedAt.Equal(opTime) {
		t.Errorf("CreatedAt = %v, want the op's ts %v", f.CreatedAt, opTime)
	}
}

// The inverse of an import is a delete carrying the same bytes: the payload
// already describes everything that would have to be put back.
func TestImportFilesInverse(t *testing.T) {
	raw, err := Marshal(importFixture(domain.NewBoxID(), domain.NewFolderID()))
	if err != nil {
		t.Fatal(err)
	}
	action, inv := inverseOf(t, ActionImportFiles, raw)
	if action != ActionDeleteFiles {
		t.Errorf("inverse action = %q, want %q", action, ActionDeleteFiles)
	}
	if string(inv) != string(raw) {
		t.Error("inverse changed the payload; it should be the same bytes")
	}
}

// Targets is how op_files gets built, and invariant 4 requires it to come from
// the payload alone.
func TestImportFilesTargets(t *testing.T) {
	h, err := Lookup(ActionImportFiles)
	if err != nil {
		t.Fatal(err)
	}
	box := domain.NewBoxID()
	p := importFixture(box, domain.NewFolderID())
	raw, _ := Marshal(p)

	got, err := h.Targets(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Boxes) != 1 || got.Boxes[0] != box {
		t.Errorf("Boxes = %v, want [%v]", got.Boxes, box)
	}
	if len(got.Files) != 2 {
		t.Errorf("Files = %v, want both imported files", got.Files)
	}
}

// Undoing an import is refused rather than partly done. Three things can have
// moved on since: the file was renamed or moved, it was tagged, or somebody
// imported more files into a folder this op created. All three would make a
// silent delete lose information the user cannot get back (spec 2.5).
func TestDeleteFilesRefusesWhenTheFileMoved(t *testing.T) {
	tx := newFakeTx(opTime)
	box, root := seedBox(t, tx)
	p := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, p)

	f := tx.files[p.Files[0].FileID]
	f.Name = "改过名.jpg"
	tx.files[f.ID] = f

	err := applyPayload(t, tx, ActionDeleteFiles, p)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore", err)
	}
}

func TestDeleteFilesRefusesWhenTheFileWasTagged(t *testing.T) {
	tx := newFakeTx(opTime)
	box, root := seedBox(t, tx)
	p := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, p)

	if err := tx.AddFileTag(p.Files[0].FileID, domain.NewTagID(), "user"); err != nil {
		t.Fatal(err)
	}
	err := applyPayload(t, tx, ActionDeleteFiles, p)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore — undoing would silently drop the tag", err)
	}
}

func TestDeleteFilesRefusesWhenTheFolderGainedFiles(t *testing.T) {
	tx := newFakeTx(opTime)
	box, root := seedBox(t, tx)
	p := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, p)

	// A later import dropped something else into the folder this op created.
	later := ImportFilesPayload{
		BoxID: box,
		Files: []ImportedFile{{
			FileID: domain.NewFileID(), FolderID: p.Folders[0].FolderID,
			Name: "later.jpg", BlobKey: "sha256:cc", Sha256: "cc", Size: 1,
		}},
	}
	mustApply(t, tx, ActionImportFiles, later)

	err := applyPayload(t, tx, ActionDeleteFiles, p)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore — the folder is not empty", err)
	}
}

func TestDeleteFilesUndoesACleanImport(t *testing.T) {
	tx := newFakeTx(opTime)
	box, root := seedBox(t, tx)
	p := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, p)
	mustApply(t, tx, ActionDeleteFiles, p)

	if len(tx.files) != 0 {
		t.Errorf("%d files left", len(tx.files))
	}
	if _, ok, _ := tx.GetFolder(p.Folders[0].FolderID); ok {
		t.Error("the folder this import created is still there")
	}
	// Blobs are not deleted: reclaiming them is GC's job, and v0.1 has no GC
	// (spec 3.9). An orphan blob costs space and nothing else.
	if len(tx.blobs) != 2 {
		t.Errorf("%d blobs left, want 2 — deleting files must not touch blob storage", len(tx.blobs))
	}
}

func TestDeleteFilesInverseIsAnImport(t *testing.T) {
	raw, _ := Marshal(importFixture(domain.NewBoxID(), domain.NewFolderID()))
	action, inv := inverseOf(t, ActionDeleteFiles, raw)
	if action != ActionImportFiles {
		t.Errorf("inverse action = %q, want %q", action, ActionImportFiles)
	}
	if string(inv) != string(raw) {
		t.Error("inverse changed the payload")
	}
}

// A box that still holds files cannot be deleted. Without this the user sees
// "FOREIGN KEY constraint failed (787)", which is true, useless and
// frightening — the same bug M1b fixed for tag_boxes.
func TestDeleteBoxRefusesWhenTheBoxHoldsFiles(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "云南")
	if err := apply(t, tx, ActionCreateBox, raw); err != nil {
		t.Fatal(err)
	}
	mustApply(t, tx, ActionImportFiles, importFixture(p.BoxID, p.RootFolderID))

	err := apply(t, tx, ActionDeleteBox, raw)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore rather than a raw foreign key error", err)
	}
}
