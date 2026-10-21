package library

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

type DerivedSource interface {
	Capabilities(context.Context) (SyncCapabilities, error)
	DescribeDerived(context.Context, string, string, bool) (SyncDerived, error)
	OpenDerived(context.Context, SyncDerived, int64) (io.ReadCloser, error)
}

type cachedDerived struct {
	Version    int              `json:"version"`
	Descriptor SyncDerived      `json:"descriptor"`
	Outputs    map[string]int64 `json:"outputs,omitempty"`
}

func (c *Client) derivedPath(original, recipe string) (string, error) {
	if _, err := blob.ParseKey(original); err != nil {
		return "", ErrInvalidRequest
	}
	if _, ok := derive.Lookup(recipe); !ok {
		return "", ErrInvalidRequest
	}
	return filepath.Join(c.root, "cache", "derived", strings.TrimPrefix(original, "sha256:")+"."+recipe+".json"), nil
}

func (c *Client) derivedRecord(original, recipe string) (SyncDerived, error) {
	path, err := c.derivedPath(original, recipe)
	if err != nil {
		return SyncDerived{}, err
	}
	var record cachedDerived
	if err := readClientJSON(path, &record); err != nil {
		return SyncDerived{}, err
	}
	if record.Version != 1 {
		return SyncDerived{}, ErrSyncVersion
	}
	if err := ValidateSyncDerived(record.Descriptor, original, recipe); err != nil {
		return SyncDerived{}, ErrCacheMetadata
	}
	return record.Descriptor, nil
}

// CacheDerived prepares one NAS preview without downloading its original. The
// mapping is disposable; byte storage, reservations, leases and hashes reuse
// the same cache as originals. A non-ready remote job is a successful status.
func (c *Client) CacheDerived(ctx context.Context, original, recipe string, retry bool, source DerivedSource) (SyncDerived, error) {
	var descriptor SyncDerived
	if !retry {
		cached, reader, err := c.OpenDerived(ctx, original, recipe)
		if err == nil {
			return cached, reader.Close()
		}
		if !errors.Is(err, ErrNotCached) {
			return descriptor, err
		}
	}
	if _, err := c.originalSize(ctx, original); err != nil {
		return descriptor, err
	}
	path, err := c.derivedPath(original, recipe)
	if err != nil {
		return descriptor, err
	}
	init, err := c.cacheLock(ctx)
	if err != nil {
		return descriptor, err
	}
	init()
	release, err := lockfile.New(filepath.Join(c.root, "cache", "locks", filepath.Base(path)+".lock")).Acquire(ctx)
	if err != nil {
		return descriptor, err
	}
	defer release()
	caps, err := source.Capabilities(ctx)
	if err != nil {
		return descriptor, err
	}
	settings, err := c.settings()
	if err != nil {
		return descriptor, err
	}
	if caps.LibraryID != settings.LibraryID {
		return descriptor, ErrSyncConflict
	}
	if err := ValidateSyncCapabilities(caps); err != nil {
		return descriptor, err
	}
	if !caps.DerivedPreviews {
		return descriptor, ErrSyncVersion
	}
	descriptor, err = source.DescribeDerived(ctx, original, recipe, retry)
	if err != nil {
		return descriptor, err
	}
	if err := ValidateSyncDerived(descriptor, original, recipe); err != nil {
		return descriptor, err
	}
	unlock, err := c.cacheLock(ctx)
	if err != nil {
		return descriptor, err
	}
	err = os.MkdirAll(filepath.Dir(path), 0700)
	if err == nil {
		outputs := map[string]int64{}
		var old cachedDerived
		readErr := readClientJSON(path, &old)
		if readErr == nil {
			if old.Version != 1 || ValidateSyncDerived(old.Descriptor, original, recipe) != nil {
				err = ErrCacheMetadata
			} else {
				for key, size := range old.Outputs {
					outputs[key] = size
				}
				if old.Descriptor.Job.State == "ready" {
					outputs[old.Descriptor.ContentKey] = old.Descriptor.Job.Result.Bytes
				}
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			err = readErr
		}
		if descriptor.Job.State == "ready" {
			outputs[descriptor.ContentKey] = descriptor.Job.Result.Bytes
		}
		if err == nil {
			stored := descriptor
			// A queued replacement must not hide an already cached compatible preview.
			if descriptor.Job.State != "ready" && readErr == nil && old.Descriptor.Job.State == "ready" {
				stored = old.Descriptor
			}
			err = writeClientJSON(path, cachedDerived{Version: 1, Descriptor: stored, Outputs: outputs})
		}
	}
	unlock()
	if err != nil || descriptor.Job.State != "ready" {
		return descriptor, err
	}
	err = c.cacheContent(ctx, descriptor.ContentKey, descriptor.Job.Result.Bytes, derivedDownload{source: source, descriptor: descriptor})
	return descriptor, err
}

type derivedDownload struct {
	source     DerivedSource
	descriptor SyncDerived
}

func (d derivedDownload) Capabilities(ctx context.Context) (SyncCapabilities, error) {
	return d.source.Capabilities(ctx)
}
func (d derivedDownload) OpenOriginal(ctx context.Context, key string, size, offset int64) (io.ReadCloser, error) {
	if key != d.descriptor.ContentKey || size != d.descriptor.Job.Result.Bytes {
		return nil, ErrSyncConflict
	}
	return d.source.OpenDerived(ctx, d.descriptor, offset)
}

// OpenDerived is strictly local. An absent mapping or evicted/incomplete output
// is not_cached, never a claim that the NAS original was lost.
func (c *Client) OpenDerived(ctx context.Context, original, recipe string) (SyncDerived, io.ReadSeekCloser, error) {
	if _, err := c.originalSize(ctx, original); err != nil {
		return SyncDerived{}, nil, err
	}
	release, err := c.cacheLock(ctx)
	if err != nil {
		return SyncDerived{}, nil, err
	}
	descriptor, err := c.derivedRecord(original, recipe)
	release()
	if errors.Is(err, os.ErrNotExist) {
		return descriptor, nil, ErrNotCached
	}
	if err != nil {
		return descriptor, nil, err
	}
	if descriptor.Job.State != "ready" {
		return descriptor, nil, ErrNotCached
	}
	reader, err := c.OpenCachedOriginal(ctx, descriptor.ContentKey)
	return descriptor, reader, err
}

// derivedRecordsLocked also preserves superseded output facts for accounting
// and recovery until their bytes are evicted. It does not create archive facts.
func (c *Client) derivedRecordsLocked() ([]SyncDerived, map[string]int64, error) {
	root := filepath.Join(c.root, "cache", "derived")
	files, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var records []SyncDerived
	facts := map[string]int64{}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		var record cachedDerived
		if err := readClientJSON(filepath.Join(root, file.Name()), &record); err != nil {
			return nil, nil, ErrCacheMetadata
		}
		descriptor := record.Descriptor
		if record.Version != 1 {
			return nil, nil, ErrSyncVersion
		}
		if err := ValidateSyncDerived(descriptor, descriptor.Job.BlobKey, descriptor.Job.Recipe); err != nil {
			return nil, nil, ErrCacheMetadata
		}
		expected, err := c.derivedPath(descriptor.Job.BlobKey, descriptor.Job.Recipe)
		if err != nil || expected != filepath.Join(root, file.Name()) {
			return nil, nil, ErrCacheMetadata
		}
		for key, size := range record.Outputs {
			if _, err := blob.ParseKey(key); err != nil || size <= 0 || size > derive.MaxOutputBytes {
				return nil, nil, ErrCacheMetadata
			}
			if previous, found := facts[key]; found && previous != size {
				return nil, nil, ErrCacheMetadata
			}
			facts[key] = size
		}
		if descriptor.Job.State == "ready" {
			facts[descriptor.ContentKey] = descriptor.Job.Result.Bytes
		}
		records = append(records, descriptor)
	}
	return records, facts, nil
}
