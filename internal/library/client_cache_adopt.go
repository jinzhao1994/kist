package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/oplog"
)

// AdoptConfirmedOriginals moves already stored local bytes into the cache. Only
// the confirmed catalog, never an unverified response or overlay, proves NAS
// ownership. Moving adds no disk bytes; pruning applies the configured quota
// afterwards, preserving pins even when they exceed that quota.
func (c *Client) AdoptConfirmedOriginals(ctx context.Context) error {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return ErrRequestScope
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := c.cacheEvictionLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	head, _, err := c.boundary()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	if err := c.catalog.Iter(func(line journal.Line) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID > head.OpID {
			return nil
		}
		h, err := oplog.Lookup(oplog.Action(line.Action))
		if err != nil {
			return err
		}
		targets, err := h.Targets(line.Payload)
		if err != nil {
			return err
		}
		for _, key := range targets.Blobs {
			known[key] = true
		}
		return nil
	}); err != nil {
		return err
	}
	state, err := c.queue.read(ctx)
	if err != nil {
		return err
	}
	for _, entry := range state.entries {
		if !entry.active() {
			continue
		}
		// An unresolved inverse may restore historical content; leave originals
		// protected until that dependency has a confirmed formal result.
		if entry.Proposal.Undo != nil {
			return nil
		}
		h, err := oplog.Lookup(oplog.Action(entry.Proposal.Action))
		if err != nil {
			return err
		}
		targets, err := h.Targets(entry.Proposal.Payload)
		if err != nil {
			return err
		}
		for _, key := range targets.Blobs {
			delete(known, key)
		}
	}
	// Imports hold a shared lease while preparing and rehashing originals.
	// Try only: they may need the editing lock currently held by this operation.
	preparation, acquired, err := lockfile.New(filepath.Join(c.root, "local-content-use.lock")).TryAcquire()
	if err != nil || !acquired {
		return err
	}
	defer preparation()
	// Validate the cache before changing any ownership; unknown occupied bytes
	// must not disappear from its accounting.
	if _, err := c.cacheEntriesLocked(); err != nil {
		return err
	}
	err = c.blobs.Walk(func(key blob.Key, _ int64) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !known[string(key)] {
			return nil
		}
		unlock, acquired, err := c.cacheObjectLock(string(key), false).TryAcquire()
		if err != nil || !acquired {
			return err
		}
		defer unlock()
		// PutOriginal publishes under the same placement lock. Preparation outside
		// it can continue, but cannot race this move of an existing immutable blob.
		placement, err := lockfile.New(filepath.Join(c.root, "local-blobs.lock")).Acquire(ctx)
		if err != nil {
			return err
		}
		defer placement()
		source, _ := c.blobs.LocalPath(key)
		reader, err := c.blobs.Get(ctx, key)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		var size int64
		err = c.db.QueryRowContext(ctx, `SELECT size FROM blobs WHERE key=?`, string(key)).Scan(&size)
		if err == nil {
			err = verifyCachedFile(ctx, reader, string(key), size)
		}
		closeErr := reader.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		record := cachedOriginal{Version: 1, Key: string(key), Size: size, Offset: size, AccessedAt: time.Now().UTC()}
		// The source was already fsynced, so its full offset is durable.
		// A crash between the rename and completion leaves recoverable cache bytes,
		// never a false ready claim. The NAS retains the authoritative original.
		if err := c.writeCacheObject(record); err != nil {
			return err
		}
		destination := c.cachePath(string(key), ".data")
		if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		// Cache files must be writable so later pruning also works on Windows.
		if err := os.Chmod(source, 0600); err != nil {
			return err
		}
		if err := os.Rename(source, destination); err != nil {
			return err
		}
		if err := syncClientDir(filepath.Dir(source)); err != nil {
			return err
		}
		if err := syncClientDir(filepath.Dir(destination)); err != nil {
			return err
		}
		record.Offset, record.Complete = size, true
		return c.writeCacheObject(record)
	})
	if err != nil {
		return err
	}
	_, err = c.pruneCacheLocked(ctx, 0, "")
	return err
}

func (c *Client) localPreparationLease(ctx context.Context) (func(), error) {
	return lockfile.NewShared(filepath.Join(c.root, "local-content-use.lock")).Acquire(ctx)
}
