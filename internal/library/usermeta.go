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
		file, ok, err := vtx.GetFile(id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s: %w", id, ErrNoSuchFile)
		}
		rows, err := store.FileMetadata(vtx.SQL(), id)
		if err != nil {
			return err
		}

		var before *oplog.MetadataState
		for _, r := range rows {
			if r.Key != key {
				continue
			}
			if r.Tombstone {
				return nil // already suppressed
			}
			before = &oplog.MetadataState{Value: r.Value, Type: r.Type, Source: r.Source}
		}
		if before == nil {
			return fmt.Errorf("%s on %s: %w", key, id, ErrNoSuchMetadata)
		}

		change = &oplog.FileMetadataChange{
			FileID: id,
			Keys: []oplog.KeyChange{{
				Key:    key,
				Before: before,
				After:  &oplog.MetadataState{Source: extract.UserSource, Tombstone: true},
			}},
			CapturedAt:  l.resolveCaptured(withoutKey(rows, key)),
			CapturedWas: file.CapturedAt,
		}
		return nil
	})
	if err != nil || change == nil {
		return false, err
	}

	raw, err := oplog.Marshal(oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{*change}})
	if err != nil {
		return false, err
	}
	if _, err := l.write(ctx, oplog.ActionSetMetadata, raw, oplog.SourceUser, nil); err != nil {
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
	captured := extract.ResolveCaptured(kvs, l.zone())
	if !captured.Known {
		return domain.Instant{}
	}
	return domain.NewInstant(captured.UnixNano)
}
