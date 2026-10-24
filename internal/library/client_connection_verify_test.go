package library

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
)

type connectionVerifyPeer struct {
	*testSyncPeer
	changed        int64
	changedReceipt int64
	fail           bool
	headOverride   *SyncBoundary
	beforeCatalog  func()
}

func (p *connectionVerifyPeer) Head(ctx context.Context) (SyncBoundary, error) {
	if p.headOverride != nil {
		return *p.headOverride, nil
	}
	return p.testSyncPeer.Head(ctx)
}
func (p *connectionVerifyPeer) Catalog(ctx context.Context, head SyncBoundary, after int64, limit, budget int) (SyncCatalogPage, error) {
	if p.beforeCatalog != nil {
		fn := p.beforeCatalog
		p.beforeCatalog = nil
		fn()
	}
	if p.fail {
		return SyncCatalogPage{}, io.ErrUnexpectedEOF
	}
	page, err := p.testSyncPeer.Catalog(ctx, head, after, limit, budget)
	if err != nil {
		return page, err
	}
	for i, raw := range page.Lines {
		line, err := journal.UnmarshalLine(raw)
		if err != nil {
			return page, err
		}
		if line.ID == p.changedReceipt && line.Sync != nil {
			line.Sync.ID = "different-operation"
			page.Lines[i], err = journal.MarshalLine(line)
			if err != nil {
				return page, err
			}
		}
		if line.ID == p.changed {
			line.Origin = "different-origin"
			page.Lines[i], err = journal.MarshalLine(line)
			if err != nil {
				return page, err
			}
		}
	}
	return page, nil
}

func TestVerifyConnectionPreservesLocalStateOnMismatch(t *testing.T) {
	for _, scenario := range []string{"same", "ahead", "behind", "different-library", "changed-middle", "interrupted", "local-history-race", "settings-race"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := t.Context()
			nas := openTestLibrary(t)
			for i := 0; i < 3; i++ {
				if _, err := nas.AcceptSync(ctx, syncBox(t, fmt.Sprintf("remote-change-%d", i))); err != nil {
					t.Fatal(err)
				}
			}
			c := newSyncClient(t, nas)
			head, _ := nas.SyncHead(ctx)
			if err := c.SetConnection(ctx, head.LibraryID, ClientConnection{Server: "https://old.example"}, "old-token"); err != nil {
				t.Fatal(err)
			}
			proposal := syncBox(t, "local-change")
			if err := c.Stage(ctx, proposal); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(c.root, "kist.toml")
			before, _ := os.ReadFile(path)
			peer := &connectionVerifyPeer{testSyncPeer: &testSyncPeer{nas: nas}}
			switch scenario {
			case "ahead":
				if _, err := nas.AcceptSync(ctx, syncBox(t, "new-remote")); err != nil {
					t.Fatal(err)
				}
			case "behind":
				changed := head
				changed.OpID--
				peer.headOverride = &changed
			case "different-library":
				changed := head
				changed.LibraryID = domain.NewID()
				peer.headOverride = &changed
			case "changed-middle":
				peer.changed = 1
			case "interrupted":
				peer.fail = true
			case "local-history-race":
				peer.beforeCatalog = func() {
					if _, err := nas.AcceptSync(ctx, syncBox(t, "racing-remote")); err != nil {
						t.Fatal(err)
					}
					newer, _ := nas.SyncHead(ctx)
					page, err := nas.SyncCatalog(ctx, newer, head.OpID, 256, 4<<20)
					if err != nil {
						t.Fatal(err)
					}
					if err := c.ReceiveCatalogPage(ctx, page); err != nil {
						t.Fatal(err)
					}
				}
			case "settings-race":
				peer.beforeCatalog = func() {
					if err := c.SetConnection(ctx, head.LibraryID, ClientConnection{Server: "https://other.example"}, "other-token"); err != nil {
						t.Fatal(err)
					}
				}
			}
			err := c.VerifyAndSetConnection(ctx, peer, ClientConnection{Server: "https://new.example"}, "new-token", config.Revision(before))
			success := scenario == "same" || scenario == "ahead"
			if success && err != nil {
				t.Fatal(err)
			}
			if !success && err == nil {
				t.Fatal("accepted incompatible connection")
			}
			if scenario == "changed-middle" && !errors.Is(err, ErrSyncConflict) {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(path)
			if !success && scenario != "settings-race" && !bytes.Equal(before, after) {
				t.Fatal("failed verification changed connection")
			}
			connection, token, err := c.Connection(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if success && (connection.Server != "https://new.example" || token != "new-token") {
				t.Fatal("connection not saved")
			}
			if scenario == "settings-race" && connection.Server != "https://other.example" {
				t.Fatal("overwrote concurrent configuration")
			}
			entries, err := c.queue.snapshot(ctx)
			if err != nil || len(entries) != 1 || entries[0].Proposal.ID != proposal.ID || !entries[0].active() {
				t.Fatal("pending operation changed", err)
			}
		})
	}
}

func TestVerifyConnectionChecksPartialHistoryAndReceipts(t *testing.T) {
	for _, mode := range []string{"partial", "receipt"} {
		t.Run(mode, func(t *testing.T) {
			ctx := t.Context()
			nas := openTestLibrary(t)
			c := newSyncClient(t, nas)
			head, _ := nas.SyncHead(ctx)
			if err := c.SetConnection(ctx, head.LibraryID, ClientConnection{Server: "https://old.example"}, "old-token"); err != nil {
				t.Fatal(err)
			}
			p := syncBox(t, "acknowledged-operation")
			if err := c.Stage(ctx, p); err != nil {
				t.Fatal(err)
			}
			receipt, err := nas.AcceptSync(ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.recordSyncEvent(ctx, "pending_send", pendingEvent{Target: p.ID}); err != nil {
				t.Fatal(err)
			}
			if err := c.recordSyncEvent(ctx, "pending_ack", pendingEvent{Target: p.ID, Receipt: &receipt}); err != nil {
				t.Fatal(err)
			}
			if mode == "partial" {
				if _, err := nas.AcceptSync(ctx, syncBox(t, "later-operation")); err != nil {
					t.Fatal(err)
				}
				head, _ = nas.SyncHead(ctx)
				page, err := nas.SyncCatalog(ctx, head, 0, 1, 4<<20)
				if err != nil || page.Done {
					t.Fatal(page, err)
				}
				if err := c.ReceiveCatalogPage(ctx, page); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(c.root, "kist.toml")
			before, _ := os.ReadFile(path)
			peer := &connectionVerifyPeer{testSyncPeer: &testSyncPeer{nas: nas}}
			// An older same-ID endpoint may lack an accepted operation or a transfer target.
			older := SyncBoundary{LibraryID: head.LibraryID}
			peer.headOverride = &older
			if err := c.VerifyAndSetConnection(ctx, peer, ClientConnection{Server: "https://new.example"}, "new-token", ""); !errors.Is(err, ErrSyncConflict) {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("rejection changed configuration")
			}
			peer.headOverride = nil
			if mode == "receipt" {
				peer.changedReceipt = receipt.OpID
				if err := c.VerifyAndSetConnection(ctx, peer, ClientConnection{Server: "https://new.example"}, "new-token", ""); !errors.Is(err, ErrSyncConflict) {
					t.Fatal(err)
				}
				peer.changedReceipt = 0
			}
			if err := c.VerifyAndSetConnection(ctx, peer, ClientConnection{Server: "https://new.example"}, "new-token", ""); err != nil {
				t.Fatal(err)
			}
		})
	}
}
