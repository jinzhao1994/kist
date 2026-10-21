package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
)

type ClientCatalogStatus struct {
	Confirmed  SyncBoundary  `json:"confirmed"`
	Complete   bool          `json:"complete"`
	Transfer   *SyncBoundary `json:"transfer,omitempty"`
	Downloaded int64         `json:"downloaded"`
}

func (c *Client) CatalogStatus(ctx context.Context) (ClientCatalogStatus, error) {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return ClientCatalogStatus{}, err
	}
	defer release()
	return c.catalogStatusLocked()
}

func (c *Client) catalogStatusLocked() (ClientCatalogStatus, error) {
	var status ClientCatalogStatus
	var err error
	status.Confirmed, status.Complete, err = c.boundary()
	if err != nil {
		return status, err
	}
	if _, err := c.catalog.RecoverTail(); err != nil {
		return status, err
	}
	last, found, err := c.catalog.Last()
	if err != nil {
		return status, err
	}
	if found {
		status.Downloaded = last.ID
	}
	var transfer SyncBoundary
	err = readClientJSON(filepath.Join(c.root, "catalog-transfer.json"), &transfer)
	if err == nil {
		status.Transfer = &transfer
	} else if !errors.Is(err, os.ErrNotExist) {
		return status, err
	}
	return status, nil
}

// ReceiveCatalogPage durably resumes an authenticated NAS history transfer.
// Partial downloads never advance the visible/complete metadata boundary.
func (c *Client) ReceiveCatalogPage(ctx context.Context, page SyncCatalogPage) error {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	status, err := c.catalogStatusLocked()
	if err != nil {
		return err
	}
	if page.Boundary.LibraryID != status.Confirmed.LibraryID || page.Boundary.OpID < status.Confirmed.OpID || page.After < 0 || page.Next < page.After || page.Next > page.Boundary.OpID || page.Done != (page.Next == page.Boundary.OpID) || (page.Boundary.OpID == 0) != (page.Boundary.Sum == "") || page.After > status.Downloaded {
		return ErrInvalidRequest
	}
	// A completed transfer marker can survive a crash after publishing head.
	if status.Transfer != nil && *status.Transfer == status.Confirmed && page.Boundary != status.Confirmed {
		status.Transfer = nil
	}
	if status.Transfer != nil && *status.Transfer != page.Boundary {
		return ErrSyncConflict
	}
	if status.Transfer == nil && page.After != status.Confirmed.OpID {
		return ErrSyncDependency
	}
	lines := make([]journal.Line, 0, len(page.Lines))
	next := page.After
	for _, raw := range page.Lines {
		line, err := journal.UnmarshalLine(raw)
		if err != nil {
			return err
		}
		if line.ID != next+1 {
			return ErrSyncDependency
		}
		if _, err := oplog.Lookup(oplog.Action(line.Action)); err != nil {
			return ErrSyncVersion
		}
		next = line.ID
		lines = append(lines, line)
	}
	if next != page.Next || (!page.Done && len(lines) == 0) {
		return ErrInvalidRequest
	}
	existing := map[int64]string{}
	if err := c.catalog.Iter(func(line journal.Line) error {
		if line.ID >= page.After && line.ID <= page.Next {
			existing[line.ID] = line.Sum
		}
		return ctx.Err()
	}); err != nil {
		return err
	}
	for _, line := range lines {
		if line.ID <= status.Downloaded && existing[line.ID] != line.Sum {
			return ErrSyncConflict
		}
	}
	if page.Done && page.Next > 0 {
		sum := existing[page.Next]
		if len(lines) > 0 {
			sum = lines[len(lines)-1].Sum
		}
		if sum != page.Boundary.Sum {
			return ErrSyncConflict
		}
	}
	if err := writeClientJSON(filepath.Join(c.root, "catalog-transfer.json"), page.Boundary); err != nil {
		return err
	}
	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID <= status.Downloaded {
			continue
		}
		if err := c.catalog.AppendVerified(line); err != nil {
			return err
		}
	}
	if !page.Done {
		return nil
	}
	if err := writeClientJSON(filepath.Join(c.root, "catalog-head.json"), page.Boundary); err != nil {
		return err
	}
	_, err = c.rebuildLocked(ctx)
	return err
}
