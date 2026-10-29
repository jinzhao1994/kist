package library

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/jinzhao1994/kist/internal/blob"
)

// OpenOriginal serves known content from protected local originals first, then
// from verified cache. It never contacts the NAS. Corrupt local originals are
// errors, not cache misses, even if another copy could be downloaded.
func (c *Client) OpenOriginal(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	size, err := c.originalSize(ctx, key)
	if err != nil {
		return nil, err
	}
	parsed, err := blob.ParseKey(key)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	initialize, err := c.cacheLock(ctx)
	if err != nil {
		return nil, err
	}
	initialize()
	release, err := c.cacheObjectLock(key, true).Acquire(ctx)
	if err != nil {
		return nil, err
	}
	reader, err := c.blobs.Get(ctx, parsed)
	if err == nil {
		if err := verifyCachedFile(ctx, reader, key, size); err != nil {
			reader.Close()
			release()
			return nil, err
		}
		return &localOriginalReader{ReadSeekCloser: reader, release: release}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		release()
		return nil, err
	}
	reader, err = c.openCachedLeased(ctx, key, release)
	if err != nil {
		release()
	}
	return reader, err
}

type ClientOfflineStatus struct {
	Previews          []ClientOfflinePreview  `json:"previews"`
	PreviewsAvailable int                     `json:"previews_available"`
	Catalog           ClientCatalogStatus     `json:"catalog"`
	Originals         []ClientOfflineOriginal `json:"originals"`
	Available         int                     `json:"available"`
	Missing           []string                `json:"missing"`
	Ready             bool                    `json:"ready"`
}

// offlineSelectionSnapshot refreshes retained content and captures its catalog
// boundary under the editing lock. No network I/O occurs under either lock.
func (c *Client) offlineSelectionSnapshot(ctx context.Context, owner string) (ClientOfflineStatus, error) {
	var status ClientOfflineStatus
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return status, err
	}
	defer release()
	if _, err := c.rebuildLocked(ctx); err != nil {
		return status, err
	}
	unlock, err := c.cacheLock(ctx)
	if err != nil {
		return status, err
	}
	defer unlock()
	pins, _, err := c.cachePinsLocked()
	if err != nil {
		return status, err
	}
	selection, ok := pins.Selections[owner]
	if !ok {
		return status, ErrInvalidRequest
	}
	status.Originals, err = c.offlineOriginalsLocked(ctx, selection)
	if err != nil {
		return status, err
	}
	status.Previews, err = c.offlinePreviewsLocked(ctx, status.Originals)
	if err != nil {
		return status, err
	}
	status.Catalog, err = c.catalogStatusLocked()
	if err != nil {
		return status, err
	}
	keys := make([]string, 0, len(status.Originals))
	for _, original := range status.Originals {
		keys = append(keys, original.Key)
	}
	pins.Owners[owner] = keys
	if err := c.writeCachePins(pins); err != nil {
		return status, err
	}
	status.Missing = []string{}
	return status, nil
}

// OfflineStatus verifies selected originals and required previews. Ready describes
// the captured local view; it does not assert that an unreachable NAS has no
// newer edits. Missing bytes and integrity failures remain distinct.
func (c *Client) OfflineStatus(ctx context.Context, owner string) (ClientOfflineStatus, error) {
	status, err := c.offlineSelectionSnapshot(ctx, owner)
	if err != nil {
		return status, err
	}
	for _, original := range status.Originals {
		reader, err := c.OpenOriginal(ctx, original.Key)
		if errors.Is(err, ErrNotCached) {
			status.Missing = append(status.Missing, original.Key)
			continue
		}
		if err != nil {
			return status, err
		}
		if err := reader.Close(); err != nil {
			return status, err
		}
		status.Available++
	}
	for i, preview := range status.Previews {
		_, reader, err := c.OpenDerived(ctx, preview.Key, preview.Recipe)
		if errors.Is(err, ErrNotCached) {
			continue
		}
		if err != nil {
			return status, err
		}
		if err := reader.Close(); err != nil {
			return status, err
		}
		status.Previews[i].Available = true
		status.PreviewsAvailable++
	}
	status.Ready = len(status.Originals) > 0 && len(status.Missing) == 0 && status.PreviewsAvailable == len(status.Previews)
	return status, nil
}

// PrepareOffline prepares originals and previews, retaining partial progress on
// failure. It rechecks the current selection after network I/O so changes made
// during a download cannot inherit an obsolete readiness result.
func (c *Client) PrepareOffline(ctx context.Context, owner string, source OriginalSource, renderers ...ClientRenderer) (ClientOfflineStatus, error) {
	status, err := c.offlineSelectionSnapshot(ctx, owner)
	if err != nil {
		return status, err
	}
	for _, original := range status.Originals {
		reader, err := c.OpenOriginal(ctx, original.Key)
		if err == nil {
			if err := reader.Close(); err != nil {
				return status, err
			}
			continue
		}
		if !errors.Is(err, ErrNotCached) {
			return status, err
		}
		if source == nil {
			return status, ErrNotCached
		}
		if err := c.CacheOriginal(ctx, original.Key, source); err != nil {
			return status, err
		}
	}
	var renderer ClientRenderer
	if len(renderers) > 0 {
		renderer = renderers[0]
	}
	derived, _ := source.(DerivedSource)
	for _, preview := range status.Previews {
		if err := c.prepareOfflinePreview(ctx, preview, derived, renderer); err != nil {
			return status, err
		}
	}
	return c.OfflineStatus(ctx, owner)
}

// localOriginalReader shares the cache object's lease across ownership changes.
type localOriginalReader struct {
	io.ReadSeekCloser
	release func()
	once    sync.Once
	err     error
}

func (r *localOriginalReader) Close() error {
	r.once.Do(func() { r.err = r.ReadSeekCloser.Close(); r.release() })
	return r.err
}
