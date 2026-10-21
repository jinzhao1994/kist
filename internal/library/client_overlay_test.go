package library

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func overlayQueue(t *testing.T, q *pendingQueue, nas, view *Library) clientOverlay {
	t.Helper()
	entries, err := q.snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	result, err := rebuildClientInto(t.Context(), view.db, nas.ops, pendingState{entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestClientOverlayPendingUndoAndLostAcknowledgement(t *testing.T) {
	nas, view := openTestLibrary(t), openTestLibrary(t)
	q, err := openPendingQueue(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := syncBox(t, "overlay-create-local")
	undo := SyncProposal{Version: 1, ID: "overlay-undo-local", OccurredAt: p.OccurredAt.Add(time.Second), Origin: "user", Undo: &SyncReference{LocalID: p.ID}}
	for _, proposal := range []SyncProposal{p, undo} {
		if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &proposal}); err != nil {
			t.Fatal(err)
		}
	}
	result := overlayQueue(t, q, nas, view)
	if result.Conflict != nil || result.References[1].LocalID != p.ID || result.References[2].LocalID != undo.ID {
		t.Fatalf("local refs=%+v", result)
	}
	created, _ := oplog.Payload[oplog.CreateBoxPayload](p.Payload)
	if _, found, err := view.Box(t.Context(), created.BoxID); err != nil || found {
		t.Fatalf("local undo failed: %v %v", found, err)
	}
	before := snapshotOf(t, view)
	overlayQueue(t, q, nas, view)
	if snapshotOf(t, view) != before {
		t.Fatal("repeated overlay rebuild changed state")
	}
	// Simulate acceptance with a lost acknowledgement. Downloaded history
	// alone proves acceptance; replay must not import the same entity twice.
	accepted, err := nas.AcceptSync(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	result = overlayQueue(t, q, nas, view)
	if result.Conflict != nil || result.References[1].OpID != accepted.OpID || result.References[2].LocalID != undo.ID {
		t.Fatalf("receipt reconciliation=%+v", result)
	}
	if _, err := nas.AcceptSync(t.Context(), undo); err != nil {
		t.Fatal(err)
	}
	result = overlayQueue(t, q, nas, view)
	if result.Conflict != nil || result.References[2].OpID != 2 {
		t.Fatalf("confirmed undo=%+v", result)
	}
	if snapshotOf(t, nas) != snapshotOf(t, view) {
		t.Fatal("client did not converge to authoritative metadata")
	}
}

func TestClientOverlayRebaseConflictAndResolution(t *testing.T) {
	nas, view := openTestLibrary(t), openTestLibrary(t)
	box, err := nas.CreateBox(t.Context(), "before", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	q, err := openPendingQueue(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box.ID, Name: &oplog.TextChange{Before: "before", After: "local"}})
	rename := SyncProposal{Version: 1, ID: "overlay-rename-local", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionUpdateBox), Origin: "user", Payload: raw}
	later := syncBox(t, "overlay-later-create")
	for _, proposal := range []SyncProposal{rename, later} {
		if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &proposal}); err != nil {
			t.Fatal(err)
		}
	}
	result := overlayQueue(t, q, nas, view)
	if result.Conflict != nil {
		t.Fatal(result.Conflict)
	}
	if _, err := nas.UpdateBox(t.Context(), box.ID, oplog.BoxFields{Name: "remote"}); err != nil {
		t.Fatal(err)
	}
	result = overlayQueue(t, q, nas, view)
	if result.BlockedID != rename.ID || !errors.Is(result.Conflict, oplog.ErrStaleBefore) {
		t.Fatalf("rebase should conflict: %+v", result)
	}
	current, _, err := view.Box(t.Context(), box.ID)
	if err != nil || current.Name != "remote" {
		t.Fatalf("partial stale edit leaked: %+v %v", current, err)
	}
	created, _ := oplog.Payload[oplog.CreateBoxPayload](later.Payload)
	if _, found, err := view.Box(t.Context(), created.BoxID); err != nil || found {
		t.Fatalf("skipped conflict: %v %v", found, err)
	}
	rename.ID = "overlay-resolved-local"
	rename.Payload, _ = oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box.ID, Name: &oplog.TextChange{Before: "remote", After: "local choice"}})
	if err := q.append(t.Context(), "pending_resolve", pendingEvent{Target: result.BlockedID, Proposal: &rename, Reason: "choose local name"}); err != nil {
		t.Fatal(err)
	}
	result = overlayQueue(t, q, nas, view)
	if result.Conflict != nil {
		t.Fatal(result.Conflict)
	}
	current, _, err = view.Box(t.Context(), box.ID)
	if err != nil || current.Name != "local choice" || result.References[3].LocalID != rename.ID || result.References[4].LocalID != later.ID {
		t.Fatalf("resolution lost ordering: %+v %+v %v", result, current, err)
	}
}

func TestClientOverlayUncachedOriginalAndStrictUndo(t *testing.T) {
	nas, box := openWithBox(t)
	view := openTestLibrary(t)
	f := importOneFile(t, nas, box, "uncached.txt", "only NAS stores these bytes")
	imported, err := nas.OpCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	q, err := openPendingQueue(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result := overlayQueue(t, q, nas, view)
	if result.Conflict != nil {
		t.Fatal(result.Conflict)
	}
	if _, found, err := view.File(t.Context(), f.ID); err != nil || !found {
		t.Fatalf("uncached file missing from catalog: %v %v", found, err)
	}
	// No original has been copied to the view's content directory.
	entries, err := os.ReadDir(view.layout.Blobs())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "tmp" {
			t.Fatalf("unexpected downloaded content: %s", entry.Name())
		}
	}
	undo := SyncProposal{Version: 1, ID: "uncached-import-undo", OccurredAt: time.Now().UTC(), Origin: "user", Undo: &SyncReference{OpID: imported}}
	if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &undo}); err != nil {
		t.Fatal(err)
	}
	result = overlayQueue(t, q, nas, view)
	if result.Conflict != nil {
		t.Fatal(result.Conflict)
	}
	if _, err := nas.RemoveMetadata(t.Context(), f.ID, "file:mtime"); err != nil {
		t.Fatal(err)
	}
	result = overlayQueue(t, q, nas, view)
	if result.BlockedID != undo.ID || !errors.Is(result.Conflict, ErrUndoBlocked) {
		t.Fatalf("rebase erased later metadata: %+v", result)
	}
	if _, found, err := view.File(t.Context(), f.ID); err != nil || !found {
		t.Fatalf("blocked undo deleted uncached file: %v %v", found, err)
	}
	before := snapshotOf(t, view)
	bad, err := journal.NewDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := bad.Append(journal.Line{ID: 1, TS: journal.FormatTime(time.Now()), Action: "unsupported", Origin: "user", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := rebuildClientInto(t.Context(), view.db, bad, pendingState{}); err == nil {
		t.Fatal("accepted unreadable catalog")
	}
	if snapshotOf(t, view) != before {
		t.Fatal("failed rebuild replaced the last usable view")
	}
}
