package library

import (
	"context"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func (c *Client) RenameEntry(ctx context.Context, box domain.BoxID, id domain.ID, folder bool, before, name string) (SyncReference, error) {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return SyncReference{}, err
	}
	defer release()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return SyncReference{}, err
	}
	snapshot, found, err := store.NewViewTx(tx, time.Time{}).EntryContents(box, id, folder)
	rollbackErr := tx.Rollback()
	if err != nil {
		return SyncReference{}, err
	}
	if rollbackErr != nil {
		return SyncReference{}, rollbackErr
	}
	if !found {
		return SyncReference{}, ErrNoSuchFile
	}
	p, err := renameEntryPayload(snapshot, before, name)
	if err != nil {
		return SyncReference{}, err
	}
	return c.commandIntentLocked(ctx, scope, oplog.ActionRenameEntry, p)
}

// RemoveMetadata suppresses a key with a tombstone. The file identity binds the
// decision to the content whose metadata the user was editing.
func (c *Client) RemoveMetadata(ctx context.Context, id domain.FileID, key string) (bool, error) {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	settings, err := c.settings()
	if err != nil {
		return false, err
	}
	zone, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return false, err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	vtx := store.NewViewTx(tx, time.Time{})
	change, err := removeMetadataChange(vtx, id, key, zone)
	var file domain.File
	if err == nil && change != nil {
		file, _, err = vtx.GetFile(id)
	}
	rollbackErr := tx.Rollback()
	if err != nil {
		return false, err
	}
	if rollbackErr != nil {
		return false, rollbackErr
	}
	if change == nil {
		return false, nil
	}
	_, err = c.commandIntentLocked(ctx, scope, oplog.ActionSetMetadata, oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{*change}}, SyncFileState{ID: id, BlobKey: file.BlobKey})
	return err == nil, err
}
