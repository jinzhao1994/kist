package library

import (
	"context"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// DeleteBox moves the entire current box into the log-backed trash in one op.
// The payload comes from one read snapshot; Apply rechecks it under the write
// lock so a concurrent import, tag edit or extraction cannot be lost.
func (l *Library) DeleteBox(ctx context.Context, id domain.BoxID, revision int64) (int64, error) {
	var contents domain.BoxContents
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		last, _, err := store.LastOpTouchingBox(tx.Queryer(), id, 0)
		if err != nil {
			return err
		}
		if revision != last {
			return oplog.ErrStaleBefore
		}
		var found bool
		contents, found, err = tx.BoxContents(id)
		if err == nil && !found {
			return ErrNoSuchBox
		}
		return err
	})
	if err != nil {
		return 0, err
	}
	raw, err := oplog.Marshal(contents)
	if err != nil {
		return 0, err
	}
	return l.write(ctx, oplog.ActionDeleteBoxContents, raw, oplog.OriginUser, writeOptions{})
}

// BoxDeletionPreview is the exact scope shown before a destructive action.
type BoxDeletionPreview struct {
	Name      string `json:"name"`
	FileCount int    `json:"file_count"`
	Revision  int64  `json:"revision"`
}

// PreviewBoxDeletion reads the count and revision in one database snapshot.
func (l *Library) PreviewBoxDeletion(ctx context.Context, id domain.BoxID) (BoxDeletionPreview, error) {
	var p BoxDeletionPreview
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		b, found, err := tx.GetBox(id)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchBox
		}
		p.Name = b.Name
		p.FileCount, err = tx.CountBoxFiles(id)
		if err != nil {
			return err
		}
		p.Revision, _, err = store.LastOpTouchingBox(tx.Queryer(), id, 0)
		return err
	})
	return p, err
}
