package library

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
	"math"
	"strconv"
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
		if key == "orientation:clockwise_degrees" {
			change.BlobKey = file.BlobKey
			removeLegacyOrientationHash(change, rows)
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

// MetadataEdit binds an edit to the exact value and content the user saw.
// Only existing keys are edited; the declared scalar type is preserved.
type MetadataEdit struct {
	BlobKey      string `json:"blob_key"`
	Key          string `json:"key"`
	BeforeValue  string `json:"before_value"`
	BeforeSource string `json:"before_source"`
	BeforeType   string `json:"before_type"`
	Value        string `json:"value"`
}

func editMetadataChange(tx *store.ViewTx, id domain.FileID, edit MetadataEdit, zone *time.Location) (*oplog.FileMetadataChange, error) {
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
	rows, err := store.FileMetadata(tx.Queryer(), id)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		if row.Key != edit.Key || row.Tombstone {
			continue
		}
		if row.Value != edit.BeforeValue || row.Source != edit.BeforeSource || string(row.Type) != edit.BeforeType {
			return nil, oplog.ErrStaleBefore
		}
		kv := extract.KV{Key: row.Key, Value: edit.Value, Type: row.Type}
		if err := extract.CheckValue(kv); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRequest, err)
		}
		if kv.Type == extract.TypeReal {
			n, _ := strconv.ParseFloat(kv.Value, 64)
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, ErrInvalidRequest
			}
		}
		if kv.Key == "orientation:clockwise_degrees" && kv.Value != "0" && kv.Value != "90" && kv.Value != "180" && kv.Value != "270" {
			return nil, ErrInvalidRequest
		}
		if row.Value == edit.Value {
			return nil, nil
		}
		before := &oplog.MetadataState{Value: row.Value, Type: row.Type, Source: row.Source}
		rows[i].Value, rows[i].Source = edit.Value, extract.UserSource
		change := &oplog.FileMetadataChange{
			FileID:      id,
			Keys:        []oplog.KeyChange{{Key: row.Key, Before: before, After: &oplog.MetadataState{Value: edit.Value, Type: row.Type, Source: extract.UserSource}}},
			CapturedWas: file.CapturedAt, CapturedAt: capturedFrom(observations(rows), zone),
		}
		if row.Key == "orientation:clockwise_degrees" {
			change.BlobKey = file.BlobKey
			removeLegacyOrientationHash(change, rows)
		}
		return change, nil
	}
	return nil, ErrNoSuchMetadata
}
