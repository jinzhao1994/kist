package library

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestUndoImportPreservesLaterMetadata(t *testing.T) {
	for _, mode := range []string{"extracted", "empty extraction", "tombstone"} {
		t.Run(mode, func(t *testing.T) {
			l, box := openWithBox(t)
			f := importOneFile(t, l, box, "a.txt", "original")
			imported, _ := l.OpCount(t.Context())
			if mode == "tombstone" {
				if _, err := l.RemoveMetadata(t.Context(), f.ID, "file:mtime"); err != nil {
					t.Fatal(err)
				}
			} else {
				e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"}}
				if mode == "extracted" {
					e.tags = []extract.KV{text("fake:description", "keep me")}
				}
				extractWith(t, l, e)
			}
			changed, _ := l.OpCount(t.Context())
			if _, err := l.UpdateBox(t.Context(), box, BoxFields{Name: "unrelated rename"}); err != nil {
				t.Fatal(err)
			}
			before := snapshotOf(t, l)
			journal := historyJournalHashes(t, l)
			_, err := l.Undo(t.Context(), imported)
			var blocked *BlockedUndoError
			if !errors.As(err, &blocked) || blocked.BlockingOp != changed {
				t.Fatalf("undo = %v; want metadata blocker #%d", err, changed)
			}
			if after := snapshotOf(t, l); before != after {
				t.Fatal("refused undo changed the database")
			}
			if !reflect.DeepEqual(journal, historyJournalHashes(t, l)) {
				t.Fatal("refused undo appended to the journal")
			}
			if _, err := l.Undo(t.Context(), changed); err != nil {
				t.Fatal(err)
			}
			deleted, err := l.Undo(t.Context(), imported)
			if err != nil {
				t.Fatalf("undo after restoring import state: %v", err)
			}
			if _, err := l.Undo(t.Context(), deleted); err != nil {
				t.Fatalf("restore: %v", err)
			}
		})
	}
}

func TestLegacyImportUndoWithMetadataStillReplays(t *testing.T) {
	l, box := openWithBox(t)
	f := importOneFile(t, l, box, "a.txt", "legacy content")
	imported, _ := l.OpCount(t.Context())
	if _, err := l.RemoveMetadata(t.Context(), f.ID, "file:mtime"); err != nil {
		t.Fatal(err)
	}
	beforeDelete, _ := l.OpCount(t.Context())
	row, _, err := store.GetOp(l.db, imported)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a durable event written before the command guard existed. Do
	// not route this fixture through today's command validation.
	legacy := store.OpRow{ID: beforeDelete + 1, TS: l.now().UTC(),
		Action: string(oplog.ActionDeleteFiles), Origin: string(oplog.OriginUser),
		Payload: row.Payload, RevertsID: &imported}
	if err := l.ops.Append(lineFromRow(legacy)); err != nil {
		t.Fatal(err)
	}
	// The next command must catch up that old deletion before appending.
	if _, err := l.CreateTag(t.Context(), "after recovery", nil); err != nil {
		t.Fatalf("catch-up rejected legacy delete: %v", err)
	}
	if _, found, err := l.File(t.Context(), f.ID); err != nil || found {
		t.Fatalf("legacy deleted file: found=%v err=%v", found, err)
	}
	want := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := snapshotOf(t, l); got != want {
		t.Fatal("legacy rebuild changed the materialized state")
	}
	if err := l.ViewAt(t.Context(), beforeDelete, func(v *HistoricalView) error {
		rows, err := v.FileMetadata(t.Context(), f.ID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Key == "file:mtime" && row.Tombstone {
				return nil
			}
		}
		t.Error("history lost the suppressed metadata")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Tail recovery rebuilds the index, not the human-readable projection.
	if _, err := l.Relink(t.Context()); err != nil {
		t.Fatal(err)
	}
	r, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || r.Total() != 0 {
		t.Fatalf("legacy deep verify: report=%+v err=%v", r, err)
	}
}

func TestImportUndoChecksMetadataAfterCatchUpAndRefusesWholeBatch(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "first", "b.txt": "second"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	imported, _ := l.OpCount(t.Context())
	files := listAll(t, l, box)
	l.hooks.afterAppend = func() error { return errors.New("simulated crash") }
	_, err := l.RemoveMetadata(t.Context(), files[1].ID, "file:mtime")
	var outcome *WriteOutcomeError
	if !errors.As(err, &outcome) {
		t.Fatalf("expected durable but unapplied edit: %v", err)
	}
	l.hooks.afterAppend = nil
	journal := historyJournalHashes(t, l)
	_, err = l.Undo(t.Context(), imported)
	var blocked *BlockedUndoError
	if !errors.As(err, &blocked) || blocked.BlockingOp != imported+1 {
		t.Fatalf("undo failed to check recovered metadata: %v", err)
	}
	if len(listAll(t, l, box)) != 2 {
		t.Fatal("undo deleted part of the batch")
	}
	if !reflect.DeepEqual(journal, historyJournalHashes(t, l)) {
		t.Fatal("refused undo appended an event")
	}
	if !metaOf(t, l, files[1].ID)["file:mtime"].Tombstone {
		t.Fatal("recovered user edit was lost")
	}
}
