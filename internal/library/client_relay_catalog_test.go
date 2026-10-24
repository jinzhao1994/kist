package library

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

type testRelayPeer struct {
	*Client
	loseReply bool
	failAfter int
	calls     int
}

func (p *testRelayPeer) HasOriginal(ctx context.Context, key string) (bool, error) {
	s, err := (&Library{blobs: p.blobs}).SyncOriginal(ctx, key)
	return s.Present, err
}
func (p *testRelayPeer) UploadOriginal(ctx context.Context, key string, size int64, r io.ReadSeeker) error {
	return p.StoreRelayOriginal(ctx, key, size, r)
}
func (p *testRelayPeer) ReceiveRelay(ctx context.Context, id domain.ID, proposal SyncProposal) (RelayOutcome, error) {
	result, err := p.Client.ReceiveRelay(ctx, id, proposal)
	if err == nil && p.loseReply {
		p.loseReply = false
		return RelayOutcome{}, io.ErrUnexpectedEOF
	}
	return result, err
}
func (p *testRelayPeer) RelayCatalog(ctx context.Context, h RelayHead, after, limit, bytes int) (RelayPage, error) {
	p.calls++
	if p.failAfter > 0 && p.calls == p.failAfter {
		return RelayPage{}, io.ErrUnexpectedEOF
	}
	return p.Client.RelayCatalog(ctx, h, after, limit, bytes)
}

func TestRelayExchangePhoneLaptopRoot(t *testing.T) {
	ctx := t.Context()
	root := openTestLibrary(t)
	laptop := newSyncClient(t, root)
	phone := newSyncClient(t, root)
	parent := syncBox(t, "laptop-trip")
	if err := laptop.Stage(ctx, parent); err != nil {
		t.Fatal(err)
	}
	peer := &testRelayPeer{Client: laptop}
	result, err := phone.SynchronizeRelay(ctx, peer)
	if err != nil || result.AwaitingRoot != 1 {
		t.Fatalf("first snapshot %+v %v", result, err)
	}
	b, _ := oplog.Payload[oplog.CreateBoxPayload](parent.Payload)
	if _, found, err := phone.Box(ctx, b.BoxID); err != nil || !found {
		t.Fatalf("laptop edit not visible: %v %v", found, err)
	}
	content, err := phone.PutOriginal(ctx, strings.NewReader("phone photo"))
	if err != nil {
		t.Fatal(err)
	}
	file := oplog.ImportedFile{FileID: domain.NewFileID(), FolderID: b.RootFolderID, Name: "phone.txt", BlobKey: string(content.Key), Sha256: content.Key.Hex(), Size: content.Size}
	raw, _ := oplog.Marshal(oplog.ImportFilesPayload{BoxID: b.BoxID, Files: []oplog.ImportedFile{file}})
	imported := SyncProposal{Version: 1, ID: "phone-photo", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionImportFiles), Origin: "user", Payload: raw, Requires: []string{parent.ID}}
	if err := phone.Stage(ctx, imported); err != nil {
		t.Fatal(err)
	}
	peer.loseReply = true
	if _, err := phone.SynchronizeRelay(ctx, peer); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("lost custody reply %v", err)
	}
	result, err = phone.SynchronizeRelay(ctx, peer)
	if err != nil || result.AwaitingRoot != 2 {
		t.Fatalf("retry %+v %v", result, err)
	}
	entries, _ := phone.queue.snapshot(ctx)
	if entries[1].Receipt != nil || entries[1].Upstream == nil || entries[1].Sending {
		t.Fatal("relay receipt became root confirmation")
	}
	if _, found, err := laptop.File(ctx, file.FileID); err != nil || !found {
		t.Fatalf("phone edit not visible on laptop: %v %v", found, err)
	}
	if _, err := phone.PruneCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := phone.blobs.Stat(ctx, content.Key); err != nil {
		t.Fatalf("unconfirmed phone original evicted: %v", err)
	}
	undo := SyncProposal{Version: 1, ID: "phone-photo-undo", OccurredAt: time.Now().UTC(), Origin: "user", Undo: &SyncReference{LocalID: imported.ID}}
	if err := phone.Stage(ctx, undo); err != nil {
		t.Fatal(err)
	}
	if _, err := phone.SynchronizeRelay(ctx, peer); err != nil {
		t.Fatal(err)
	}
	if _, err := laptop.Synchronize(ctx, &testSyncPeer{nas: root}); err != nil {
		t.Fatal(err)
	}
	result, err = phone.SynchronizeRelay(ctx, peer)
	if err != nil || result.AwaitingRoot != 0 {
		t.Fatalf("root outcome not returned %+v %v", result, err)
	}
	if snapshotOf(t, root) != snapshotOf(t, &Library{db: phone.db}) {
		t.Fatal("phone did not converge")
	}
	if _, found, err := root.File(ctx, file.FileID); err != nil || found {
		t.Fatalf("undo failed: %v %v", found, err)
	}
	before, _ := root.OpCount(ctx)
	if _, err := phone.Synchronize(ctx, &testSyncPeer{nas: root}); err != nil {
		t.Fatal(err)
	}
	after, _ := root.OpCount(ctx)
	if after != before {
		t.Fatal("direct root sync duplicated imported operations")
	}
}

func TestRelaySnapshotResumePublicationAndRebuild(t *testing.T) {
	ctx := t.Context()
	root := openTestLibrary(t)
	parent := newSyncClient(t, root)
	child := newSyncClient(t, root)
	first := syncBox(t, "parent-first")
	second := syncBox(t, "parent-second")
	for _, p := range []SyncProposal{first, second} {
		if err := parent.Stage(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	head, err := parent.RelayHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	page, err := parent.RelayCatalog(ctx, head, 0, 1, 1)
	if err != nil || page.Done {
		t.Fatal(page, err)
	}
	if err := child.ReceiveRelayPage(ctx, page); err != nil {
		t.Fatal(err)
	}
	b, _ := oplog.Payload[oplog.CreateBoxPayload](first.Payload)
	if _, found, _ := child.Box(ctx, b.BoxID); found {
		t.Fatal("incomplete overlay published")
	}
	own := syncBox(t, "child-during-transfer")
	if err := child.Stage(ctx, own); err != nil {
		t.Fatal(err)
	}
	// The fixed snapshot remains readable after the parent appends more edits.
	if err := parent.Stage(ctx, syncBox(t, "parent-later")); err != nil {
		t.Fatal(err)
	}
	rootPath := child.root
	if err := child.Close(); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(filepath.Join(rootPath, "client.db") + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	child, err = OpenClient(ctx, rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	saved, after, err := child.RelayTransfer(ctx)
	if err != nil || saved == nil || *saved != head || after != 1 {
		t.Fatalf("resume lost %+v %d %v", saved, after, err)
	}
	if err := child.ReceiveRelayPage(ctx, page); err != nil {
		t.Fatal("repeated page", err)
	}
	last, err := parent.RelayCatalog(ctx, head, 1, 1, 1)
	if err != nil || !last.Done {
		t.Fatal(last, err)
	}
	if err := child.ReceiveRelayPage(ctx, last); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := child.Box(ctx, b.BoxID); !found {
		t.Fatal("complete snapshot missing")
	}
	ownBox, _ := oplog.Payload[oplog.CreateBoxPayload](own.Payload)
	if _, found, _ := child.Box(ctx, ownBox.BoxID); !found {
		t.Fatal("lost concurrent local edit")
	}
	if err := child.ReceiveRelayPage(ctx, last); err != nil {
		t.Fatal("repeated final page", err)
	}
	tampered := last
	tampered.Items = append([]RelayItem(nil), last.Items...)
	tampered.Items[0].Proposal.OccurredAt = tampered.Items[0].Proposal.OccurredAt.Add(time.Second)
	if err := child.ReceiveRelayPage(ctx, tampered); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("accepted changed retry: %v", err)
	}
	// The child must not forward a parent's unconfirmed overlay directly to root.
	before, _ := root.OpCount(ctx)
	if _, err := child.Synchronize(ctx, &testSyncPeer{nas: root}); !errors.Is(err, ErrSyncDependency) {
		t.Fatalf("unsafe direct handover: %v", err)
	}
	afterRoot, _ := root.OpCount(ctx)
	if before != afterRoot {
		t.Fatal("root changed on unsafe handover")
	}
	if err := child.Resolve(ctx, first.ID, nil, "drop parent operation"); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("resolved upstream intent locally: %v", err)
	}
}

func TestRelayConflictReplacementFlowsDownstream(t *testing.T) {
	ctx := t.Context()
	root, box := openWithBox(t)
	parent := newSyncClient(t, root)
	child := newSyncClient(t, root)
	peer := &testRelayPeer{Client: parent}
	b, _, _ := parent.Box(ctx, box)
	raw, _ := oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box, Name: &oplog.TextChange{Before: b.Name, After: "phone name"}})
	p := SyncProposal{Version: 1, ID: "child-rename", OccurredAt: time.Now().UTC(), Origin: "user", Action: string(oplog.ActionUpdateBox), Payload: raw}
	if err := child.Stage(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := child.SynchronizeRelay(ctx, peer); err != nil {
		t.Fatal(err)
	}
	if _, err := root.UpdateBox(ctx, box, oplog.BoxFields{Name: "root name"}); err != nil {
		t.Fatal(err)
	}
	if _, err := parent.Synchronize(ctx, &testSyncPeer{nas: root}); !errors.Is(err, ErrSyncConflict) {
		t.Fatal(err)
	}
	if _, err := child.SynchronizeRelay(ctx, peer); err != nil {
		t.Fatal(err)
	}
	status, err := child.Status(ctx)
	if err != nil || status.BlockedID != p.ID {
		t.Fatalf("conflict missing: %+v %v", status, err)
	}
	replacement := p
	replacement.ID = "parent-resolution"
	replacement.Payload, _ = oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box, Name: &oplog.TextChange{Before: "root name", After: "chosen name"}})
	if err := parent.Resolve(ctx, p.ID, &replacement, "choose current before-state"); err != nil {
		t.Fatal(err)
	}
	if _, err := child.SynchronizeRelay(ctx, peer); err != nil {
		t.Fatal(err)
	}
	entries, _ := child.queue.snapshot(ctx)
	if entries[0].ReplacedBy != replacement.ID || entries[1].Proposal.ID != replacement.ID {
		t.Fatal("replacement lost")
	}
	if _, err := parent.Synchronize(ctx, &testSyncPeer{nas: root}); err != nil {
		t.Fatal(err)
	}
	if result, err := child.SynchronizeRelay(ctx, peer); err != nil || result.AwaitingRoot != 0 {
		t.Fatal(result, err)
	}
	if snapshotOf(t, root) != snapshotOf(t, &Library{db: child.db}) {
		t.Fatal("resolution did not converge")
	}
}

func TestRelayMultipleHopsAndIdlePolling(t *testing.T) {
	ctx := t.Context()
	root := openTestLibrary(t)
	a := newSyncClient(t, root)
	b := newSyncClient(t, root)
	c := newSyncClient(t, root)
	pa, pb := &testRelayPeer{Client: a}, &testRelayPeer{Client: b}
	p := syncBox(t, "top-relay-box")
	if err := a.Stage(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SynchronizeRelay(ctx, pa); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SynchronizeRelay(ctx, pb); err != nil {
		t.Fatal(err)
	}
	child := syncBox(t, "bottom-relay-box")
	child.Requires = []string{p.ID}
	if err := c.Stage(ctx, child); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SynchronizeRelay(ctx, pb); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SynchronizeRelay(ctx, pa); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Synchronize(ctx, &testSyncPeer{nas: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SynchronizeRelay(ctx, pa); err != nil {
		t.Fatal(err)
	}
	if result, err := c.SynchronizeRelay(ctx, pb); err != nil || result.AwaitingRoot != 0 {
		t.Fatal(result, err)
	}
	if snapshotOf(t, root) != snapshotOf(t, &Library{db: c.db}) {
		t.Fatal("multi-hop state differs")
	}
	before, _, _ := c.queue.log.Last()
	calls := pb.calls
	for range 3 {
		if _, err := c.SynchronizeRelay(ctx, pb); err != nil {
			t.Fatal(err)
		}
	}
	after, _, _ := c.queue.log.Last()
	if before.ID != after.ID {
		t.Fatal("idle polling appended journal events")
	}
	if pb.calls != calls {
		t.Fatal("idle polling downloaded unchanged snapshots")
	}
}

func TestRelayRejectsInvalidSnapshotWithoutPublishing(t *testing.T) {
	for _, kind := range []string{"receipt", "duplicate", "dependency", "node", "root", "replacement"} {
		t.Run(kind, func(t *testing.T) {
			ctx := t.Context()
			root, _ := openWithBox(t)
			parent := newSyncClient(t, root)
			child := newSyncClient(t, root)
			p := syncBox(t, "snapshot-first")
			q := syncBox(t, "snapshot-second")
			q.Requires = []string{p.ID}
			if err := parent.Stage(ctx, p); err != nil {
				t.Fatal(err)
			}
			if err := parent.Stage(ctx, q); err != nil {
				t.Fatal(err)
			}
			head, err := parent.RelayHead(ctx)
			if err != nil {
				t.Fatal(err)
			}
			page, err := parent.RelayCatalog(ctx, head, 0, 256, 4<<20)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "receipt":
				page.Items[0].Confirmed = &SyncResult{OpID: 1, AcceptedAt: time.Now().UTC()}
			case "duplicate":
				page.Items[1] = page.Items[0]
			case "dependency":
				page.Items[0], page.Items[1] = page.Items[1], page.Items[0]
			case "node":
				settings, _ := child.settings()
				page.Head.Node = settings.ID
			case "root":
				page.Head.Root.Sum = strings.Repeat("a", 64)
			case "replacement":
				page.Items[0].ReplacedBy = "unknown-replacement"
				page.Items[0].Reason = "replace"
			}
			before, _, _ := child.queue.log.Last()
			if err := child.ReceiveRelayPage(ctx, page); err == nil {
				t.Fatal("accepted invalid snapshot")
			}
			after, _, _ := child.queue.log.Last()
			if before.ID != after.ID {
				t.Fatal("invalid publication was persisted")
			}
			if _, err := child.Status(ctx); err != nil {
				t.Fatal("invalid snapshot poisoned client", err)
			}
		})
	}
}
