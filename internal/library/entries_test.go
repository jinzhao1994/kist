package library

import (
	"errors"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"reflect"
	"testing"
)

func TestEntryRenameTrashRestoreRebuild(t *testing.T) {
	l := busyLibrary(t)
	ctx := t.Context()
	boxes, _ := l.Boxes(ctx, ListBoxesOptions{})
	var box domain.BoxID
	for _, b := range boxes {
		if len(listAll(t, l, b.ID)) > 0 {
			box = b.ID
			break
		}
	}
	files := listAll(t, l, box)
	file := files[0]
	op, err := l.RenameEntry(ctx, box, file.ID.ID, false, file.Name, "renamed.txt")
	if err != nil {
		t.Fatal(err)
	}
	renamed, _, _ := l.File(ctx, file.ID)
	if renamed.Name != "renamed.txt" {
		t.Fatal(renamed)
	}
	if _, err = l.RenameEntry(ctx, box, file.ID.ID, false, file.Name, "stale.txt"); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal(err)
	}
	if _, err = l.Undo(ctx, op); err != nil {
		t.Fatal(err)
	}
	if _, err = l.RemoveMetadata(ctx, file.ID, "file:mtime"); err != nil {
		t.Fatal(err)
	}
	before := contentsOf(t, l, box)
	p, err := l.PreviewEntryDeletion(ctx, box, file.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.FileCount != 1 || p.FolderCount != 0 {
		t.Fatal(p)
	}
	deleted, err := l.DeleteEntry(ctx, box, file.ID.ID, false, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := l.File(ctx, file.ID); err != nil || found {
		t.Fatalf("still exists: %v", err)
	}
	checkRebuilt(t, l)
	if _, err = l.Undo(ctx, deleted); err != nil {
		t.Fatal(err)
	}
	after := contentsOf(t, l, box)
	clearDerivedTimes(&before)
	clearDerivedTimes(&after)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("file restoration lost state")
	}
	checkRebuilt(t, l)
}

func TestFolderTrashIncludesSubtreeAndGuardsConfirmation(t *testing.T) {
	l := busyLibrary(t)
	ctx := t.Context()
	boxes, _ := l.Boxes(ctx, ListBoxesOptions{})
	var box domain.BoxID
	var id domain.FolderID
	for _, b := range boxes {
		fs, _ := l.Folders(ctx, b.ID)
		for _, f := range fs {
			if f.ParentID != nil {
				box = b.ID
				id = f.ID
				break
			}
		}
		if !id.IsZero() {
			break
		}
	}
	if id.IsZero() {
		t.Fatal("fixture has no folder")
	}
	before := contentsOf(t, l, box)
	p, err := l.PreviewEntryDeletion(ctx, box, id.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	op, err := l.RenameEntry(ctx, box, id.ID, true, p.Name, "renamed directory")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.DeleteEntry(ctx, box, id.ID, true, p.Revision); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal(err)
	}
	if _, err = l.Undo(ctx, op); err != nil {
		t.Fatal(err)
	}
	p, err = l.PreviewEntryDeletion(ctx, box, id.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := l.DeleteEntry(ctx, box, id.ID, true, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	checkRebuilt(t, l)
	entries, _, err := l.History(ctx, TrashOptions(ListOpsOptions{Box: &box}))
	if err != nil || len(entries) == 0 || entries[0].ID != deleted {
		t.Fatal(entries, err)
	}
	if _, err = l.Undo(ctx, deleted); err != nil {
		t.Fatal(err)
	}
	after := contentsOf(t, l, box)
	clearDerivedTimes(&before)
	clearDerivedTimes(&after)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("folder restoration lost state")
	}
	checkRebuilt(t, l)
	b, _, _ := l.Box(ctx, box)
	if _, err = l.PreviewEntryDeletion(ctx, box, b.RootFolderID.ID, true); err == nil {
		t.Fatal("root deletion allowed")
	}
}

func TestEntryRenameRejectsOccupiedNames(t *testing.T) {
	l := busyLibrary(t)
	ctx := t.Context()
	boxes, _ := l.Boxes(ctx, ListBoxesOptions{})
	box := boxes[0]
	if _, err := l.CreateFolder(ctx, box.ID, box.RootFolderID, "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.CreateFolder(ctx, box.ID, box.RootFolderID, "two"); err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(ctx, box.ID)
	for _, f := range folders {
		if f.Name == "one" {
			if _, err := l.RenameEntry(ctx, box.ID, f.ID.ID, true, "one", "TWO"); !errors.Is(err, oplog.ErrStaleBefore) {
				t.Fatal(err)
			}
		}
	}
	checkRebuilt(t, l)
}

func TestEntryRestoreRejectsOccupiedName(t *testing.T) {
	l := busyLibrary(t)
	ctx := t.Context()
	boxes, _ := l.Boxes(ctx, ListBoxesOptions{})
	box := boxes[0]
	if _, err := l.CreateFolder(ctx, box.ID, box.RootFolderID, "restore conflict"); err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(ctx, box.ID)
	var id domain.FolderID
	for _, f := range folders {
		if f.Name == "restore conflict" {
			id = f.ID
		}
	}
	preview, err := l.PreviewEntryDeletion(ctx, box.ID, id.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := l.DeleteEntry(ctx, box.ID, id.ID, true, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.CreateFolder(ctx, box.ID, box.RootFolderID, "restore conflict"); err != nil {
		t.Fatal(err)
	}
	if _, err = l.Undo(ctx, deleted); err == nil {
		t.Fatal("restore overwrote an occupied name")
	}
	checkRebuilt(t, l)
}
