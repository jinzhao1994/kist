package library

import (
	"context"
	"strconv"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// PreparedExtraction contains the result of one extractor on one file. Slow
// parsing happens before ExecuteRequest acquires the writer lock.
type PreparedExtraction struct {
	library *Library
	payload []byte
}

func (l *Library) PrepareExtraction(ctx context.Context, id domain.FileID, extractor string) (*PreparedExtraction, error) {
	if ctx.Value(requestContextKey{}) != nil {
		return nil, ErrRequestScope
	}
	e, err := extract.Lookup(extractor)
	if err != nil {
		return nil, err
	}
	_, found, err := l.File(ctx, id)
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
	if versions[e.ID()] >= e.Version() {
		return prepared, nil
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
	_, err := p.library.write(ctx, oplog.ActionExtractMetadata, p.payload, oplog.OriginExtractor, writeOptions{})
	return err
}

// MediaInfo is persisted classification plus extraction work still needed by
// the current build. Missing classification is distinct from the "other" kind.
type MediaInfo struct {
	Size    *int64
	MIME    string
	Kind    string
	Pending []string
}

func (l *Library) MediaInfo(ctx context.Context, id domain.FileID) (MediaInfo, error) {
	out := MediaInfo{Pending: []string{}}
	rows, err := l.FileMetadata(ctx, id)
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
	versions, err := store.Extractions(l.db, id)
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
