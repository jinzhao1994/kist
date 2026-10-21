package library

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
)

// publishDerived moves a completed local conversion into the existing shared
// cache without copying it or admitting a second copy outside the reservation.
// All converter processes must have exited, and the input lease must be closed
// (a re-encoded image may have the same hash as its original).
func (c *Client) publishDerived(ctx context.Context, w *cacheWorkspace, original, recipe string, result derive.Result) (SyncDerived, error) {
	d := SyncDerived{Job: MediaJob{BlobKey: original, Recipe: recipe, State: "ready", Generation: rand.Text(), UpdatedAt: time.Now().UTC(), Result: result}}
	path, err := c.derivedPath(original, recipe)
	if err != nil {
		return d, err
	}
	if _, err = c.originalSize(ctx, original); err != nil {
		return d, err
	}
	if w.client != c {
		return d, ErrInvalidRequest
	}
	source := filepath.Join(w.path(), "output")
	info, err := os.Lstat(source)
	if err != nil {
		return d, err
	}
	if !info.Mode().IsRegular() {
		return d, ErrInvalidRequest
	}
	f, err := os.OpenFile(source, os.O_RDWR, 0600)
	if err != nil {
		return d, err
	}
	info, err = f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = ErrInvalidRequest
	}
	if err == nil {
		d.ContentKey, err = derivedDigest(ctx, f, result.Bytes)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return d, err
	}
	if closeErr != nil {
		return d, closeErr
	}
	size, err := workspaceBytes(w.path())
	if err != nil {
		return d, err
	}
	if size > w.record.Budget {
		return d, ErrCacheQuota
	}
	if err = ValidateSyncDerived(d, original, recipe); err != nil {
		return d, err
	}
	release, err := c.cacheObjectLock(d.ContentKey, false).Acquire(ctx)
	if err != nil {
		return d, err
	}
	defer release()
	unlock, err := c.cacheLock(ctx)
	if err != nil {
		return d, err
	}
	defer unlock()
	existing, err := c.cacheRecord(d.ContentKey)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return d, err
	}
	if err == nil && existing.Size != result.Bytes {
		return d, ErrCacheMetadata
	}
	if err = c.saveDerivedLocked(path, d); err != nil {
		return d, err
	}
	record := cachedOriginal{Version: 1, Key: d.ContentKey, Size: result.Bytes, Offset: result.Bytes, AccessedAt: time.Now().UTC()}
	if err = writeClientJSON(c.cachePath(d.ContentKey, ".json"), record); err != nil {
		return d, err
	}
	// Windows does not replace an existing destination on rename. The exclusive
	// output lease excludes readers, transfers and eviction during replacement.
	if err = os.Remove(c.cachePath(d.ContentKey, ".data")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return d, err
	}
	if err = os.Rename(source, c.cachePath(d.ContentKey, ".data")); err != nil {
		return d, err
	}
	if err = syncClientDir(w.path()); err != nil {
		return d, err
	}
	if err = syncClientDir(filepath.Join(c.root, "cache", "objects")); err != nil {
		return d, err
	}
	record.Complete = true
	if err = writeClientJSON(c.cachePath(d.ContentKey, ".json"), record); err != nil {
		return d, err
	}
	// Drop the scratch reservation in the same cache critical section as output
	// installation. Concurrent admission never counts both or neither of them.
	return d, w.closeLocked()
}
