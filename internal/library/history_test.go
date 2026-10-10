package library

import (
	"context"
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestHistoryPagingDeletedFilesAndUndoRelationships(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	if _, err := l.Import(ctx, writeTree(t, map[string]string{"a.txt": "alpha"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	file := listAll(t, l, box)[0]
	before := snapshotOf(t, l)
	entries, cursor, err := l.History(ctx, ListOpsOptions{File: &file.ID, Limit: 1})
	if err != nil || len(entries) != 1 || cursor != 0 || len(entries[0].Payload) != 0 {
		t.Fatalf("entries=%+v cursor=%d err=%v", entries, cursor, err)
	}
	imported := entries[0].ID
	detail, err := l.HistoryOp(ctx, imported)
	if err != nil || len(detail.Payload) == 0 || !detail.Undoable {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
	if before != snapshotOf(t, l) {
		t.Fatal("history reads changed the library")
	}
	deleted, err := l.Undo(ctx, imported)
	if err != nil {
		t.Fatal(err)
	}
	entries, cursor, err = l.History(ctx, ListOpsOptions{File: &file.ID, Limit: 1})
	if err != nil || len(entries) != 1 || entries[0].ID != deleted || cursor != deleted {
		t.Fatalf("deleted history=%+v cursor=%d err=%v", entries, cursor, err)
	}
	entries, cursor, err = l.History(ctx, ListOpsOptions{File: &file.ID, Before: cursor, Limit: 1})
	if err != nil || len(entries) != 1 || entries[0].ID != imported || entries[0].UndoneBy == nil || *entries[0].UndoneBy != deleted || cursor != 0 {
		t.Fatalf("older history=%+v cursor=%d err=%v", entries, cursor, err)
	}
	restored, err := l.Undo(ctx, deleted)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := l.File(ctx, file.ID); err != nil || !found {
		t.Fatalf("restored file: %v %v", found, err)
	}
	detail, err = l.HistoryOp(ctx, deleted)
	if err != nil || detail.UndoneBy == nil || *detail.UndoneBy != restored {
		t.Fatalf("undo chain=%+v %v", detail, err)
	}
	entries, _, err = l.History(ctx, ListOpsOptions{Action: "delete_files", Box: &box})
	if err != nil || len(entries) != 1 || entries[0].ID != deleted {
		t.Fatalf("filtered=%+v %v", entries, err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	after, _, err := l.History(ctx, ListOpsOptions{Action: "delete_files", Box: &box})
	if err != nil || len(after) != 1 || after[0].UndoneBy == nil || *after[0].UndoneBy != restored {
		t.Fatalf("rebuilt=%+v %v", after, err)
	}
}
func TestHistoryInputAndReceipts(t *testing.T) {
	l := openTestLibrary(t)
	for _, opt := range []ListOpsOptions{{Limit: -1}, {Limit: 1001}, {Before: -1}, {Action: "not-an-action"}} {
		if _, _, err := l.History(t.Context(), opt); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("%+v: %v", opt, err)
		}
	}
	if _, err := l.HistoryOp(t.Context(), 99); !errors.Is(err, ErrNoSuchOp) {
		t.Fatal(err)
	}
	box, err := l.CreateBox(t.Context(), "test", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	entries, _, err := l.History(t.Context(), ListOpsOptions{Box: &box.ID})
	if err != nil || len(entries) != 1 || !entries[0].Undoable {
		t.Fatalf("%+v %v", entries, err)
	}
	receipt, err := l.ExecuteRequest(t.Context(), requestFor("history-receipt", "no change"), func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	detail, err := l.HistoryOp(t.Context(), receipt.OpID)
	if err != nil || detail.Undoable {
		t.Fatalf("receipt=%+v err=%v", detail, err)
	}
}
