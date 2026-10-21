package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// SetBoxCover changes only the soft reference, preserving it while a file is trashed.
func (l *Library) SetBoxCover(ctx context.Context, box domain.BoxID, file domain.FileID) error {
	b, found, err := l.Box(ctx, box)
	if err != nil {
		return err
	}
	if !found {
		return ErrNoSuchBox
	}
	if b.CoverFileID == file {
		return nil
	}
	raw, err := oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box, CoverFileID: &oplog.CoverChange{Before: b.CoverFileID, After: file}})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionUpdateBox, raw, oplog.OriginUser, writeOptions{})
	return err
}

// checkBoxCover runs under the writer lock, after catch-up and before append.
// Undo may restore a reference to a trashed file, so it deliberately skips this check.
func checkBoxCover(tx *store.ViewTx, box domain.BoxID, file domain.FileID) error {
	if file.IsZero() {
		return nil
	}
	f, found, err := tx.GetFile(file)
	if err != nil {
		return err
	}
	if !found {
		return ErrNoSuchFile
	}
	owner, _, err := store.FolderChain(tx.Queryer(), f.FolderID)
	if err != nil {
		return err
	}
	if owner != box {
		return fmt.Errorf("cover file must belong to box: %w", oplog.ErrStaleBefore)
	}
	return nil
}

// BoxCover resolves the current file, never a stale cached thumbnail or deleted row.
func (l *Library) BoxCover(ctx context.Context, box domain.BoxID) (domain.File, bool, error) {
	b, found, err := l.Box(ctx, box)
	if err != nil {
		return domain.File{}, false, err
	}
	if !found {
		return domain.File{}, false, ErrNoSuchBox
	}
	if b.CoverFileID.IsZero() {
		return domain.File{}, false, nil
	}
	f, found, err := l.File(ctx, b.CoverFileID)
	if err != nil || !found {
		return f, false, err
	}
	owner, _, err := l.FileLocation(ctx, f)
	return f, owner == box, err
}
