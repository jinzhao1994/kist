package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientHistoryListsOnlyConfirmedNASOperations(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "old.txt", "history content")
	c := newSyncClient(t, nas)
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.RenameEntry(t.Context(), box, file.ID.ID, false, "old.txt", "pending.txt"); err != nil {
		t.Fatal(err)
	}
	entries, before, err := c.History(t.Context(), ListOpsOptions{Limit: 1})
	if err != nil || len(entries) != 1 || entries[0].ID != head.OpID || before != head.OpID {
		t.Fatal(entries, before, err)
	}
	rest, next, err := c.History(t.Context(), ListOpsOptions{Before: before})
	if err != nil || len(rest) == 0 || next != 0 {
		t.Fatal(rest, next, err)
	}
	entry, err := c.HistoryOp(t.Context(), head.OpID)
	if err != nil || len(entry.Payload) == 0 || len(entry.Files) != 1 || entry.Files[0] != file.ID {
		t.Fatal(entry, err)
	}
	if _, err := c.HistoryOp(t.Context(), head.OpID+1); !errors.Is(err, ErrNoSuchOp) {
		t.Fatal(err)
	}
	summary, err := c.HistorySummary(t.Context(), head.OpID)
	if err != nil || summary.Name != "old.txt" {
		t.Fatal(summary, err)
	}
	// A local inverse is a pending operation, not a confirmed undone_by ID.
	c2 := newSyncClient(t, nas)
	if _, err := c2.Undo(t.Context(), SyncReference{OpID: head.OpID}); err != nil {
		t.Fatal(err)
	}
	original, err := c2.HistoryOp(t.Context(), head.OpID)
	if err != nil || original.UndoneBy != nil {
		t.Fatal(original, err)
	}

}

func TestClientEmptyConfirmedHistoryExcludesPendingObjects(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	if _, err := c.CreateBox(t.Context(), "local only", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	entries, before, err := c.History(t.Context(), ListOpsOptions{})
	if err != nil || len(entries) != 0 || before != 0 {
		t.Fatal(entries, before, err)
	}
	if _, err := c.HistoryOp(t.Context(), 1); !errors.Is(err, ErrNoSuchOp) {
		t.Fatal(err)
	}
	if _, err := c.HistorySummary(t.Context(), 1); !errors.Is(err, ErrNoSuchOp) {
		t.Fatal(err)
	}
	if _, _, err := c.History(t.Context(), ListOpsOptions{Limit: -1}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
}
