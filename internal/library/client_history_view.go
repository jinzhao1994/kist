package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/store"
)

// ClientHistoricalView reads a confirmed NAS prefix, excluding pending edits.
// It exists only for the ViewAt callback; opened content keeps its cache lease.
type ClientHistoricalView struct{ *ClientView }

func (v *ClientHistoricalView) OpenContent(ctx context.Context, id domain.FileID) (domain.File, io.ReadSeekCloser, error) {
	file, found, err := v.File(ctx, id)
	if err != nil {
		return file, nil, err
	}
	if !found {
		return file, nil, ErrNoSuchFile
	}
	handle, err := v.client.OpenOriginal(ctx, file.BlobKey)
	return file, handle, err
}

// ViewAt uses the verified local catalog boundary, never a transient overlay ID.
// Replay happens in a private database without holding the editing lock.
func (c *Client) ViewAt(ctx context.Context, target int64, fn func(*ClientHistoricalView) error) error {
	if target <= 0 || fn == nil {
		return ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if ctx.Value(clientViewContextKey{}) != nil || ctx.Value(clientRequestContextKey{}) != nil {
		return ErrRequestScope
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	head, err := c.catalogStatusLocked()
	zone, zoneErr := c.readZone()
	release()
	if err != nil {
		return err
	}
	if zoneErr != nil {
		return zoneErr
	}
	if target > head.Confirmed.OpID {
		return ErrNoSuchOp
	}
	dir, err := os.MkdirTemp(c.root, ".client-history-")
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
	reached := errors.New("client history cutoff reached")
	var last int64
	err = c.catalog.Iter(func(line journal.Line) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID != last+1 {
			return fmt.Errorf("non-contiguous client history at op %d: %w", line.ID, ErrInvariantViolated)
		}
		row, err := rowFromLine(line)
		if err != nil {
			return err
		}
		if _, err := applyOp(tx, row); err != nil {
			return err
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
		return fmt.Errorf("client history cutoff %d missing: %w", target, ErrInvariantViolated)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	readTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer readTx.Rollback()
	scope := &clientViewScope{active: true, client: c, tx: store.NewViewTx(readTx, time.Time{})}
	defer func() { scope.mu.Lock(); scope.active = false; scope.mu.Unlock() }()
	if err := fn(&ClientHistoricalView{ClientView: &ClientView{client: c, scope: scope, zone: zone}}); err != nil {
		return err
	}
	return ctx.Err()
}
