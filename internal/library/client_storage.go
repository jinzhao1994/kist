package library

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
)

// ClientProtectedStorage describes disk bytes excluded from automatic cache eviction.
// Originals includes prepared and previously acknowledged content still retained
// locally; it must not be presented as only the size of pending operations.
type ClientProtectedStorage struct {
	OriginalsBytes      int64 `json:"originals_bytes"`
	PendingJournalBytes int64 `json:"pending_journal_bytes"`
}

// ProtectedStorage measures retained originals and the durable local journal.
// Imports can still write preparation bytes during the scan, so this is an
// observation, not a reservation or a promise of available filesystem capacity.
func (c *Client) ProtectedStorage(ctx context.Context) (ClientProtectedStorage, error) {
	var result ClientProtectedStorage
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return result, ErrRequestScope
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	result.OriginalsBytes, err = clientDirectoryBytes(ctx, filepath.Join(c.root, "local-blobs"))
	if err != nil {
		return result, err
	}
	result.PendingJournalBytes, err = clientDirectoryBytes(ctx, filepath.Join(c.root, "pending"))
	return result, err
}

func clientDirectoryBytes(ctx context.Context, root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		// An import can atomically publish a temporary file during the observation.
		if errors.Is(err, os.ErrNotExist) && path != root {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return ErrInvariantViolated
		}
		if info.Size() < 0 || info.Size() > math.MaxInt64-total {
			return ErrInvariantViolated
		}
		total += info.Size()
		return nil
	})
	return total, err
}
