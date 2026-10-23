package library

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
)

type testSyncPeer struct {
	nas          *Library
	beforeSubmit func()
	loseReply    bool
	failUpload   bool
}

func (p *testSyncPeer) Capabilities(ctx context.Context) (SyncCapabilities, error) {
	head, err := p.nas.SyncHead(ctx)
	return SyncCapabilities{DeletionSnapshots: true, Timezone: p.nas.Zone().String(), Version: 1, LibraryID: head.LibraryID, JournalVersion: journal.Version, Actions: SyncActions(), UndoReferences: true}, err
}
func (p *testSyncPeer) Head(ctx context.Context) (SyncBoundary, error) { return p.nas.SyncHead(ctx) }
func (p *testSyncPeer) Catalog(ctx context.Context, head SyncBoundary, after int64, limit, bytes int) (SyncCatalogPage, error) {
	return p.nas.SyncCatalog(ctx, head, after, limit, bytes)
}
func (p *testSyncPeer) HasOriginal(ctx context.Context, name string) (bool, error) {
	key, err := blob.ParseKey(name)
	if err != nil {
		return false, err
	}
	_, err = p.nas.blobs.Stat(ctx, key)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
func (p *testSyncPeer) UploadOriginal(ctx context.Context, name string, size int64, r io.ReadSeeker) error {
	if p.failUpload {
		p.failUpload = false
		_, _ = io.CopyN(io.Discard, r, 3)
		return io.ErrUnexpectedEOF
	}
	result, err := p.nas.blobs.Put(ctx, r)
	if err == nil && (string(result.Key) != name || result.Size != size) {
		return ErrSyncConflict
	}
	return err
}
func (p *testSyncPeer) Submit(ctx context.Context, proposal SyncProposal) (SyncResult, error) {
	if p.beforeSubmit != nil {
		p.beforeSubmit()
		p.beforeSubmit = nil
	}
	r, err := p.nas.AcceptSync(ctx, proposal)
	if err != nil {
		return r, err
	}
	if p.loseReply {
		p.loseReply = false
		return SyncResult{}, io.ErrUnexpectedEOF
	}
	return r, nil
}

func newSyncClient(t *testing.T, nas *Library) *Client {
	t.Helper()
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	c, err := CreateClient(t.Context(), t.TempDir(), ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	if _, err := c.Synchronize(t.Context(), &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClientSynchronizeConvergesAfterUploadInterruption(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	create := syncBox(t, "sync-travel-box")
	if err := c.Stage(t.Context(), create); err != nil {
		t.Fatal(err)
	}
	box, _ := oplog.Payload[oplog.CreateBoxPayload](create.Payload)
	content := "travel original"
	stored, err := c.PutOriginal(t.Context(), strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	f := oplog.ImportedFile{FileID: domain.NewFileID(), FolderID: box.RootFolderID, Name: "photo.jpg", BlobKey: string(stored.Key), Sha256: stored.Key.Hex(), Size: stored.Size}
	raw, _ := oplog.Marshal(oplog.ImportFilesPayload{BoxID: box.BoxID, Files: []oplog.ImportedFile{f}})
	imported := SyncProposal{Version: 1, ID: "sync-travel-photo", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionImportFiles), Origin: "user", Payload: raw, Requires: []string{create.ID}}
	if err := c.Stage(t.Context(), imported); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.CreateBox(t.Context(), "independent NAS change", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	peer := &testSyncPeer{nas: nas, failUpload: true}
	result, err := c.Synchronize(t.Context(), peer)
	if !errors.Is(err, io.ErrUnexpectedEOF) || result.Accepted != 1 || result.BlockedID != imported.ID {
		t.Fatalf("interrupted=%+v err=%v", result, err)
	}
	entries, _ := c.queue.snapshot(t.Context())
	if entries[1].Sending || entries[1].Receipt != nil {
		t.Fatal("upload failure marked intent as submitted")
	}
	result, err = c.Synchronize(t.Context(), peer)
	if err != nil || !result.Complete || result.Accepted != 1 {
		t.Fatalf("retry=%+v err=%v", result, err)
	}
	if snapshotOf(t, nas) != snapshotOf(t, &Library{db: c.db}) {
		t.Fatal("client metadata did not converge")
	}
	count, _ := nas.OpCount(t.Context())
	result, err = c.Synchronize(t.Context(), peer)
	if err != nil || !result.Complete || result.Accepted != 0 {
		t.Fatalf("idempotent sync=%+v err=%v", result, err)
	}
	if after, _ := nas.OpCount(t.Context()); after != count {
		t.Fatal("repeat synchronization appended NAS operations")
	}
}

func TestClientSynchronizeRecoversLostCommitResponse(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	p := syncBox(t, "sync-lost-response")
	if err := c.Stage(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	peer := &testSyncPeer{nas: nas, loseReply: true}
	if _, err := c.Synchronize(t.Context(), peer); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("lost reply=%v", err)
	}
	if err := c.Resolve(t.Context(), p.ID, nil, "discard"); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("discarded uncertain commit: %v", err)
	}
	root := c.root
	c.Close()
	reopened, err := OpenClient(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	result, err := reopened.Synchronize(t.Context(), peer)
	if err != nil || !result.Complete || result.Accepted != 1 {
		t.Fatalf("recovered=%+v err=%v", result, err)
	}
	if count, _ := nas.OpCount(t.Context()); count != 1 {
		t.Fatalf("duplicate NAS commits: %d", count)
	}
	if snapshotOf(t, nas) != snapshotOf(t, &Library{db: reopened.db}) {
		t.Fatal("recovered client differs from NAS")
	}
}

func TestClientSynchronizeConcurrentConflictAndResolution(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	b, _, _ := c.Box(t.Context(), box)
	raw, _ := oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box, Name: &oplog.TextChange{Before: b.Name, After: "local name"}})
	p := SyncProposal{Version: 1, ID: "sync-concurrent-rename", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionUpdateBox), Origin: "user", Payload: raw}
	if err := c.Stage(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	later := syncBox(t, "sync-after-conflict")
	if err := c.Stage(t.Context(), later); err != nil {
		t.Fatal(err)
	}
	peer := &testSyncPeer{nas: nas, beforeSubmit: func() {
		if _, err := nas.UpdateBox(t.Context(), box, oplog.BoxFields{Name: "NAS name"}); err != nil {
			t.Fatal(err)
		}
	}}
	result, err := c.Synchronize(t.Context(), peer)
	var rejected *SyncRejectedError
	if !errors.As(err, &rejected) || result.BlockedID != p.ID {
		t.Fatalf("conflict=%+v err=%v", result, err)
	}
	b, _, _ = c.Box(t.Context(), box)
	if b.Name != "NAS name" {
		t.Fatal("conflict view did not refresh NAS changes")
	}
	invalid := p
	invalid.ID = "sync-invalid-resolution"
	missing := oplog.ImportedFile{FileID: domain.NewFileID(), FolderID: b.RootFolderID, Name: "missing.txt", BlobKey: "sha256:" + strings.Repeat("a", 64), Sha256: strings.Repeat("a", 64), Size: 1}
	invalid.Action = string(oplog.ActionImportFiles)
	invalid.Payload, _ = oplog.Marshal(oplog.ImportFilesPayload{BoxID: box, Files: []oplog.ImportedFile{missing}})
	if err := c.Resolve(t.Context(), result.BlockedID, &invalid, "invalid content replacement"); err == nil {
		t.Fatal("resolution bypassed original durability checks")
	}
	entries, _ := c.queue.snapshot(t.Context())
	if entries[0].ReplacedBy != "" || entries[0].Conflict == "" || len(entries) != 2 {
		t.Fatal("failed resolution changed original intent")
	}
	p.ID = "sync-resolved-rename"
	p.Payload, _ = oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box, Name: &oplog.TextChange{Before: "NAS name", After: "chosen name"}})
	if err := c.Resolve(t.Context(), result.BlockedID, &p, "choose revised local name"); err != nil {
		t.Fatal(err)
	}
	result, err = c.Synchronize(t.Context(), peer)
	if err != nil || !result.Complete || result.Accepted != 2 {
		t.Fatalf("resolved=%+v err=%v", result, err)
	}
	if snapshotOf(t, nas) != snapshotOf(t, &Library{db: c.db}) {
		t.Fatal("resolved client did not converge")
	}
}

type automaticPeer struct {
	*testSyncPeer
	beforeCatalog func()
}

func (p *automaticPeer) Capabilities(ctx context.Context) (SyncCapabilities, error) {
	caps, err := p.testSyncPeer.Capabilities(ctx)
	caps.ConditionalSubmit = true
	return caps, err
}
func (p *automaticPeer) Catalog(ctx context.Context, head SyncBoundary, after int64, limit, bytes int) (SyncCatalogPage, error) {
	if p.beforeCatalog != nil {
		fn := p.beforeCatalog
		p.beforeCatalog = nil
		fn()
	}
	return p.testSyncPeer.Catalog(ctx, head, after, limit, bytes)
}
func (p *automaticPeer) SubmitAt(ctx context.Context, proposal SyncProposal, expected SyncBoundary) (SyncResult, error) {
	if p.beforeSubmit != nil {
		fn := p.beforeSubmit
		p.beforeSubmit = nil
		fn()
	}
	result, err := p.nas.AcceptSyncAt(ctx, proposal, expected)
	if err == nil && p.loseReply {
		p.loseReply = false
		return SyncResult{}, io.ErrUnexpectedEOF
	}
	return result, err
}
func TestAutomaticSyncDirectionsAndRaces(t *testing.T) {
	for _, scenario := range []string{"download", "upload", "both", "remote-race", "local-race", "lost-reply"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := t.Context()
			nas := openTestLibrary(t)
			c := newSyncClient(t, nas)
			peer := &automaticPeer{testSyncPeer: &testSyncPeer{nas: nas}}
			before, _ := c.CatalogStatus(ctx)
			remote := func() {
				if _, err := nas.AcceptSync(ctx, syncBox(t, "remote-change")); err != nil {
					t.Fatal(err)
				}
			}
			local := func() {
				if err := c.Stage(ctx, syncBox(t, "local-change")); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "download":
				remote()
			case "upload":
				local()
				if err := c.Stage(ctx, syncBox(t, "second-local")); err != nil {
					t.Fatal(err)
				}
			case "both":
				remote()
				local()
			case "remote-race":
				local()
				peer.beforeSubmit = remote
			case "local-race":
				remote()
				peer.beforeCatalog = local
			case "lost-reply":
				local()
				peer.loseReply = true
			}
			var progress []ClientSyncProgress
			result, err := c.SynchronizeWithProgress(ctx, peer, true, func(p ClientSyncProgress) { progress = append(progress, p) })
			if len(progress) == 0 || progress[len(progress)-1] != (ClientSyncProgress{}) {
				t.Fatal("progress was not cleared", progress)
			}
			if scenario == "download" || scenario == "upload" {
				expected := ClientSyncProgress{Direction: scenario, Count: 1}
				if scenario == "upload" {
					expected.Count = 2
				}
				found := false
				for _, p := range progress {
					if p == expected {
						found = true
					}
				}
				if !found {
					t.Fatal("missing transfer progress", expected, progress)
				}
			}
			if scenario == "lost-reply" {
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatal(err)
				}
				status, _ := c.Status(ctx)
				if len(status.Pending) != 1 || !status.Pending[0].AwaitingReceipt {
					t.Fatal(status)
				}
				if _, err := c.Synchronize(ctx, peer); err != nil {
					t.Fatal(err)
				}
				head, _ := nas.SyncHead(ctx)
				if head.OpID != before.Confirmed.OpID+1 {
					t.Fatal("duplicate submission", head)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			status, err := c.Status(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "download" || scenario == "upload" {
				head, _ := nas.SyncHead(ctx)
				if !result.Complete || result.ManualRequired || len(status.Pending) != 0 || status.Catalog.Confirmed != head {
					t.Fatal(result, status)
				}
			} else {
				if !result.ManualRequired || len(status.Pending) != 1 || status.Catalog.Confirmed != before.Confirmed {
					t.Fatal("automatic merge occurred", result, status)
				}
				head, _ := nas.SyncHead(ctx)
				if head.OpID != before.Confirmed.OpID+1 {
					t.Fatal("unexpected remote write", head)
				}
				if _, err := c.Synchronize(ctx, peer); err != nil {
					if scenario != "remote-race" || !errors.Is(err, ErrSyncConflict) {
						t.Fatal("manual merge failed", err)
					}
					replacement := syncBox(t, "confirmed-local")
					if err := c.Resolve(ctx, status.Pending[0].ID, &replacement, "confirmed after remote change"); err != nil {
						t.Fatal(err)
					}
					if _, err := c.Synchronize(ctx, peer); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestConditionalSyncReceiptPrecedesHeadCheck(t *testing.T) {
	nas := openTestLibrary(t)
	ctx := t.Context()
	head, _ := nas.SyncHead(ctx)
	p := syncBox(t, "conditional-retry")
	first, err := nas.AcceptSyncAt(ctx, p, head)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nas.AcceptSync(ctx, syncBox(t, "later-remote")); err != nil {
		t.Fatal(err)
	}
	repeated, err := nas.AcceptSyncAt(ctx, p, head)
	if err != nil || repeated != first {
		t.Fatal(repeated, err)
	}
	if _, err := nas.AcceptSyncAt(ctx, syncBox(t, "stale-new"), head); !errors.Is(err, ErrSyncConflict) {
		t.Fatal(err)
	}
}

func TestAutomaticSyncReconcilesOwnReceiptsWithNewLocalEdits(t *testing.T) {
	ctx := t.Context()
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	if err := c.Stage(ctx, syncBox(t, "first-online-edit")); err != nil {
		t.Fatal(err)
	}
	peer := &automaticPeer{testSyncPeer: &testSyncPeer{nas: nas}, beforeCatalog: func() {
		if err := c.Stage(ctx, syncBox(t, "next-online-edit")); err != nil {
			t.Fatal(err)
		}
	}}
	first, err := c.SynchronizeAutomatic(ctx, peer)
	if err != nil || first.ManualRequired || first.Remaining != 1 {
		t.Fatal(first, err)
	}
	second, err := c.SynchronizeAutomatic(ctx, peer)
	if err != nil || second.ManualRequired || !second.Complete {
		t.Fatal(second, err)
	}
}
