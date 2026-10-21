package library

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
	"time"
)

func removeMetadataChange(vtx *store.ViewTx, id domain.FileID, key string, zone *time.Location) (*oplog.FileMetadataChange, error) {
	var change *oplog.FileMetadataChange
	err := func() error {
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
			CapturedAt:  capturedFrom(withoutKey(rows, key), zone),
			CapturedWas: file.CapturedAt,
		}
		return nil
	}()
	return change, err
}

func capturedFrom(kvs []extract.KV, zone *time.Location) domain.Instant {
	captured := extract.ResolveCaptured(kvs, zone)
	if !captured.Known {
		return domain.Instant{}
	}
	return domain.NewInstant(captured.UnixNano)
}
