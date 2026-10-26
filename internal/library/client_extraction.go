package library

import (
	"context"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// PreparedClientExtraction retains facts and the exact original they describe.
// Parsing holds an original read lease, never the client editing lock.
type PreparedClientExtraction struct {
	client  *Client
	payload []byte
	file    SyncFileState
}

func (c *Client) PrepareExtraction(ctx context.Context, id domain.FileID, extractor string) (*PreparedClientExtraction, error) {
	return c.prepareExtraction(ctx, id, extractor, false)
}

// PrepareReExtraction explicitly refreshes existing observations.
func (c *Client) PrepareReExtraction(ctx context.Context, id domain.FileID, extractor string) (*PreparedClientExtraction, error) {
	return c.prepareExtraction(ctx, id, extractor, true)
}

func (c *Client) prepareExtraction(ctx context.Context, id domain.FileID, extractor string, force bool) (*PreparedClientExtraction, error) {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return nil, ErrRequestScope
	}
	e, err := extract.Lookup(extractor)
	if err != nil {
		return nil, err
	}
	var file domain.File
	var rows []store.MetadataRow
	var previous int
	var zone *time.Location
	err = c.readView(ctx, func(tx *store.ViewTx) error {
		var found bool
		file, found, err = tx.GetFile(id)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchFile
		}
		rows, err = store.FileMetadata(tx.Queryer(), id)
		if err != nil {
			return err
		}
		versions, err := store.Extractions(tx.Queryer(), id)
		if err != nil {
			return err
		}
		previous = versions[e.ID()]
		settings, err := c.settings()
		if err != nil {
			return err
		}
		zone, err = time.LoadLocation(settings.Timezone)
		return err
	})
	if err != nil {
		return nil, err
	}
	prepared := &PreparedClientExtraction{client: c, file: SyncFileState{ID: id, BlobKey: file.BlobKey}}
	if !force && previous >= e.Version() {
		return prepared, nil
	}
	reader, err := c.OpenOriginal(ctx, file.BlobKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	mime, err := extract.DetectMedia(reader)
	if err != nil {
		return nil, err
	}
	change := &oplog.FileExtraction{FileID: id, CapturedAt: file.CapturedAt, CapturedWas: file.CapturedAt}
	if e.Wants(mime) {
		if _, err := reader.Seek(0, 0); err != nil {
			return nil, err
		}
		facts, err := e.Extract(ctx, reader, mime)
		if err != nil {
			return nil, err
		}
		if err := extract.Check(e, facts); err != nil {
			return nil, err
		}
		change = extractionChange(e, file, facts, rows, zone)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prepared.payload, err = oplog.Marshal(oplog.ExtractMetadataPayload{Extractor: e.ID(), Version: e.Version(), PreviousVersion: previous, Files: []oplog.FileExtraction{*change}})
	return prepared, err
}

func (p *PreparedClientExtraction) Commit(ctx context.Context) error {
	scope, release, err := p.client.commandLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	if len(p.payload) == 0 {
		return nil
	}
	_, err = p.client.stageCommandLocked(ctx, scope, SyncProposal{Version: 1, ID: domain.NewID().String(), OccurredAt: time.Now().UTC(), Action: string(oplog.ActionExtractMetadata), Origin: string(oplog.OriginExtractor), Payload: p.payload, Files: []SyncFileState{p.file}})
	return err
}
