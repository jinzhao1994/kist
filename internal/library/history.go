package library

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"time"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// HistoryEntry describes an event and its current undo relationship.
// Undoable means the action has an inverse, not that its preconditions still hold.
type HistoryEntry struct {
	Op
	AcceptedAt *time.Time
	UndoneBy   *int64
	Undoable   bool
	Boxes      []domain.BoxID
	Files      []domain.FileID
}

// History reads one bounded page and its undo relationships in the same snapshot.
// It never reads payloads, including for imports with thousands of targets.
func (l *Library) History(ctx context.Context, opt ListOpsOptions) ([]HistoryEntry, int64, error) {
	tx, err := l.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	return readHistory(ctx, tx, opt)
}

func readHistory(ctx context.Context, tx store.Queryer, opt ListOpsOptions) ([]HistoryEntry, int64, error) {
	if opt.Limit < 0 || opt.Limit > query.MaxLimit || opt.Before < 0 || (opt.Group != nil && *opt.Group <= 0) {
		return nil, 0, fmt.Errorf("invalid history pagination: %w", ErrInvalidRequest)
	}
	if opt.Action != "" {
		if _, err := oplog.Lookup(oplog.Action(opt.Action)); err != nil {
			return nil, 0, fmt.Errorf("invalid history action: %w", ErrInvalidRequest)
		}
	}
	for _, action := range opt.Actions {
		if _, err := oplog.Lookup(oplog.Action(action)); err != nil {
			return nil, 0, ErrInvalidRequest
		}
	}
	limit := opt.Limit
	if limit == 0 {
		limit = query.DefaultLimit
	}
	opt.Limit = limit + 1
	rows, err := store.ListOps(tx, opt)
	if err != nil {
		return nil, 0, err
	}
	var before int64
	if len(rows) > limit {
		rows = rows[:limit]
		before = rows[limit-1].ID
	}
	entries := make([]HistoryEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := historyEntry(tx, row)
		if err != nil {
			return nil, 0, err
		}
		entries = append(entries, entry)
	}
	return entries, before, ctx.Err()
}

// HistoryOp reads the self-contained payload only for a requested event.
func (l *Library) HistoryOp(ctx context.Context, id int64) (HistoryEntry, error) {
	tx, err := l.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return HistoryEntry{}, err
	}
	defer tx.Rollback()
	return readHistoryOp(tx, id)
}

func readHistoryOp(tx store.Queryer, id int64) (HistoryEntry, error) {
	row, found, err := store.GetOp(tx, id)
	if err != nil {
		return HistoryEntry{}, err
	}
	if !found {
		return HistoryEntry{}, ErrNoSuchOp
	}
	entry, err := historyEntry(tx, row)
	if err != nil {
		return entry, err
	}
	handler, err := oplog.Lookup(entry.Action)
	if err != nil {
		return entry, err
	}
	targets, err := handler.Targets(entry.Payload)
	entry.Boxes, entry.Files = targets.Boxes, targets.Files
	return entry, err
}

// TrashOptions lists uncompensated deletion events, preserving atomic restore.
func TrashOptions(opt ListOpsOptions) ListOpsOptions {
	opt.Actions = []string{string(oplog.ActionDeleteFilesetEntry), string(oplog.ActionDeleteEntryContents), string(oplog.ActionDeleteFiles), string(oplog.ActionDeleteFolder), string(oplog.ActionDeleteBox), string(oplog.ActionDeleteBoxContents), string(oplog.ActionDeleteTag)}
	opt.Unreverted = true
	return opt
}

func historyEntry(q store.Queryer, row store.OpRow) (HistoryEntry, error) {
	entry := HistoryEntry{Op: toOp(row), Undoable: row.Action != string(oplog.ActionRequestReceipt)}
	accepted, err := store.OpAcceptedAt(q, row.ID)
	if err != nil {
		return entry, err
	}
	entry.AcceptedAt = accepted
	by, found, err := store.RevertedBy(q, row.ID)
	if found {
		entry.UndoneBy = &by
	}
	return entry, err
}
