package library

import (
	"context"
	"time"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// PendingHistory lists locally applied events not yet present in confirmed
// history. Pagination uses durable local identities, never overlay ordinals.
func (c *Client) PendingHistory(ctx context.Context, before string, opt ListOpsOptions) ([]ClientLocalHistoryEntry, string, error) {
	if opt.Limit < 0 || opt.Limit > query.MaxLimit {
		return nil, "", ErrInvalidRequest
	}
	limit := opt.Limit
	if limit == 0 {
		limit = query.DefaultLimit
	}
	out := []ClientLocalHistoryEntry{}
	next := ""
	err := c.withLocalHistory(ctx, func(state pendingState, view clientOverlay) error {
		ids := map[string]int64{}
		for id, ref := range view.References {
			if ref.LocalID != "" {
				ids[ref.LocalID] = id
			}
		}
		end := len(state.entries)
		if before != "" {
			index, found := state.byID[before]
			if !found {
				return ErrNoSuchOp
			}
			end = index
		}
		for i := end - 1; i >= 0; i-- {
			entry := state.entries[i]
			id, found := ids[entry.Proposal.ID]
			if !found {
				continue
			}
			filter := opt
			filter.Before = id + 1
			filter.Limit = 1
			rows, err := store.ListOps(c.db, filter)
			if err != nil {
				return err
			}
			if len(rows) == 0 || rows[0].ID != id {
				continue
			}
			if len(out) == limit {
				next = out[len(out)-1].ID
				break
			}
			item, err := c.localHistoryEntry(entry, view)
			if err != nil {
				return err
			}
			out = append(out, item)
		}
		tx, err := c.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for i := range out {
			p := state.entries[state.byID[out[i].ID]].Proposal
			if p.Undo != nil {
				continue
			}
			summary, err := summarizeHistory(store.NewViewTx(tx, time.Time{}), oplog.Action(p.Action), p.Payload)
			if err != nil {
				return err
			}
			out[i].Summary = &summary
		}
		return nil
	})
	return out, next, err
}
