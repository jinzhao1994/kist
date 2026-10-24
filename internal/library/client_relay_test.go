package library

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestRelayCustodySurvivesRebuildAndForwardsOriginalIdentity(t *testing.T) {
	nas := openTestLibrary(t)
	relay := newSyncClient(t, nas)
	head, _ := nas.SyncHead(t.Context())
	p := syncBox(t, "phone-box")
	first, err := relay.ReceiveRelay(t.Context(), head.LibraryID, p)
	if err != nil || first.Confirmed != nil || !first.Custody.valid() {
		t.Fatalf("custody=%+v err=%v", first, err)
	}
	before, _ := nas.OpCount(t.Context())
	entries, err := relay.queue.snapshot(t.Context())
	if err != nil || len(entries) != 1 || !entries[0].active() || entries[0].Receipt != nil {
		t.Fatalf("custody incorrectly finalized intent: %+v %v", entries, err)
	}
	if !reflect.DeepEqual(entries[0].Proposal, p) {
		t.Fatal("relay rewrote proposal")
	}
	digest, _ := p.digest()
	if err := relay.Close(); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(filepath.Join(relay.root, "client.db") + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	relay, err = OpenClient(t.Context(), relay.root)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	got, err := relay.ReceiveRelay(t.Context(), head.LibraryID, p)
	if err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("retry/rebuild changed receipt: %+v %v", got, err)
	}
	changed := p
	changed.OccurredAt = p.OccurredAt.Add(time.Second)
	if _, err := relay.ReceiveRelay(t.Context(), head.LibraryID, changed); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("accepted reused identity: %v", err)
	}
	if _, err := relay.ReceiveRelay(t.Context(), domain.NewID(), p); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("accepted wrong library: %v", err)
	}
	// A lost final reply is uncertain, never a second root operation.
	if _, err := relay.Synchronize(t.Context(), &testSyncPeer{nas: nas, loseReply: true}); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("lost reply: %v", err)
	}
	got, err = relay.RelayStatus(t.Context(), p.ID, digest)
	if err != nil || !got.Sending || got.Confirmed != nil {
		t.Fatalf("uncertain result: %+v %v", got, err)
	}
	if _, err := relay.Synchronize(t.Context(), &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	got, err = relay.RelayStatus(t.Context(), p.ID, digest)
	if err != nil || got.Confirmed == nil || got.Custody != first.Custody || got.Sending {
		t.Fatalf("final result: %+v %v", got, err)
	}
	direct, err := nas.AcceptSync(t.Context(), p)
	if err != nil || direct != *got.Confirmed {
		t.Fatalf("direct path did not deduplicate: %+v %v", direct, err)
	}
	after, _ := nas.OpCount(t.Context())
	if after != before+1 {
		t.Fatalf("root appended %d operations, want one", after-before)
	}
	if _, err := relay.RelayStatus(t.Context(), p.ID, strings.Repeat("0", 64)); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("status accepted wrong digest: %v", err)
	}
}

func TestRelayOriginalsProtectedUntilRootConfirmation(t *testing.T) {
	nas := openTestLibrary(t)
	phone := newSyncClient(t, nas)
	relay := newSyncClient(t, nas)
	head, _ := nas.SyncHead(t.Context())
	key, fileID := stageAdoptionOriginal(t, phone)
	entries, _ := phone.queue.snapshot(t.Context())
	if _, err := relay.ReceiveRelay(t.Context(), head.LibraryID, entries[0].Proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := relay.ReceiveRelay(t.Context(), head.LibraryID, entries[1].Proposal); err == nil {
		t.Fatal("acknowledged missing original")
	}
	if err := relay.StoreRelayOriginal(t.Context(), string(key), 13, strings.NewReader("wrong content")); err == nil {
		t.Fatal("accepted corrupt original")
	}
	r, err := phone.blobs.Get(t.Context(), key)
	if err != nil {
		t.Fatal(err)
	}
	err = relay.StoreRelayOriginal(t.Context(), string(key), 13, r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	received, err := relay.ReceiveRelay(t.Context(), head.LibraryID, entries[1].Proposal)
	if err != nil || received.Confirmed != nil {
		t.Fatalf("receive: %+v %v", received, err)
	}
	if _, err := relay.PruneCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := relay.AdoptConfirmedOriginals(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := relay.blobs.Stat(t.Context(), key); err != nil {
		t.Fatalf("custody original was evicted: %v", err)
	}
	// Local references survive both hops, including an inverse of a phone edit.
	undo := SyncProposal{Version: 1, ID: "phone-undo", OccurredAt: time.Now().UTC(), Origin: "user", Undo: &SyncReference{LocalID: entries[1].Proposal.ID}}
	if _, err := relay.ReceiveRelay(t.Context(), head.LibraryID, undo); err != nil {
		t.Fatal(err)
	}
	if _, err := relay.Synchronize(t.Context(), &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := nas.File(t.Context(), fileID); err != nil || found {
		t.Fatalf("root undo failed: %v %v", found, err)
	}
	for _, p := range []SyncProposal{entries[0].Proposal, entries[1].Proposal, undo} {
		digest, _ := p.digest()
		outcome, err := relay.RelayStatus(t.Context(), p.ID, digest)
		if err != nil || outcome.Confirmed == nil {
			t.Fatalf("missing final outcome for %s: %+v %v", p.ID, outcome, err)
		}
	}
}

func TestRelayConcurrentRetryAndInterruptedCustody(t *testing.T) {
	nas := openTestLibrary(t)
	relay := newSyncClient(t, nas)
	head, _ := nas.SyncHead(t.Context())
	p := syncBox(t, "interrupted-custody")
	// Simulate a crash after durable enqueue but before the custody record.
	if err := relay.Stage(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	other, err := OpenClient(t.Context(), relay.root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var wg sync.WaitGroup
	results := make([]RelayOutcome, 8)
	for i := range results {
		wg.Go(func() {
			client := []*Client{relay, other}[i%2]
			var err error
			results[i], err = client.ReceiveRelay(t.Context(), head.LibraryID, p)
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	for _, got := range results {
		if got.Custody != results[0].Custody || !got.Custody.valid() {
			t.Fatalf("inconsistent custody: %+v", got)
		}
	}
	last, _, err := relay.queue.log.Last()
	if err != nil || last.ID != 2 {
		t.Fatalf("retry duplicated events: %+v %v", last, err)
	}
}

func TestRelayRecoversDirectRootConfirmationWithoutReply(t *testing.T) {
	nas := openTestLibrary(t)
	relay := newSyncClient(t, nas)
	head, _ := nas.SyncHead(t.Context())
	p := syncBox(t, "phone-direct-root")
	if _, err := relay.ReceiveRelay(t.Context(), head.LibraryID, p); err != nil {
		t.Fatal(err)
	}
	receipt, err := nas.AcceptSync(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if err := relay.pullCatalog(t.Context(), &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	digest, _ := p.digest()
	outcome, err := relay.RelayStatus(t.Context(), p.ID, digest)
	if err != nil || outcome.Confirmed == nil || *outcome.Confirmed != receipt {
		t.Fatalf("lost root reply not recovered: %+v %v", outcome, err)
	}
	entries, _ := relay.queue.snapshot(t.Context())
	if entries[0].Receipt != nil {
		t.Fatal("test should recover from catalog, not a recorded acknowledgement")
	}
}

func TestRelayReportsConflictDiscoveredBeforeSubmission(t *testing.T) {
	nas, boxID := openWithBox(t)
	relay := newSyncClient(t, nas)
	head, _ := nas.SyncHead(t.Context())
	box, _, _ := relay.Box(t.Context(), boxID)
	raw, _ := oplog.Marshal(oplog.UpdateBoxPayload{BoxID: boxID, Name: &oplog.TextChange{Before: box.Name, After: "phone name"}})
	p := SyncProposal{Version: 1, ID: "relay-preflight-conflict", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionUpdateBox), Origin: "user", Payload: raw}
	if _, err := relay.ReceiveRelay(t.Context(), head.LibraryID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.UpdateBox(t.Context(), boxID, oplog.BoxFields{Name: "root name"}); err != nil {
		t.Fatal(err)
	}
	if _, err := relay.Synchronize(t.Context(), &testSyncPeer{nas: nas}); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("expected preflight conflict: %v", err)
	}
	digest, _ := p.digest()
	outcome, err := relay.RelayStatus(t.Context(), p.ID, digest)
	if err != nil || outcome.Conflict == "" || outcome.Sending || outcome.Confirmed != nil {
		t.Fatalf("invisible preflight conflict: %+v %v", outcome, err)
	}
	if err := relay.Resolve(t.Context(), p.ID, nil, "keep root name"); err != nil {
		t.Fatal(err)
	}
	outcome, err = relay.RelayStatus(t.Context(), p.ID, digest)
	if err != nil || !outcome.Discarded || outcome.Reason != "keep root name" || outcome.Confirmed != nil {
		t.Fatalf("discard not retained: %+v %v", outcome, err)
	}
}

func TestRelayConflictResolutionRemainsExplicit(t *testing.T) {
	nas, boxID := openWithBox(t)
	relay := newSyncClient(t, nas)
	head, _ := nas.SyncHead(t.Context())
	box, _, _ := relay.Box(t.Context(), boxID)
	raw, _ := oplog.Marshal(oplog.UpdateBoxPayload{BoxID: boxID, Name: &oplog.TextChange{Before: box.Name, After: "phone name"}})
	p := SyncProposal{Version: 1, ID: "relay-conflict", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionUpdateBox), Origin: "user", Payload: raw}
	if _, err := relay.ReceiveRelay(t.Context(), head.LibraryID, p); err != nil {
		t.Fatal(err)
	}
	peer := &testSyncPeer{nas: nas, beforeSubmit: func() {
		if _, err := nas.UpdateBox(t.Context(), boxID, oplog.BoxFields{Name: "root name"}); err != nil {
			t.Fatal(err)
		}
	}}
	var rejected *SyncRejectedError
	if _, err := relay.Synchronize(t.Context(), peer); !errors.As(err, &rejected) {
		t.Fatalf("expected rejection: %v", err)
	}
	digest, _ := p.digest()
	outcome, err := relay.RelayStatus(t.Context(), p.ID, digest)
	if err != nil || outcome.Conflict == "" || outcome.Confirmed != nil {
		t.Fatalf("conflict=%+v err=%v", outcome, err)
	}
	replacement := p
	replacement.ID = "relay-replacement"
	replacement.Payload, _ = oplog.Marshal(oplog.UpdateBoxPayload{BoxID: boxID, Name: &oplog.TextChange{Before: "root name", After: "chosen name"}})
	if err := relay.Resolve(t.Context(), p.ID, &replacement, "choose replacement"); err != nil {
		t.Fatal(err)
	}
	outcome, err = relay.ReceiveRelay(t.Context(), head.LibraryID, p)
	if err != nil || outcome.Replacement == nil || !reflect.DeepEqual(*outcome.Replacement, replacement) || outcome.Reason != "choose replacement" || outcome.Confirmed != nil {
		t.Fatalf("replacement=%+v err=%v", outcome, err)
	}
	if _, err := relay.Synchronize(t.Context(), peer); err != nil {
		t.Fatal(err)
	}
	got, found, err := nas.Box(t.Context(), boxID)
	if err != nil || !found || got.Name != "chosen name" {
		t.Fatalf("resolution not forwarded: %+v %v", got, err)
	}
}
