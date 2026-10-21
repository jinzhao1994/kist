package library

import (
	"context"
	"time"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// ClientLocalHistoryEntry keeps local intent identities separate from NAS IDs.
// Resolved intents remain addressable; only an actual receipt supplies an OpID.
type ClientLocalHistoryEntry struct {
	ID         string         `json:"id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Action     string         `json:"action"`
	Origin     string         `json:"origin"`
	State      string         `json:"state" enum:"pending,awaiting_receipt,conflict,accepted,discarded,replaced"`
	Receipt    *SyncResult    `json:"receipt,omitempty"`
	ReplacedBy string         `json:"replaced_by,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	Undo       *SyncReference `json:"undo,omitempty"`
	Group      *SyncReference `json:"group,omitempty"`
}

type ClientLocalHistoryDetail struct {
	ClientLocalHistoryEntry
	Proposal SyncProposal    `json:"proposal"`
	Summary  *HistorySummary `json:"summary,omitempty"`
}

// LocalHistory pages immutable intentions in reverse append order. A replacement
// has its own cursor even when it inherits the original logical queue position.
func (c *Client) LocalHistory(ctx context.Context, before string, limit int) ([]ClientLocalHistoryEntry, string, error) {
	if limit < 0 || limit > query.MaxLimit {
		return nil, "", ErrInvalidRequest
	}
	if limit == 0 {
		limit = query.DefaultLimit
	}
	var out []ClientLocalHistoryEntry
	var next string
	err := c.withLocalHistory(ctx, func(state pendingState, view clientOverlay) error {
		end := len(state.entries)
		if before != "" {
			index, found := state.byID[before]
			if !found {
				return ErrNoSuchOp
			}
			end = index
		}
		start := max(0, end-limit)
		out = make([]ClientLocalHistoryEntry, 0, end-start)
		for i := end - 1; i >= start; i-- {
			item, err := c.localHistoryEntry(state.entries[i], view)
			if err != nil {
				return err
			}
			out = append(out, item)
		}
		if start > 0 {
			next = state.entries[start].Proposal.ID
		}
		return nil
	})
	return out, next, err
}

func (c *Client) LocalHistoryOp(ctx context.Context, id string) (ClientLocalHistoryDetail, error) {
	var detail ClientLocalHistoryDetail
	err := c.withLocalHistory(ctx, func(state pendingState, view clientOverlay) error {
		index, found := state.byID[id]
		if !found {
			return ErrNoSuchOp
		}
		entry := state.entries[index]
		var err error
		detail.ClientLocalHistoryEntry, err = c.localHistoryEntry(entry, view)
		detail.Proposal = entry.Proposal
		if err != nil || entry.Proposal.Undo != nil {
			return err
		}
		tx, err := c.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		summary, err := summarizeHistory(store.NewViewTx(tx, time.Time{}), oplog.Action(entry.Proposal.Action), entry.Proposal.Payload)
		if err == nil {
			detail.Summary = &summary
		}
		return err
	})
	return detail, err
}

func (c *Client) withLocalHistory(ctx context.Context, fn func(pendingState, clientOverlay) error) error {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return ErrRequestScope
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	view, err := c.rebuildLocked(ctx)
	if err != nil {
		return err
	}
	state, err := c.queue.read(ctx)
	if err != nil {
		return err
	}
	if err := fn(state, view); err != nil {
		return err
	}
	return ctx.Err()
}

func (c *Client) localHistoryEntry(entry pendingEntry, view clientOverlay) (ClientLocalHistoryEntry, error) {
	p := entry.Proposal
	item := ClientLocalHistoryEntry{ID: p.ID, OccurredAt: p.OccurredAt, Action: p.Action, Origin: p.Origin, State: "pending", Receipt: entry.Receipt, ReplacedBy: entry.ReplacedBy, Reason: entry.Reason, Undo: p.Undo, Group: p.Group}
	if p.Undo != nil {
		item.Action = "undo"
	}
	if item.Receipt == nil {
		digest, id, accepted, found, err := store.SyncOp(c.db, p.ID)
		if err != nil {
			return item, err
		}
		if found {
			if digest != entry.Digest {
				return item, ErrRequestConflict
			}
			item.Receipt = &SyncResult{OpID: id, AcceptedAt: accepted}
		}
	}
	switch {
	case item.Receipt != nil:
		item.State = "accepted"
	case entry.Discarded:
		item.State = "discarded"
	case entry.ReplacedBy != "":
		item.State = "replaced"
	case entry.Sending:
		item.State = "awaiting_receipt"
	case entry.Conflict != "":
		item.State = "conflict"
		item.Reason = entry.Conflict
	case view.BlockedID == p.ID:
		item.State = "conflict"
		if view.Conflict != nil {
			item.Reason = view.Conflict.Error()
		}
	}
	return item, nil
}
