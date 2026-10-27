package library

import (
	"context"
	"reflect"
	"strconv"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// PreparedExtraction contains the result of one extractor on one file. Slow
// parsing happens before ExecuteRequest acquires the writer lock.
type PreparedExtraction struct {
	library   *Library
	payload   []byte
	file      *domain.File
	rows      []store.MetadataRow
	extractor string
	version   int
}

func (l *Library) PrepareExtraction(ctx context.Context, id domain.FileID, extractor string) (*PreparedExtraction, error) {
	return l.prepareExtraction(ctx, id, extractor, false)
}

// PrepareReExtraction explicitly refreshes existing observations.
func (l *Library) PrepareReExtraction(ctx context.Context, id domain.FileID, extractor string) (*PreparedExtraction, error) {
	return l.prepareExtraction(ctx, id, extractor, true)
}

func (l *Library) prepareExtraction(ctx context.Context, id domain.FileID, extractor string, force bool) (*PreparedExtraction, error) {
	if ctx.Value(requestContextKey{}) != nil {
		return nil, ErrRequestScope
	}
	e, err := extract.Lookup(extractor)
	if err != nil {
		return nil, err
	}
	file, found, err := l.File(ctx, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNoSuchFile
	}
	versions, err := store.Extractions(l.db, id)
	if err != nil {
		return nil, err
	}
	prepared := &PreparedExtraction{library: l}
	if !force && versions[e.ID()] >= e.Version() {
		return prepared, nil
	}
	prepared.file, prepared.extractor, prepared.version = &file, e.ID(), versions[e.ID()]
	prepared.rows, err = store.FileMetadata(l.db, id)
	if err != nil {
		return nil, err
	}
	change, result := l.extractOne(ctx, e, id)
	if result.Err != nil {
		return nil, result.Err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if change == nil {
		return prepared, nil
	}
	prepared.payload, err = oplog.Marshal(oplog.ExtractMetadataPayload{Extractor: e.ID(), Version: e.Version(), PreviousVersion: versions[e.ID()], Files: []oplog.FileExtraction{*change}})
	return prepared, err
}
func (p *PreparedExtraction) Commit(ctx context.Context) error {
	if len(p.payload) == 0 {
		return nil
	}
	if p.file != nil {
		file, found, err := p.library.File(ctx, p.file.ID)
		if err != nil {
			return err
		}
		if !found || file.BlobKey != p.file.BlobKey || file.CapturedAt != p.file.CapturedAt {
			return ErrSyncConflict
		}
		rows, err := store.FileMetadata(p.library.db, p.file.ID)
		if err != nil {
			return err
		}
		versions, err := store.Extractions(p.library.db, p.file.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(rows, p.rows) || versions[p.extractor] != p.version {
			return ErrSyncConflict
		}
	}
	_, err := p.library.write(ctx, oplog.ActionExtractMetadata, p.payload, oplog.OriginExtractor, writeOptions{})
	return err
}

// MediaInfo is persisted classification plus extraction work still needed by
// the current build. Missing classification is distinct from the "other" kind.
type MediaInfo struct {
	Rotation       int
	RotationBlob   string
	RotationManual bool
	Size           *int64
	MIME           string
	Kind           string
	Pending        []string
}

func (l *Library) MediaInfo(ctx context.Context, id domain.FileID) (MediaInfo, error) {
	var result MediaInfo
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = readMediaInfo(tx.Queryer(), id)
		return err
	})
	return result, err
}

func readMediaInfo(q store.Queryer, id domain.FileID) (MediaInfo, error) {
	out := MediaInfo{Pending: []string{}}
	rows, err := store.FileMetadata(q, id)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		if row.Tombstone {
			continue
		}
		switch row.Key {
		case "file:size":
			if size, err := strconv.ParseInt(row.Value, 10, 64); err == nil && size >= 0 {
				out.Size = &size
			}
		case "media:mime":
			out.MIME = row.Value
		case "media:kind":
			out.Kind = row.Value
		}
	}
	for _, row := range rows {
		if row.Key == "orientation:input_sha256" && !row.Tombstone {
			out.RotationBlob = row.Value
		}
	}
	out.Rotation, _, out.RotationManual = imageRotation(rows, out.RotationBlob)
	versions, err := store.Extractions(q, id)
	if err != nil {
		return out, err
	}
	for _, e := range extract.All() {
		if versions[e.ID()] < e.Version() {
			out.Pending = append(out.Pending, e.ID())
		}
	}
	return out, nil
}
