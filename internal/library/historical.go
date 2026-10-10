package library

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// historicalReader deliberately excludes every command and maintenance method.
// Its concrete implementation is private to this package.
type historicalReader interface {
	Box(context.Context, domain.BoxID) (domain.Box, bool, error)
	Boxes(context.Context, ListBoxesOptions) ([]domain.Box, error)
	BoxTags(context.Context, domain.BoxID) ([]domain.TagID, error)
	File(context.Context, domain.FileID) (domain.File, bool, error)
	Files(context.Context, query.Query) ([]domain.File, string, error)
	Folders(context.Context, domain.BoxID) ([]domain.Folder, error)
	FileTags(context.Context, domain.FileID) ([]domain.TagID, error)
	FileLocation(context.Context, domain.File) (domain.BoxID, string, error)
	FileMetadata(context.Context, domain.FileID) ([]extract.Row, error)
	CapturedInfo(context.Context, domain.File) (CapturedInfo, error)
	MediaInfo(context.Context, domain.FileID) (MediaInfo, error)
	OpenContent(context.Context, domain.FileID) (domain.File, *os.File, error)
	TagTree() (*TagTree, map[domain.TagID]int, error)
	TagCondition([]string) (query.TagExpr, error)
	ExcludeTags([]string) (query.TagExpr, error)
	Zone() *time.Location
}

// HistoricalView reads the state immediately after a particular operation.
// It is valid only during the ViewAt callback. Original content handles opened
// by the callback belong to the caller and remain valid after the callback.
type HistoricalView struct {
	historicalReader
}

// ViewAt replays the global journal prefix into a private temporary database.
// It never runs commands, extraction or projection work against the live library.
// The target must name an applied event; stopping at that event also avoids
// observing a concurrent append or incomplete future tail. No writer lock is
// held during replay. The callback must finish its reads before returning.
func (l *Library) ViewAt(ctx context.Context, target int64, fn func(*HistoricalView) error) error {
	if target <= 0 || fn == nil {
		return ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Do not fetch a potentially large payload merely to validate the cutoff.
	var exists bool
	if err := l.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ops WHERE id = ?)`, target).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNoSuchOp
	}
	// Use the archive filesystem, whose capacity already accommodates its index.
	// Each request owns a unique directory, including its WAL and SHM files.
	dir, err := os.MkdirTemp(filepath.Dir(l.layout.DB()), ".kist-history-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	db, err := store.Open(filepath.Join(dir, "history.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	reached := errors.New("history cutoff reached")
	var last int64
	err = l.ops.Iter(func(line journal.Line) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID != last+1 {
			return fmt.Errorf("non-contiguous history at op %d: %w", line.ID, ErrInvariantViolated)
		}
		row, err := rowFromLine(line)
		if err != nil {
			return err
		}
		if _, err := applyOp(tx, row); err != nil {
			return fmt.Errorf("replaying history: %w", err)
		}
		last = line.ID
		if last == target {
			return reached
		}
		return nil
	})
	if !errors.Is(err, reached) {
		if err != nil {
			return err
		}
		return fmt.Errorf("history cutoff %d missing from journal: %w", target, ErrInvariantViolated)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	view := &HistoricalView{historicalReader: &Library{db: db, blobs: l.blobs, loc: l.loc}}
	if err := fn(view); err != nil {
		return err
	}
	return ctx.Err()
}
