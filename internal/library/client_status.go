package library

import (
	"context"
	"errors"
	"os"
	"time"
)

type ClientPendingStatus struct {
	ID              string    `json:"id"`
	Sequence        int64     `json:"sequence"`
	OccurredAt      time.Time `json:"occurred_at"`
	Action          string    `json:"action"`
	AwaitingRoot    bool      `json:"awaiting_root"`
	AwaitingReceipt bool      `json:"awaiting_receipt"`
	Conflict        string    `json:"conflict,omitempty"`
}

type ClientStatus struct {
	Settings   ClientSettings        `json:"settings"`
	Connection *ClientConnection     `json:"connection,omitempty"`
	Catalog    ClientCatalogStatus   `json:"catalog"`
	Pending    []ClientPendingStatus `json:"pending"`
	BlockedID  string                `json:"blocked_id,omitempty"`
}

// Status derives a coherent offline snapshot while holding the client lock.
// An unacknowledged sending intent remains uncertain, never safe to discard.
func (c *Client) Status(ctx context.Context) (ClientStatus, error) {
	var status ClientStatus
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return status, err
	}
	defer release()
	status.Settings, err = c.settings()
	if err != nil {
		return status, err
	}
	connection, _, err := c.connection()
	if err == nil {
		status.Connection = &connection
	} else if !errors.Is(err, os.ErrNotExist) {
		return status, err
	}
	status.Catalog, err = c.catalogStatusLocked()
	if err != nil {
		return status, err
	}
	view, err := c.rebuildLocked(ctx)
	if err != nil {
		return status, err
	}
	status.BlockedID = view.BlockedID
	status.Pending, err = c.pendingStatuses(ctx, view)
	if err != nil {
		return status, err
	}

	return status, nil
}
