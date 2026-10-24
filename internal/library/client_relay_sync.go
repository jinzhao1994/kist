package library

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// RelayPeer is deliberately separate from SyncPeer: a relay cannot return a
// root commit in place of a custody acknowledgement.
type RelayPeer interface {
	originalUploadPeer
	RelayHead(context.Context) (RelayHead, error)
	RelayCatalog(context.Context, RelayHead, int, int, int) (RelayPage, error)
	RelayRootCatalog(context.Context, SyncBoundary, int64, int, int) (SyncCatalogPage, error)
	ReceiveRelay(context.Context, domain.ID, SyncProposal) (RelayOutcome, error)
}

type RelaySyncResult struct {
	Submitted    int `json:"submitted"`
	AwaitingRoot int `json:"awaiting_root"`
}

// SynchronizeRelay exchanges published root metadata and relay overlays, then
// hands local intents to that relay. It never labels custody as final sync.
func (c *Client) SynchronizeRelay(ctx context.Context, peer RelayPeer) (RelaySyncResult, error) {
	return c.synchronizeRelay(ctx, peer, false, nil)
}

func (c *Client) synchronizeRelay(ctx context.Context, peer RelayPeer, automatic bool, report func(ClientSyncProgress)) (RelaySyncResult, error) {
	var result RelaySyncResult
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return result, ErrRequestScope
	}
	release, err := lockfile.New(filepath.Join(c.root, "sync.lock")).Acquire(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	head, err := peer.RelayHead(ctx)
	if err != nil {
		return result, err
	}
	if automatic {
		entries, err := c.queue.snapshot(ctx)
		if err != nil {
			return result, err
		}
		old, _, err := c.RelayTransfer(ctx)
		if err != nil {
			return result, err
		}
		for _, e := range entries {
			if e.active() && e.Upstream == nil && e.Forwarded == nil {
				if old == nil || *old != head {
					return result, ErrSyncConflict
				}
				if _, ok := peer.(ConditionalRelayPeer); !ok {
					return result, ErrSyncConflict
				}
			}
		}
	}
	if report != nil {
		report(ClientSyncProgress{Direction: "download", Count: int64(head.Count)})
	}
	if err := c.pullRelay(ctx, peer); err != nil {
		return result, err
	}
	for {
		entry, err := c.nextRelayIntent(ctx)
		if err != nil {
			return result, err
		}
		if entry == nil {
			break
		}
		conditional, conditionalOK := peer.(ConditionalRelayPeer)
		if automatic && !conditionalOK {
			return result, ErrSyncConflict
		}
		if report != nil {
			report(ClientSyncProgress{Direction: "upload", Count: 1})
		}
		if !entry.Sending {
			if err := c.uploadIntentOriginals(ctx, peer, entry.Proposal); err != nil {
				return result, err
			}
			if err := c.recordSyncEvent(ctx, "pending_send", pendingEvent{Target: entry.Proposal.ID}); err != nil {
				return result, err
			}
		}
		settings, err := c.settings()
		if err != nil {
			return result, err
		}
		var outcome RelayOutcome
		if automatic {
			outcome, err = conditional.ReceiveRelayAt(ctx, settings.LibraryID, entry.Proposal, head)
		} else {
			outcome, err = peer.ReceiveRelay(ctx, settings.LibraryID, entry.Proposal)
		}
		if err != nil {
			var rejected *SyncRejectedError
			if errors.As(err, &rejected) {
				if persistErr := c.recordSyncEvent(ctx, "pending_reject", pendingEvent{Target: entry.Proposal.ID, Reason: rejected.Error()}); persistErr != nil {
					return result, persistErr
				}
			}
			return result, err
		}
		receipt := outcome.Custody
		if !receipt.valid() || receipt.LibraryID != settings.LibraryID || receipt.ProposalID != entry.Proposal.ID || receipt.Digest != entry.Digest {
			return result, ErrSyncConflict
		}
		if err := c.recordSyncEvent(ctx, "pending_relay_ack", pendingEvent{Target: entry.Proposal.ID, Custody: &receipt}); err != nil {
			return result, err
		}
		result.Submitted++
		if automatic {
			break
		}
	}
	if err := c.pullRelay(ctx, peer); err != nil {
		return result, err
	}
	entries, err := c.queue.snapshot(ctx)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if entry.active() {
			result.AwaitingRoot++
		}
	}
	if err := c.AdoptConfirmedOriginals(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (c *Client) pullRelay(ctx context.Context, peer RelayPeer) error {
	old, next, err := c.RelayTransfer(ctx)
	completeSnapshot := old != nil && next == old.Count
	if err != nil {
		return err
	}
	head, err := peer.RelayHead(ctx)
	if err != nil {
		return err
	}
	settings, err := c.settings()
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(c.root, "kist.toml"))
	if err != nil {
		return err
	}
	if !cfg.Client.ParentID.IsZero() && cfg.Client.ParentID != head.Node {
		return ErrSyncConflict
	}
	if head.Node == settings.ID || head.Root.LibraryID != settings.LibraryID {
		return ErrSyncConflict
	}
	if old != nil && next < old.Count {
		if head.Node != old.Node {
			return ErrSyncConflict
		}
		head = *old
	} else {
		next = 0
	}
	catalog, err := c.CatalogStatus(ctx)
	if err != nil {
		return err
	}
	if old != nil && *old == head && catalog.Complete && catalog.Confirmed == head.Root && (catalog.Transfer == nil || *catalog.Transfer == catalog.Confirmed) && completeSnapshot {
		return nil
	}
	if catalog.Confirmed.OpID > head.Root.OpID {
		return ErrSyncConflict
	}
	if catalog.Transfer != nil && *catalog.Transfer != catalog.Confirmed && *catalog.Transfer != head.Root {
		return ErrSyncConflict
	}
	if !catalog.Complete || catalog.Confirmed != head.Root {
		after := catalog.Downloaded
		for {
			page, err := peer.RelayRootCatalog(ctx, head.Root, after, 128, 4<<20)
			if err != nil {
				return err
			}
			if page.After != after || page.Boundary != head.Root {
				return ErrSyncConflict
			}
			if err := c.ReceiveCatalogPage(ctx, page); err != nil {
				return err
			}
			if page.Done {
				break
			}
			after = page.Next
		}
	}
	for {
		page, err := peer.RelayCatalog(ctx, head, next, 128, 4<<20)
		if err != nil {
			return err
		}
		if page.Head != head || page.After != next {
			return ErrSyncConflict
		}
		if err := c.ReceiveRelayPage(ctx, page); err != nil {
			return err
		}
		if page.Done {
			return nil
		}
		next = page.Next
	}
}

func (s *pendingState) nextRelay() (*pendingEntry, error) {
	var next *pendingEntry
	for i := range s.entries {
		e := &s.entries[i]
		if e.active() && e.Upstream == nil && e.Forwarded == nil && (next == nil || comparePending(*e, *next) < 0) {
			next = e
		}
	}
	if next == nil {
		return nil, nil
	}
	if next.Conflict != "" {
		return next, ErrSyncConflict
	}
	refs := append([]string{}, next.Proposal.Requires...)
	for _, ref := range []*SyncReference{next.Proposal.Undo, next.Proposal.Group} {
		if ref != nil && ref.LocalID != "" {
			refs = append(refs, ref.LocalID)
		}
	}
	for _, ref := range refs {
		i, found := s.byID[ref]
		if !found {
			return next, ErrSyncDependency
		}
		e := s.entries[i]
		if e.Discarded || e.ReplacedBy != "" || (e.Receipt == nil && e.Upstream == nil && e.Forwarded == nil) {
			return next, ErrSyncDependency
		}
	}
	return next, nil
}

func (c *Client) nextRelayIntent(ctx context.Context) (*pendingEntry, error) {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	view, err := c.rebuildLocked(ctx)
	if err != nil {
		return nil, err
	}
	state, err := c.queue.read(ctx)
	if err != nil {
		return nil, err
	}
	entry, err := state.nextRelay()
	if err != nil {
		return entry, err
	}
	if entry != nil && !entry.Sending && view.Conflict != nil {
		return entry, ErrSyncConflict
	}
	return entry, nil
}

// ConditionalRelayPeer prevents automatic merges with an independently edited view.
type ConditionalRelayPeer interface {
	ReceiveRelayAt(context.Context, domain.ID, SyncProposal, RelayHead) (RelayOutcome, error)
}

func (c *Client) synchronizeRelayMode(ctx context.Context, peer RelayPeer, automatic bool, report func(ClientSyncProgress)) (ClientSyncResult, error) {
	result, err := c.synchronizeRelay(ctx, peer, automatic, report)
	out := ClientSyncResult{AwaitingRoot: result.AwaitingRoot, Remaining: result.AwaitingRoot, Complete: err == nil}
	if automatic && errors.Is(err, ErrSyncConflict) {
		out.ManualRequired = true
		return out, nil
	}
	return out, err
}
