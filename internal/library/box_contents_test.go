package library

import (
	"errors"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
	"reflect"
	"testing"
	"time"
)

func contentsOf(t *testing.T, l *Library, id domain.BoxID) domain.BoxContents {
	t.Helper()
	var s domain.BoxContents
	if err := l.readTx(t.Context(), func(tx *store.ViewTx) error {
		var found bool
		var err error
		s, found, err = tx.BoxContents(id)
		if err == nil && !found {
			t.Fatal("missing box")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return s
}
func trashBox(t *testing.T, l *Library, id domain.BoxID) int64 {
	t.Helper()
	p, err := l.PreviewBoxDeletion(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	op, err := l.DeleteBox(t.Context(), id, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return op
}
func clearDerivedTimes(s *domain.BoxContents) {
	s.Box.UpdatedAt = time.Time{}
	for i := range s.Folders {
		s.Folders[i].UpdatedAt = time.Time{}
	}
}
func checkRebuilt(t *testing.T, l *Library) {
	t.Helper()
	before := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, l) != before {
		t.Fatal("rebuild changed state")
	}
	r, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() > 0 {
		t.Fatal(report(t, r))
	}
}
func TestBoxTrashRestoresCompleteContents(t *testing.T) {
	l := busyLibrary(t)
	boxes, err := l.Boxes(t.Context(), ListBoxesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var box domain.BoxID
	for _, b := range boxes {
		if len(listAll(t, l, b.ID)) > 0 {
			box = b.ID
			break
		}
	}
	files := listAll(t, l, box)
	if _, err = l.RemoveMetadata(t.Context(), files[0].ID, "file:mtime"); err != nil {
		t.Fatal(err)
	}
	before := contentsOf(t, l, box)
	deleted := trashBox(t, l, box)
	if _, found, err := l.Box(t.Context(), box); err != nil || found {
		t.Fatalf("box remains: %v", err)
	}
	for _, f := range files {
		if _, found, err := l.File(t.Context(), f.ID); err != nil || found {
			t.Fatalf("file remains: %v", err)
		}
	}
	entries, _, err := l.History(t.Context(), TrashOptions(ListOpsOptions{Box: &box}))
	if err != nil || len(entries) != 1 || entries[0].ID != deleted {
		t.Fatalf("trash: %+v %v", entries, err)
	}
	detail, err := l.HistoryOp(t.Context(), deleted)
	if err != nil || len(detail.Files) != len(files) {
		t.Fatalf("targets: %+v %v", detail, err)
	}
	if err = l.ViewAt(t.Context(), deleted-1, func(v *HistoricalView) error {
		_, f, err := v.File(t.Context(), files[0].ID)
		if !f {
			t.Error("missing historical file")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	checkRebuilt(t, l)
	restored, err := l.Undo(t.Context(), deleted)
	if err != nil {
		t.Fatal(err)
	}
	after := contentsOf(t, l, box)
	clearDerivedTimes(&before)
	clearDerivedTimes(&after)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("restore lost state\nbefore=%+v\nafter=%+v", before, after)
	}
	entries, _, err = l.History(t.Context(), TrashOptions(ListOpsOptions{Box: &box}))
	if err != nil || len(entries) != 0 {
		t.Fatalf("restored trash: %+v %v", entries, err)
	}
	checkRebuilt(t, l)
	// Redo the deletion, then restore once more. Derived timestamps must not
	// spuriously block this chain, and the trash must represent the new event.
	again, err := l.Undo(t.Context(), restored)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.Undo(t.Context(), again); err != nil {
		t.Fatal(err)
	}
	checkRebuilt(t, l)
}
func TestBoxTrashRejectsStaleConfirmationAndRedo(t *testing.T) {
	l, box := openWithBox(t)
	preview, err := l.PreviewBoxDeletion(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	importOneFile(t, l, box, "a.txt", "content")
	before := snapshotOf(t, l)
	if _, err = l.DeleteBox(t.Context(), box, preview.Revision); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatalf("stale confirmation: %v", err)
	}
	if before != snapshotOf(t, l) {
		t.Fatal("stale confirmation changed state")
	}
	deleted := trashBox(t, l, box)
	restored, err := l.Undo(t.Context(), deleted)
	if err != nil {
		t.Fatal(err)
	}
	importOneFile(t, l, box, "b.txt", "new content")
	before = snapshotOf(t, l)
	if _, err = l.Undo(t.Context(), restored); !errors.Is(err, ErrUndoBlocked) {
		t.Fatalf("redo swallowed new file: %v", err)
	}
	if before != snapshotOf(t, l) {
		t.Fatal("blocked redo changed state")
	}
}
func TestBoxRestoreMissingTagIsAtomic(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.txt", "content")
	tag, err := l.CreateTag(t.Context(), "retired", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.TagFiles(t.Context(), tag.ID, []domain.FileID{file.ID}); err != nil {
		t.Fatal(err)
	}
	deleted := trashBox(t, l, box)
	if err = l.DeleteTag(t.Context(), tag.ID); err != nil {
		t.Fatal(err)
	}
	before := snapshotOf(t, l)
	if _, err = l.Undo(t.Context(), deleted); !errors.Is(err, ErrUndoBlocked) {
		t.Fatalf("missing tag restore: %v", err)
	}
	if before != snapshotOf(t, l) {
		t.Fatal("failed restore left partial rows")
	}
}
func TestBoxTrashCrashRecovery(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(map[bool]string{false: "delete", true: "restore"}[restore], func(t *testing.T) {
			l, box := openWithBox(t)
			importOneFile(t, l, box, "a.txt", "recover")
			var deleted int64
			if restore {
				deleted = trashBox(t, l, box)
			}
			p, _ := l.PreviewBoxDeletion(t.Context(), box)
			l.hooks.afterAppend = func() error { return errors.New("crash after fsync") }
			var err error
			if restore {
				_, err = l.Undo(t.Context(), deleted)
			} else {
				_, err = l.DeleteBox(t.Context(), box, p.Revision)
			}
			var outcome *WriteOutcomeError
			if !errors.As(err, &outcome) || outcome.OpID == 0 {
				t.Fatalf("outcome: %v", err)
			}
			l.hooks.afterAppend = nil
			if _, err = l.CreateTag(t.Context(), "recover writer", nil); err != nil {
				t.Fatal(err)
			}
			if _, found, err := l.Box(t.Context(), box); err != nil || found != restore {
				t.Fatalf("recovery found=%v err=%v", found, err)
			}
			// Journal recovery restores authoritative views; projections are
			// explicitly repaired by relink after a crash.
			if _, err := l.Relink(t.Context()); err != nil {
				t.Fatal(err)
			}
			checkRebuilt(t, l)
		})
	}
}
func TestEmptyBoxTrash(t *testing.T) {
	l, box := openWithBox(t)
	deleted := trashBox(t, l, box)
	if _, err := l.Undo(t.Context(), deleted); err != nil {
		t.Fatal(err)
	}
	checkRebuilt(t, l)
}
