package library

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/journal"
)

func TestPendingQueueDurableDecisionsAndRetry(t *testing.T) {
	root := t.TempDir()
	q, err := openPendingQueue(root)
	if err != nil {
		t.Fatal(err)
	}
	first := syncBox(t, "pending-original")
	second := syncBox(t, "pending-second")
	for _, p := range []SyncProposal{first, second} {
		if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &p}); err != nil {
			t.Fatal(err)
		}
	}
	replacement := syncBox(t, "pending-replacement")
	resolve := pendingEvent{Target: first.ID, Proposal: &replacement, Reason: "use revised local choice"}
	if err := q.append(t.Context(), "pending_resolve", resolve); err != nil {
		t.Fatal(err)
	}
	result := SyncResult{OpID: 934, AcceptedAt: time.Now().UTC()}
	ack := pendingEvent{Target: replacement.ID, Receipt: &result}
	if err := q.append(t.Context(), "pending_send", pendingEvent{Target: replacement.ID}); err != nil {
		t.Fatal(err)
	}
	if err := q.append(t.Context(), "pending_ack", ack); err != nil {
		t.Fatal(err)
	}
	discard := pendingEvent{Target: second.ID, Reason: "keep NAS change"}
	if err := q.append(t.Context(), "pending_discard", discard); err != nil {
		t.Fatal(err)
	}
	before, err := q.snapshot(t.Context())
	if err != nil || len(before) != 3 {
		t.Fatalf("snapshot=%+v err=%v", before, err)
	}
	if before[0].ReplacedBy != replacement.ID || before[0].Proposal.ID != first.ID || before[2].Sequence != before[0].Sequence || before[2].Receipt.OpID != 934 || !before[1].Discarded {
		t.Fatalf("lost original intent or decision: %+v", before)
	}
	// A fresh handle has no database or in-memory state to recover from.
	q, err = openPendingQueue(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		action string
		event  pendingEvent
	}{{"pending_enqueue", pendingEvent{Proposal: &first}}, {"pending_resolve", resolve}, {"pending_ack", ack}, {"pending_discard", discard}} {
		if err := q.append(t.Context(), change.action, change.event); err != nil {
			t.Fatalf("retry %s: %v", change.action, err)
		}
	}
	after, err := q.snapshot(t.Context())
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("restart/retry changed state: %v", err)
	}
	last, _, _ := q.log.Last()
	if last.ID != 6 {
		t.Fatalf("retry added events: %d", last.ID)
	}
	changed := first
	changed.OccurredAt = changed.OccurredAt.Add(time.Second)
	if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &changed}); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("reused intent ID: %v", err)
	}
	if err := q.append(t.Context(), "pending_discard", pendingEvent{Target: replacement.ID, Reason: "too late"}); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("discarded acknowledged operation: %v", err)
	}
}

func TestPendingQueueCrossHandleConcurrentRetry(t *testing.T) {
	root := t.TempDir()
	q1, err := openPendingQueue(root)
	if err != nil {
		t.Fatal(err)
	}
	q2, err := openPendingQueue(root)
	if err != nil {
		t.Fatal(err)
	}
	p := syncBox(t, "concurrent-pending")
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			q := []*pendingQueue{q1, q2}[i%2]
			if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &p}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	state, err := q2.snapshot(t.Context())
	if err != nil || len(state) != 1 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestPendingQueueDependenciesAndTornAppend(t *testing.T) {
	q, err := openPendingQueue(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent := syncBox(t, "pending-parent")
	child := syncBox(t, "pending-child")
	child.Requires = []string{parent.ID}
	if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &child}); !errors.Is(err, ErrSyncDependency) {
		t.Fatalf("unknown local dependency: %v", err)
	}
	if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &parent}); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(q.log.Root(), "*", "*", "*_ops.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatalf("journal paths=%v err=%v", files, err)
	}
	f, err := os.OpenFile(files[0], os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"v":2,"id":2`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &child}); err != nil {
		t.Fatal(err)
	}
	state, err := q.snapshot(t.Context())
	if err != nil || len(state) != 2 || state[1].Sequence != 2 {
		t.Fatalf("torn append recovery=%+v err=%v", state, err)
	}
	if err := q.append(t.Context(), "pending_discard", pendingEvent{Target: parent.ID, Reason: "keep NAS"}); err != nil {
		t.Fatal(err)
	}
	third := syncBox(t, "pending-third")
	third.Requires = []string{parent.ID}
	if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &third}); !errors.Is(err, ErrSyncDependency) {
		t.Fatalf("depended on discarded intent: %v", err)
	}
	// Complete, invalid bytes are corruption, not a recoverable torn tail.
	f, err = os.OpenFile(files[0], os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("{}\n")
	f.Close()
	if _, err := q.snapshot(t.Context()); err == nil || !(errors.Is(err, journal.ErrCorrupt) || errors.Is(err, journal.ErrUnsupportedVersion)) {
		t.Fatalf("silently discarded corruption: %v", err)
	}
}

func TestPendingQueueUncertainSendAndConflictBlockLaterIntents(t *testing.T) {
	root := t.TempDir()
	q, err := openPendingQueue(root)
	if err != nil {
		t.Fatal(err)
	}
	first := syncBox(t, "first-send-intent")
	later := syncBox(t, "later-send-intent")
	for _, p := range []SyncProposal{first, later} {
		if err := q.append(t.Context(), "pending_enqueue", pendingEvent{Proposal: &p}); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.append(t.Context(), "pending_send", pendingEvent{Target: later.ID}); !errors.Is(err, ErrSyncDependency) {
		t.Fatalf("sent out of order: %v", err)
	}
	if err := q.append(t.Context(), "pending_send", pendingEvent{Target: first.ID}); err != nil {
		t.Fatal(err)
	}
	q, err = openPendingQueue(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"pending_discard", "pending_resolve"} {
		e := pendingEvent{Target: first.ID, Reason: "cancel after lost response"}
		if action == "pending_resolve" {
			p := syncBox(t, "replacement-after-send")
			e.Proposal = &p
		}
		if err := q.append(t.Context(), action, e); !errors.Is(err, ErrSyncConflict) {
			t.Fatalf("%s changed an uncertain submission: %v", action, err)
		}
	}
	// A confirmed server rejection permits a recorded user resolution. A
	// transport failure must never produce this event.
	rejected := pendingEvent{Target: first.ID, Reason: "NAS name changed"}
	for range 2 {
		if err := q.append(t.Context(), "pending_reject", rejected); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.append(t.Context(), "pending_send", pendingEvent{Target: later.ID}); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("skipped unresolved conflict: %v", err)
	}
	replacement := syncBox(t, "resolved-send-intent")
	if err := q.append(t.Context(), "pending_resolve", pendingEvent{Target: first.ID, Proposal: &replacement, Reason: "choose new name"}); err != nil {
		t.Fatal(err)
	}
	if err := q.append(t.Context(), "pending_send", pendingEvent{Target: later.ID}); !errors.Is(err, ErrSyncDependency) {
		t.Fatalf("resolution lost original position: %v", err)
	}
	if err := q.append(t.Context(), "pending_send", pendingEvent{Target: replacement.ID}); err != nil {
		t.Fatal(err)
	}
	state, err := q.snapshot(t.Context())
	if err != nil || state[0].Conflict != rejected.Reason || state[0].ReplacedBy != replacement.ID || !state[2].Sending {
		t.Fatalf("resolution history=%+v err=%v", state, err)
	}
}
