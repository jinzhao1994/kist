package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// ErrNoSuchMetadata is returned when a file has no such key to remove.
var ErrNoSuchMetadata = fmt.Errorf("no such metadata key")

// RemoveMetadata suppresses one key on one file.
//
// It leaves a tombstone rather than deleting the row, so the next extraction
// does not put the value back (spec 1.7.7). Removing a key that is already
// suppressed changes nothing and writes no op.
//
// captured_at is re-resolved here rather than inside Apply, because the
// resolution reads the library's timezone and configuration is not payload
// (spec 1.5.2, invariant 3).
func (l *Library) RemoveMetadata(ctx context.Context, id domain.FileID, key string) (bool, error) {
	var change *oplog.FileMetadataChange
	err := l.readTx(ctx, func(vtx *store.ViewTx) error {
		var err error
		change, err = removeMetadataChange(vtx, id, key, l.zone())
		return err
	})
	if err != nil || change == nil {
		return false, err
	}

	raw, err := oplog.Marshal(oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{*change}})
	if err != nil {
		return false, err
	}
	if _, err := l.write(ctx, oplog.ActionSetMetadata, raw, oplog.OriginUser, writeOptions{}); err != nil {
		return false, err
	}
	return true, nil
}

// withoutKey is what the file's metadata will look like once the key is gone.
func withoutKey(rows []store.MetadataRow, key string) []extract.KV {
	kept := make([]store.MetadataRow, 0, len(rows))
	for _, r := range rows {
		if r.Key != key {
			kept = append(kept, r)
		}
	}
	return observations(kept)
}

// resolveCaptured draws the kernel's conclusion from a set of observations.
func (l *Library) resolveCaptured(kvs []extract.KV) domain.Instant {
	return capturedFrom(kvs, l.zone())
}

// EditMetadata replaces or suppresses an existing value with an explicit human override.
func (l *Library) EditMetadata(ctx context.Context, id domain.FileID, edit MetadataEdit) error {
	var change *oplog.FileMetadataChange
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		change, err = editMetadataChange(tx, id, edit, l.zone())
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
