package library

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestPendingHistoryImportDeleteRestoreAndAcceptance(t *testing.T) {
	nas, box := openWithBox(t)
	old := importOneFile(t, nas, box, "old.txt", "old")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	imported, err := c.Import(ctx, writeTree(t, map[string]string{"new.txt": "new"}), ClientImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := c.PreviewEntryDeletion(ctx, box, old.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := c.DeleteEntry(ctx, box, old.ID.ID, false, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	history, next, err := c.PendingHistory(ctx, "", ListOpsOptions{Box: &box, Limit: 1})
	if err != nil || len(history) != 1 || history[0].ID != deleted.LocalID || next == "" {
		t.Fatalf("first: %+v %q %v", history, next, err)
	}
	older, _, err := c.PendingHistory(ctx, next, ListOpsOptions{Box: &box, Limit: 1})
	if err != nil || len(older) != 1 || older[0].ID != imported.Reference.LocalID {
		t.Fatalf("second: %+v %v", older, err)
	}
	missing := domain.NewBoxID()
	other, _, err := c.PendingHistory(ctx, "", ListOpsOptions{Box: &missing})
	if err != nil || len(other) != 0 {
		t.Fatalf("scope: %+v %v", other, err)
	}
	trash, _, err := c.PendingHistory(ctx, "", TrashOptions(ListOpsOptions{File: &old.ID}))
	if err != nil || len(trash) != 1 || trash[0].Summary == nil || trash[0].Summary.Name != "old.txt" {
		t.Fatalf("trash: %+v %v", trash, err)
	}
	if _, err := c.Undo(ctx, deleted); err != nil {
		t.Fatal(err)
	}
	trash, _, err = c.PendingHistory(ctx, "", TrashOptions(ListOpsOptions{}))
	if err != nil || len(trash) != 0 {
		t.Fatalf("restored trash: %+v %v", trash, err)
	}
	detail, err := c.LocalHistoryOp(ctx, deleted.LocalID)
	if err != nil || detail.UndoneBy == nil || detail.UndoneBy.LocalID == "" {
		t.Fatalf("undo relation: %+v %v", detail, err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	remaining, _, err := c.PendingHistory(ctx, "", ListOpsOptions{})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("duplicated accepted events: %+v %v", remaining, err)
	}
	formal, _, err := c.History(ctx, ListOpsOptions{Box: &box})
	if err != nil || len(formal) < 4 {
		t.Fatalf("confirmed: %+v %v", formal, err)
	}
}

func TestConfirmedTrashHonorsOfflineRestore(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "delete.txt", "original")
	preview, err := nas.PreviewEntryDeletion(t.Context(), box, f.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	id, err := nas.DeleteEntry(t.Context(), box, f.ID.ID, false, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	if _, err := c.Undo(t.Context(), SyncReference{OpID: id}); err != nil {
		t.Fatal(err)
	}
	trash, _, err := c.History(t.Context(), TrashOptions(ListOpsOptions{}))
	if err != nil || len(trash) != 0 {
		t.Fatalf("restored confirmed deletion remains in trash: %+v %v", trash, err)
	}
}
