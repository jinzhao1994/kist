package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// publishClientView records only a projection of durable pending decisions.
// Missing or stale stamps force replay; no pending intent is authoritative here.
func (c *Client) publishClientView(stamp clientViewStamp, state pendingState) error {
	db, err := c.syncDatabase()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM pending_entries`); err != nil {
			return err
		}
		for position, e := range state.entries {
			var upstream any
			if e.Upstream != nil {
				upstream = e.Upstream.Position
			}
			action := e.Proposal.Action
			if e.Proposal.Undo != nil {
				action = "undo"
			}
			raw, err := json.Marshal(e)
			if err != nil {
				return err
			}
			_, err = tx.Exec(`INSERT INTO pending_entries VALUES(?,?,?,?,?,?,?,?,?,?,?)`, e.Proposal.ID, position, e.Sequence, upstream, e.active(), action, e.Proposal.OccurredAt.Format(time.RFC3339Nano), e.Upstream != nil || e.Forwarded != nil, e.Sending, e.Conflict, string(raw))
			if err != nil {
				return err
			}
		}
		return writeViewStampTx(tx, stamp)
	})
}
func (c *Client) readPendingProjection() (pendingState, error) {
	state := pendingState{byID: map[string]int{}}
	db, err := c.syncDatabase()
	if err != nil {
		return state, err
	}
	rows, err := db.DB.Query(`SELECT record FROM pending_entries ORDER BY position`)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return state, err
		}
		var entry pendingEntry
		if err = json.Unmarshal([]byte(raw), &entry); err != nil {
			return state, err
		}
		state.byID[entry.Proposal.ID] = len(state.entries)
		state.entries = append(state.entries, entry)
	}
	return state, rows.Err()
}
func (c *Client) pendingStatuses(ctx context.Context, view clientOverlay) ([]ClientPendingStatus, error) {
	out := []ClientPendingStatus{}
	db, err := c.syncDatabase()
	if err != nil {
		return nil, err
	}
	rows, err := db.DB.QueryContext(ctx, `SELECT id,sequence,occurred_at,action,awaiting_root,awaiting_receipt,conflict FROM pending_entries WHERE active=1 ORDER BY upstream_position IS NULL,upstream_position,sequence,position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ClientPendingStatus
		var at string
		if err = rows.Scan(&item.ID, &item.Sequence, &at, &item.Action, &item.AwaitingRoot, &item.AwaitingReceipt, &item.Conflict); err != nil {
			return nil, err
		}
		if item.OccurredAt, err = time.Parse(time.RFC3339Nano, at); err != nil {
			return nil, err
		}
		if item.ID == view.BlockedID && view.Conflict != nil && item.Conflict == "" {
			item.Conflict = view.Conflict.Error()
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
