package library

import (
	"context"
	"time"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// withConfirmedHistory never exposes transient pending ordinals as NAS IDs.
func (c *Client) withConfirmedHistory(ctx context.Context, fn func(*store.ViewTx) error) error {
	status, err := c.CatalogStatus(ctx)
	if err != nil {
		return err
	}
	if status.Confirmed.OpID == 0 {
		// An empty catalog still needs a queryable empty snapshot for pagination.
		return c.emptyHistory(ctx, fn)
	}
	return c.ViewAt(ctx, status.Confirmed.OpID, func(v *ClientHistoricalView) error {
		return c.readView(v.context(ctx), fn)
	})
}

func (c *Client) History(ctx context.Context, opt ListOpsOptions) ([]HistoryEntry, int64, error) {
	var entries []HistoryEntry
	var before int64
	err := c.withConfirmedHistory(ctx, func(v *store.ViewTx) error {
		var err error
		entries, before, err = readHistory(ctx, v.Queryer(), opt)
		return err
	})
	return entries, before, err
}

func (c *Client) HistoryOp(ctx context.Context, id int64) (HistoryEntry, error) {
	var entry HistoryEntry
	err := c.withConfirmedHistory(ctx, func(v *store.ViewTx) error {
		var err error
		entry, err = readHistoryOp(v.Queryer(), id)
		return err
	})
	return entry, err
}

func (c *Client) HistorySummary(ctx context.Context, id int64) (HistorySummary, error) {
	var summary HistorySummary
	err := c.withConfirmedHistory(ctx, func(v *store.ViewTx) error {
		row, found, err := store.GetOp(v.Queryer(), id)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchOp
		}
		summary, err = summarizeHistory(v, oplog.Action(row.Action), row.Payload)
		return err
	})
	return summary, err
}

func (c *Client) emptyHistory(ctx context.Context, fn func(*store.ViewTx) error) error {
	db, err := store.Open(":memory:")
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(store.NewViewTx(tx, time.Time{}))
}
