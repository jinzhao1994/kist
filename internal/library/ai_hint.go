package library

import (
	"context"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// AIHintEdit distinguishes a missing hint from an explicitly saved empty hint.
type AIHintEdit struct {
	BlobKey string  `json:"blob_key"`
	Before  *string `json:"before"`
	Value   string  `json:"value"`
}

func aiHintChange(tx *store.ViewTx, id domain.FileID, kind string, edit AIHintEdit) (*oplog.FileMetadataChange, error) {
	if kind != "ocr" && kind != "desc" {
		return nil, ErrInvalidRequest
	}
	file, found, err := tx.GetFile(id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNoSuchFile
	}
	if file.BlobKey != edit.BlobKey {
		return nil, oplog.ErrStaleBefore
	}
	key := kind + ":hint"
	if err := extract.CheckValue(extract.KV{Key: key, Value: edit.Value, Type: extract.TypeText}); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRequest, err)
	}
	rows, err := store.FileMetadata(tx.Queryer(), id)
	if err != nil {
		return nil, err
	}
	var before *oplog.MetadataState
	for _, row := range rows {
		if row.Key != key {
			continue
		}
		if row.Tombstone || row.Source != extract.UserSource || row.Type != extract.TypeText {
			return nil, oplog.ErrStaleBefore
		}
		before = &oplog.MetadataState{Value: row.Value, Type: row.Type, Source: row.Source}
	}
	if (before == nil) != (edit.Before == nil) || before != nil && before.Value != *edit.Before {
		return nil, oplog.ErrStaleBefore
	}
	if before == nil && edit.Value == "" || before != nil && before.Value == edit.Value {
		return nil, nil
	}
	return &oplog.FileMetadataChange{
		FileID: id, CapturedWas: file.CapturedAt, CapturedAt: file.CapturedAt,
		Keys: []oplog.KeyChange{{Key: key, Before: before, After: &oplog.MetadataState{Value: edit.Value, Type: extract.TypeText, Source: extract.UserSource}}},
	}, nil
}

// SetAIHint records reference information without starting inference.
func (l *Library) SetAIHint(ctx context.Context, id domain.FileID, kind string, edit AIHintEdit) error {
	var change *oplog.FileMetadataChange
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		change, err = aiHintChange(tx, id, kind, edit)
		return err
	})
	if err != nil || change == nil {
		return err
	}
	raw, err := oplog.Marshal(oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{*change}})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionSetMetadata, raw, oplog.OriginUser, writeOptions{})
	return err
}

// SetAIHint also works offline; only model invocation requires the primary host.
func (c *Client) SetAIHint(ctx context.Context, id domain.FileID, kind string, edit AIHintEdit) error {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	change, err := aiHintChange(store.NewViewTx(tx, time.Time{}), id, kind, edit)
	rollbackErr := tx.Rollback()
	if err != nil {
		return err
	}
	if rollbackErr != nil {
		return rollbackErr
	}
	if change == nil {
		return nil
	}
	_, err = c.commandIntentLocked(ctx, scope, oplog.ActionSetMetadata, oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{*change}}, SyncFileState{ID: id, BlobKey: edit.BlobKey})
	return err
}
