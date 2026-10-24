package library

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
)

// RelayReceipt proves durable custody at one client, never a root commit.
// Its separate type deliberately has no authoritative operation number.
type RelayReceipt struct {
	LibraryID  domain.ID `json:"library_id"`
	RelayID    domain.ID `json:"relay_id"`
	ProposalID string    `json:"proposal_id"`
	Digest     string    `json:"digest"`
	ReceivedAt time.Time `json:"received_at"`
}

func (r RelayReceipt) valid() bool {
	return !r.LibraryID.IsZero() && !r.RelayID.IsZero() &&
		(journal.Request{ID: r.ProposalID, Digest: r.Digest}).Valid() &&
		!r.ReceivedAt.IsZero() && time.Unix(0, r.ReceivedAt.UnixNano()).Equal(r.ReceivedAt)
}

// RelayOutcome separates transport custody from the eventual root result.
// Conflict and replacement are explicit decisions, not rewritten proposals.
type RelayOutcome struct {
	Custody     RelayReceipt  `json:"custody"`
	Confirmed   *SyncResult   `json:"confirmed,omitempty"`
	Sending     bool          `json:"sending"`
	Conflict    string        `json:"conflict,omitempty"`
	Discarded   bool          `json:"discarded"`
	Replacement *SyncProposal `json:"replacement,omitempty"`
	Reason      string        `json:"reason,omitempty"`
}

// ReceiveRelay stages an immutable downstream intent in the ordinary durable
// pending journal. Validation and custody acknowledgement share the edit lock.
// A failure after journal append is uncertain; callers must retry verbatim.
// Transport authentication, lineage and overlay exchange are separate concerns.
func (c *Client) ReceiveRelay(ctx context.Context, libraryID domain.ID, p SyncProposal) (RelayOutcome, error) {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return RelayOutcome{}, ErrRequestScope
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return RelayOutcome{}, err
	}
	defer release()
	return c.receiveRelayLocked(ctx, libraryID, p)
}

func (c *Client) receiveRelayLocked(ctx context.Context, libraryID domain.ID, p SyncProposal) (RelayOutcome, error) {
	settings, err := c.settings()
	if err != nil {
		return RelayOutcome{}, err
	}
	if libraryID != settings.LibraryID {
		return RelayOutcome{}, &SyncRejectedError{Cause: ErrSyncConflict}
	}
	digest, err := p.digest()
	if err != nil {
		return RelayOutcome{}, err
	}
	// This checks new content against local-blobs, never the evictable cache.
	// Existing root-owned content may remain uncached, as for local operations.
	if err := c.stageLocked(ctx, p, nil); err != nil {
		return RelayOutcome{}, err
	}
	state, err := c.queue.read(ctx)
	if err != nil {
		return RelayOutcome{}, err
	}
	entry := state.entries[state.byID[p.ID]]
	if entry.Custody == nil {
		receipt := RelayReceipt{LibraryID: libraryID, RelayID: settings.ID, ProposalID: p.ID, Digest: digest, ReceivedAt: time.Now().UTC()}
		if err := c.queue.appendLocked(ctx, "pending_custody", pendingEvent{Target: p.ID, Custody: &receipt}); err != nil {
			return RelayOutcome{}, err
		}
	}
	return c.relayOutcomeLocked(ctx, p.ID, digest)
}

// RelayStatus requires the immutable digest, so reusing an ID cannot reveal a
// successful receipt for different content. Unknown IDs are not acknowledgements.
func (c *Client) RelayStatus(ctx context.Context, id, digest string) (RelayOutcome, error) {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return RelayOutcome{}, ErrRequestScope
	}
	if !(journal.Request{ID: id, Digest: digest}).Valid() {
		return RelayOutcome{}, ErrInvalidRequest
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return RelayOutcome{}, err
	}
	defer release()
	return c.relayOutcomeLocked(ctx, id, digest)
}

func (c *Client) relayOutcomeLocked(ctx context.Context, id, digest string) (RelayOutcome, error) {
	state, err := c.queue.read(ctx)
	if err != nil {
		return RelayOutcome{}, err
	}
	i, found := state.byID[id]
	if !found || state.entries[i].Digest != digest || state.entries[i].Custody == nil {
		return RelayOutcome{}, ErrRequestConflict
	}
	e := state.entries[i]
	settings, err := c.settings()
	if err != nil {
		return RelayOutcome{}, err
	}
	if e.Custody.LibraryID != settings.LibraryID || e.Custody.RelayID != settings.ID {
		return RelayOutcome{}, ErrInvariantViolated
	}
	result := RelayOutcome{Custody: *e.Custody, Confirmed: e.Receipt, Sending: e.Sending, Conflict: e.Conflict, Discarded: e.Discarded, Reason: e.Reason}
	if e.ReplacedBy != "" {
		p := state.entries[state.byID[e.ReplacedBy]].Proposal
		result.Replacement = &p
	}
	view, err := c.rebuildLocked(ctx)
	if err != nil {
		return RelayOutcome{}, err
	}
	if result.Conflict == "" && view.BlockedID == id && view.Conflict != nil {
		result.Conflict = view.Conflict.Error()
	}
	// The root may have committed through a different route, or its reply may
	// have been lost. Only published catalog lines can establish that outcome.
	head, _, err := c.boundary()
	if err != nil {
		return RelayOutcome{}, err
	}
	err = c.catalog.Iter(func(line journal.Line) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID > head.OpID || line.Sync == nil || line.Sync.ID != id {
			return nil
		}
		if line.Sync.Digest != digest || e.Discarded || e.ReplacedBy != "" {
			return fmt.Errorf("%w: relay intent contradicts root history", ErrInvariantViolated)
		}
		at, err := journal.ParseTime(line.Sync.AcceptedAt)
		if err != nil {
			return err
		}
		receipt := SyncResult{OpID: line.ID, AcceptedAt: at}
		if result.Confirmed != nil && (result.Confirmed.OpID != receipt.OpID || !result.Confirmed.AcceptedAt.Equal(at)) {
			return ErrInvariantViolated
		}
		result.Confirmed, result.Sending, result.Conflict = &receipt, false, ""
		return nil
	})
	return result, err
}

// StoreRelayOriginal verifies received bytes before placing them outside the
// cache. No receipt is issued here; ReceiveRelay validates the referencing intent.
func (c *Client) StoreRelayOriginal(ctx context.Context, name string, size int64, source io.Reader) error {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return ErrRequestScope
	}
	release, err := c.localPreparationLease(ctx)
	if err != nil {
		return err
	}
	defer release()
	return (&Library{blobs: c.blobs}).StoreSyncOriginal(ctx, name, size, source)
}

// SyncOriginal acknowledges protected bytes or ownership proven by root history.
func (c *Client) SyncOriginal(ctx context.Context, key string) (SyncOriginalStatus, error) {
	status, err := (&Library{blobs: c.blobs}).SyncOriginal(ctx, key)
	if err != nil || status.Present {
		return status, err
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return status, err
	}
	head, _, err := c.boundary()
	known := false
	if err == nil {
		err = c.catalog.Iter(func(line journal.Line) error {
			if line.ID > head.OpID {
				return nil
			}
			h, err := oplog.Lookup(oplog.Action(line.Action))
			if err != nil {
				return err
			}
			targets, err := h.Targets(line.Payload)
			if err != nil {
				return err
			}
			for _, name := range targets.Blobs {
				if name == key {
					known = true
				}
			}
			return ctx.Err()
		})
	}
	release()
	if err != nil || !known {
		return status, err
	}
	size, err := c.originalSize(ctx, key)
	return SyncOriginalStatus{Present: err == nil, Size: size}, err
}
func (c *Client) StoreSyncOriginal(ctx context.Context, key string, size int64, r io.Reader) error {
	return c.StoreRelayOriginal(ctx, key, size, r)
}

// ReceiveRelayAt atomically rejects an independently advanced upstream view.
func (c *Client) ReceiveRelayAt(ctx context.Context, id domain.ID, p SyncProposal, expected RelayHead) (RelayOutcome, error) {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return RelayOutcome{}, ErrRequestScope
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return RelayOutcome{}, err
	}
	defer release()
	state, err := c.queue.read(ctx)
	if err != nil {
		return RelayOutcome{}, err
	}
	digest, err := p.digest()
	if err != nil {
		return RelayOutcome{}, &SyncRejectedError{Cause: err}
	}
	if i, ok := state.byID[p.ID]; ok && state.entries[i].Digest == digest {
		return c.receiveRelayLocked(ctx, id, p)
	}
	settings, err := c.settings()
	if err != nil {
		return RelayOutcome{}, err
	}
	root, complete, err := c.boundary()
	if err != nil {
		return RelayOutcome{}, err
	}
	actual := RelayHead{Root: root, Node: settings.ID, Sequence: state.last, Sum: state.sum, Count: len(state.entries)}
	if !complete || actual != expected {
		return RelayOutcome{}, &SyncRejectedError{Cause: ErrSyncConflict}
	}
	return c.receiveRelayLocked(ctx, id, p)
}
