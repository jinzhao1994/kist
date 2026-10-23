package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/oplog"
)

type SyncCapabilities struct {
	ConditionalSubmit bool      `json:"conditional_submit"`
	DerivedPreviews   bool      `json:"derived_previews"`
	DeletionSnapshots bool      `json:"deletion_snapshots"`
	Timezone          string    `json:"timezone"`
	Version           int       `json:"version"`
	LibraryID         domain.ID `json:"library_id"`
	JournalVersion    int       `json:"journal_version"`
	Actions           []string  `json:"actions"`
	UndoReferences    bool      `json:"undo_references"`
}

func ValidateSyncCapabilities(caps SyncCapabilities) error {
	if caps.Version != 1 || caps.JournalVersion != journal.Version || !caps.UndoReferences {
		return ErrSyncVersion
	}
	if caps.Timezone == "" || caps.Timezone == "Local" {
		return ErrSyncVersion
	}
	if _, err := time.LoadLocation(caps.Timezone); err != nil {
		return fmt.Errorf("%w: invalid NAS timezone", ErrSyncVersion)
	}
	if caps.LibraryID.IsZero() {
		return ErrInvalidRequest
	}
	return nil
}

// SyncPeer is the authenticated transport boundary. Transfers can seek the
// durable local original to resume; the synchronization algorithm owns intent
// ordering, acknowledgement persistence and the final catalog reconciliation.
type SyncPeer interface {
	Capabilities(context.Context) (SyncCapabilities, error)
	Head(context.Context) (SyncBoundary, error)
	Catalog(context.Context, SyncBoundary, int64, int, int) (SyncCatalogPage, error)
	HasOriginal(context.Context, string) (bool, error)
	UploadOriginal(context.Context, string, int64, io.ReadSeeker) error
	Submit(context.Context, SyncProposal) (SyncResult, error)
}

// SyncRejectedError means the server explicitly guarantees it did not commit
// this intent. A timeout, EOF or unspecified HTTP failure is never a rejection.
type SyncRejectedError struct{ Cause error }

func (e *SyncRejectedError) Error() string { return e.Cause.Error() }
func (e *SyncRejectedError) Unwrap() error { return e.Cause }

type ClientSyncResult struct {
	ManualRequired bool   `json:"manual_required,omitempty"`
	Accepted       int    `json:"accepted"`
	Remaining      int    `json:"remaining"`
	Complete       bool   `json:"complete"`
	BlockedID      string `json:"blocked_id,omitempty"`
}

// Synchronize permits NAS writes and new local edits during network I/O.
// A separate cross-process lock prevents overlapping manual synchronization.
func (c *Client) Synchronize(ctx context.Context, peer SyncPeer) (ClientSyncResult, error) {
	return c.synchronize(ctx, peer, false)
}

// ConditionalSyncPeer supports an atomic remote head precondition.
type ConditionalSyncPeer interface {
	SyncPeer
	SubmitAt(context.Context, SyncProposal, SyncBoundary) (SyncResult, error)
}

// SynchronizeAutomatic refuses to merge independently changed histories.
func (c *Client) SynchronizeAutomatic(ctx context.Context, peer SyncPeer) (ClientSyncResult, error) {
	result, err := c.synchronize(ctx, peer, true)
	if errors.Is(err, ErrSyncConflict) {
		result.ManualRequired = true
		return result, nil
	}
	return result, err
}

func (c *Client) synchronize(ctx context.Context, peer SyncPeer, automatic bool) (ClientSyncResult, error) {
	var result ClientSyncResult
	release, err := lockfile.New(filepath.Join(c.root, "sync.lock")).Acquire(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	settings, err := c.settings()
	if err != nil {
		return result, err
	}
	caps, err := peer.Capabilities(ctx)
	if err != nil {
		return result, err
	}
	if caps.LibraryID != settings.LibraryID {
		return result, ErrSyncConflict
	}
	if err := ValidateSyncCapabilities(caps); err != nil {
		return result, err
	}
	if err := c.AdoptSyncSettings(ctx, caps); err != nil {
		return result, err
	}
	var expected SyncBoundary
	skipPull := false
	if automatic {
		local, err := c.Status(ctx)
		if err != nil {
			return result, err
		}
		expected, err = peer.Head(ctx)
		if err != nil {
			return result, err
		}
		if len(local.Pending) == 0 && local.Catalog.Complete && expected == local.Catalog.Confirmed {
			result.Complete = true
			return result, nil
		}
		for _, entry := range local.Pending {
			if entry.Conflict != "" {
				return result, ErrSyncConflict
			}
		}
		if len(local.Pending) > 0 {
			allowed, err := c.automaticBoundaryAllowed(ctx, expected)
			if err != nil {
				return result, err
			}
			if !allowed || local.BlockedID != "" {
				return result, ErrSyncConflict
			}
			if _, ok := peer.(ConditionalSyncPeer); !ok || !caps.ConditionalSubmit {
				return result, ErrSyncConflict
			}
			skipPull = expected == local.Catalog.Confirmed
		}
	}
	if !skipPull {
		if err := c.pullCatalogMode(ctx, peer, automatic); err != nil {
			return result, err
		}
		if automatic {
			status, err := c.CatalogStatus(ctx)
			if err != nil {
				return result, err
			}
			expected = status.Confirmed
		}
	}
	for {
		entry, conflict, err := c.nextSyncIntent(ctx)
		if err != nil {
			if entry != nil {
				result.BlockedID = entry.Proposal.ID
			}
			return result, err
		}
		if entry == nil {
			break
		}
		if automatic {
			if _, ok := peer.(ConditionalSyncPeer); !ok || !caps.ConditionalSubmit {
				return result, ErrSyncConflict
			}
			head, err := peer.Head(ctx)
			if err != nil {
				return result, err
			}
			if head != expected {
				return result, ErrSyncConflict
			}
		}
		p := entry.Proposal
		result.BlockedID = p.ID
		if (p.Version == 2 && !caps.DeletionSnapshots) || (p.Undo == nil && !slices.Contains(caps.Actions, p.Action)) {
			return result, ErrSyncVersion
		}
		if !entry.Sending {
			if conflict != nil {
				return result, fmt.Errorf("%w: %w", ErrSyncConflict, conflict)
			}
			if err := c.uploadIntentOriginals(ctx, peer, p); err != nil {
				return result, err
			}
			if err := c.recordSyncEvent(ctx, "pending_send", pendingEvent{Target: p.ID}); err != nil {
				return result, err
			}
		}
		// Retry an uncertain submission verbatim. NAS checks ID AND digest
		// before present-day state; an ID-only status lookup cannot prove that.
		var receipt SyncResult
		if automatic {
			receipt, err = peer.(ConditionalSyncPeer).SubmitAt(ctx, p, expected)
		} else {
			receipt, err = peer.Submit(ctx, p)
		}
		if err != nil {
			var rejected *SyncRejectedError
			if errors.As(err, &rejected) {
				if persistErr := c.recordSyncEvent(ctx, "pending_reject", pendingEvent{Target: p.ID, Reason: rejected.Error()}); persistErr != nil {
					return result, persistErr
				}
				if automatic {
					return result, ErrSyncConflict
				}
				if refreshErr := c.pullCatalog(ctx, peer); refreshErr != nil {
					return result, errors.Join(err, refreshErr)
				}
			}
			return result, err
		}
		if err := c.recordSyncEvent(ctx, "pending_ack", pendingEvent{Target: p.ID, Receipt: &receipt}); err != nil {
			return result, err
		}
		result.Accepted++
		result.BlockedID = ""
		if automatic {
			expected, err = peer.Head(ctx)
			if err != nil {
				return result, err
			}
			if expected.OpID != receipt.OpID {
				return result, ErrSyncConflict
			}
		}
	}
	if err := c.pullCatalogMode(ctx, peer, automatic); err != nil {
		return result, err
	}
	entries, err := c.queue.snapshot(ctx)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if entry.active() {
			result.Remaining++
		}
	}
	result.Complete = result.Remaining == 0
	if err := c.AdoptConfirmedOriginals(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (c *Client) pullCatalog(ctx context.Context, peer SyncPeer) error {
	return c.pullCatalogMode(ctx, peer, false)
}
func (c *Client) pullCatalogMode(ctx context.Context, peer SyncPeer, automatic bool) error {
	status, err := c.CatalogStatus(ctx)
	if err != nil {
		return err
	}
	var head SyncBoundary
	if status.Transfer != nil && *status.Transfer != status.Confirmed {
		head = *status.Transfer
	} else {
		head, err = peer.Head(ctx)
		if err != nil {
			return err
		}
	}
	after := status.Downloaded
	for {
		page, err := peer.Catalog(ctx, head, after, 128, 4<<20)
		if err != nil {
			return err
		}
		if page.After != after || page.Boundary != head {
			return ErrSyncConflict
		}
		if err := c.receiveCatalogPage(ctx, page, automatic); err != nil {
			return err
		}
		if page.Done {
			return nil
		}
		after = page.Next
	}
}

func (c *Client) nextSyncIntent(ctx context.Context) (*pendingEntry, error, error) {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer release()
	view, err := c.rebuildLocked(ctx)
	if err != nil {
		return nil, nil, err
	}
	state, err := c.queue.read(ctx)
	if err != nil {
		return nil, nil, err
	}
	next, err := state.next()
	if err != nil || next == nil {
		return next, nil, err
	}
	if view.BlockedID == next.Proposal.ID {
		return next, view.Conflict, nil
	}
	return next, nil, nil
}

func (c *Client) recordSyncEvent(ctx context.Context, action string, event pendingEvent) error {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := c.queue.appendLocked(ctx, action, event); err != nil {
		return err
	}
	_, err = c.rebuildLocked(ctx)
	return err
}

// Resolve keeps the immutable original and appends a discard or replacement.
// Uncertain sends cannot be resolved until a sync obtains a definitive result.
func (c *Client) Resolve(ctx context.Context, id string, replacement *SyncProposal, reason string) error {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	action := "pending_discard"
	if replacement != nil {
		action = "pending_resolve"
	}
	event := pendingEvent{Version: 1, Target: id, Proposal: replacement, Reason: reason}
	state, err := c.queue.read(ctx)
	if err != nil {
		return err
	}
	// A repeated decision remains idempotent even after its replacement syncs.
	if i, found := state.byID[id]; found && !state.entries[i].active() {
		if err := c.queue.appendLocked(ctx, action, event); err != nil {
			return err
		}
		_, err = c.rebuildLocked(ctx)
		return err
	}
	if err := state.apply(action, event, state.last+1); err != nil {
		return err
	}
	view, err := c.rebuildLocked(ctx)
	if err != nil {
		return err
	}
	if replacement != nil {
		if view.BlockedID != id {
			return ErrSyncConflict
		}
		if err := c.validateIntentLocked(ctx, *replacement, view, state); err != nil {
			return err
		}
	}
	if err := c.queue.appendLocked(ctx, action, event); err != nil {
		return err
	}
	_, err = c.rebuildLocked(ctx)
	return err
}

func (c *Client) uploadIntentOriginals(ctx context.Context, peer SyncPeer, p SyncProposal) error {
	if p.Undo != nil {
		// The target must already be accepted before its inverse is sent.
		// NAS retains historical originals, so undo requires no new content.
		return nil
	}
	h, err := oplog.Lookup(oplog.Action(p.Action))
	if err != nil {
		return err
	}
	targets, err := h.Targets(p.Payload)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range targets.Blobs {
		if seen[name] {
			continue
		}
		seen[name] = true
		found, err := peer.HasOriginal(ctx, name)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		key, err := blob.ParseKey(name)
		if err != nil {
			return err
		}
		r, err := c.blobs.Get(ctx, key)
		if err != nil {
			return err
		}
		size, err := r.Seek(0, io.SeekEnd)
		if err == nil {
			_, err = r.Seek(0, io.SeekStart)
		}
		if err == nil {
			err = peer.UploadOriginal(ctx, name, size, r)
		}
		closeErr := r.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
