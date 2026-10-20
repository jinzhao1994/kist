package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestBoxCoverLifecycle(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	f := importOneFile(t, l, box, "cover.jpg", "picture")
	check := func(visible bool) {
		t.Helper()
		b, _, err := l.Box(ctx, box)
		if err != nil || b.CoverFileID != f.ID {
			t.Fatalf("reference: %+v %v", b, err)
		}
		got, found, err := l.BoxCover(ctx, box)
		if err != nil || found != visible || (found && got.ID != f.ID) {
			t.Fatalf("cover: %+v %v %v", got, found, err)
		}
	}
	if err := l.SetBoxCover(ctx, box, f.ID); err != nil {
		t.Fatal(err)
	}
	setOp := lastOpID(t, l)
	check(true)
	if _, err := l.RenameEntry(ctx, box, f.ID.ID, false, f.Name, "renamed.jpg"); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := l.BoxCover(ctx, box); got.Name != "renamed.jpg" {
		t.Fatal(got)
	}
	if _, err := l.UpdateBox(ctx, box, BoxFields{Name: "new box name"}); err != nil {
		t.Fatal(err)
	}
	check(true)
	// Undo only the cover, even after unrelated edits.
	if _, err := l.Undo(ctx, setOp); err != nil {
		t.Fatal(err)
	}
	b, _, _ := l.Box(ctx, box)
	if !b.CoverFileID.IsZero() || b.Name != "new box name" {
		t.Fatal(b)
	}
	if err := l.SetBoxCover(ctx, box, f.ID); err != nil {
		t.Fatal(err)
	}
	p, err := l.PreviewEntryDeletion(ctx, box, f.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	op, err := l.DeleteEntry(ctx, box, f.ID.ID, false, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	check(false)
	// Removing then undoing the cover while the file is gone must work.
	if err := l.SetBoxCover(ctx, box, domain.FileID{}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(ctx, lastOpID(t, l)); err != nil {
		t.Fatal(err)
	}
	check(false)
	if _, err := l.Undo(ctx, op); err != nil {
		t.Fatal(err)
	}
	check(true)
	trash := trashBox(t, l, box)
	if _, err := l.Undo(ctx, trash); err != nil {
		t.Fatal(err)
	}
	check(true)
	checkRebuilt(t, l)
	check(true)
}

func TestBoxCoverRejectsInvalidReferencesAndStaleUndo(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	f := importOneFile(t, l, box, "cover.jpg", "picture")
	other, err := l.CreateBox(ctx, "other", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	before := lastOpID(t, l)
	if err := l.SetBoxCover(ctx, other.ID, f.ID); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal(err)
	}
	if err := l.SetBoxCover(ctx, box, domain.NewFileID()); !errors.Is(err, ErrNoSuchFile) {
		t.Fatal(err)
	}
	if lastOpID(t, l) != before {
		t.Fatal("invalid cover appended an op")
	}
	if err := l.SetBoxCover(ctx, box, f.ID); err != nil {
		t.Fatal(err)
	}
	op := lastOpID(t, l)
	if err := l.SetBoxCover(ctx, box, f.ID); err != nil || lastOpID(t, l) != op {
		t.Fatal("no-op wrote history", err)
	}
	if err := l.SetBoxCover(ctx, box, domain.FileID{}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(ctx, op); !errors.Is(err, ErrUndoBlocked) {
		t.Fatal(err)
	}
	checkRebuilt(t, l)
}
