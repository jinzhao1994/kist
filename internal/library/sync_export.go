package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
)

// SyncBoundary identifies a fixed authoritative history prefix. Sum detects
// rollback or replacement of its final event, even when an ID is reused.
type SyncBoundary struct {
	LibraryID domain.ID `json:"library_id"`
	OpID      int64     `json:"op_id"`
	Sum       string    `json:"sum"`
}

type SyncCatalogPage struct {
	Boundary SyncBoundary      `json:"boundary"`
	After    int64             `json:"after"`
	Next     int64             `json:"next"`
	Done     bool              `json:"done"`
	Lines    []json.RawMessage `json:"lines"`
}

// SyncHead fixes the prefix to fetch without preventing subsequent NAS edits.
func (l *Library) SyncHead(ctx context.Context) (SyncBoundary, error) {
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return SyncBoundary{}, err
	}
	defer release()
	if err := l.catchUpLocked(ctx); err != nil {
		return SyncBoundary{}, err
	}
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return SyncBoundary{}, err
	}
	head := SyncBoundary{LibraryID: cfg.Library.ID}
	line, found, err := l.ops.Last()
	if found {
		head.OpID, head.Sum = line.ID, line.Sum
	}
	return head, err
}

// SyncCatalog reads original checked envelopes rather than reconstructing them
// from materialized op rows. The byte target is soft for a single large event:
// old imports may exceed it and cannot be split without changing their checksum.
// Memory is bounded by maxBytes plus the largest individual journal event.
func (l *Library) SyncCatalog(ctx context.Context, boundary SyncBoundary, after int64, limit, maxBytes int) (SyncCatalogPage, error) {
	page := SyncCatalogPage{Boundary: boundary, After: after, Next: after, Lines: []json.RawMessage{}}
	if after < 0 || boundary.OpID < after || limit < 1 || limit > 256 || maxBytes < 1 || maxBytes > 4<<20 || (boundary.OpID == 0) != (boundary.Sum == "") {
		return page, ErrInvalidRequest
	}
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return page, err
	}
	defer release()
	if err := l.catchUpLocked(ctx); err != nil {
		return page, err
	}
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return page, err
	}
	if cfg.Library.ID != boundary.LibraryID {
		return page, fmt.Errorf("%w: library identity differs", ErrSyncConflict)
	}
	return exportSyncCatalog(ctx, l.ops, page, limit, maxBytes)
}

func exportSyncCatalog(ctx context.Context, log *journal.Dir, page SyncCatalogPage, limit, maxBytes int) (SyncCatalogPage, error) {
	boundary, after := page.Boundary, page.After
	var last int64
	var bytes int
	full := false
	matched := boundary.OpID == 0
	end := errors.New("sync boundary reached")
	// Keep validating through the boundary after filling a page. This avoids
	// returning a plausible page from a replaced or truncated authoritative log.
	err := log.Iter(func(line journal.Line) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID > boundary.OpID {
			return end
		}
		if line.ID != last+1 {
			return fmt.Errorf("%w: noncontiguous sync history", journal.ErrCorrupt)
		}
		last = line.ID
		if line.ID == boundary.OpID {
			matched = line.Sum == boundary.Sum
		}
		if line.ID <= after || full {
			return nil
		}
		raw, err := json.Marshal(line)
		if err != nil {
			return err
		}
		if len(page.Lines) >= limit || (len(page.Lines) > 0 && bytes+len(raw) > maxBytes) {
			full = true
			return nil
		}
		page.Lines = append(page.Lines, raw)
		bytes += len(raw)
		page.Next = line.ID
		return nil
	})
	if err != nil && !errors.Is(err, end) {
		return SyncCatalogPage{}, err
	}
	if !matched {
		return SyncCatalogPage{}, fmt.Errorf("%w: authoritative history boundary changed", ErrSyncConflict)
	}
	page.Done = page.Next == boundary.OpID
	return page, nil
}
