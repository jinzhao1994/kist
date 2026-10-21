package library

import (
	"context"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// SetBoxCover stages a soft reference change without requiring cached content.
func (c *Client) SetBoxCover(ctx context.Context, box domain.BoxID, file domain.FileID) error {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	current, found, err := store.DisplayBox(c.db, box)
	if err != nil {
		return err
	}
	if !found {
		return ErrNoSuchBox
	}
	if current.CoverFileID == file {
		return nil
	}
	return c.commandWriteLocked(ctx, scope, oplog.ActionUpdateBox, oplog.UpdateBoxPayload{
		BoxID: box, CoverFileID: &oplog.CoverChange{Before: current.CoverFileID, After: file},
	})
}
