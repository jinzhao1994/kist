package library

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

var (
	ErrNotCached     = errors.New("original is not cached on this client")
	ErrCacheQuota    = errors.New("cache quota is exhausted by protected or active content")
	ErrCacheMetadata = errors.New("client cache metadata needs rebuilding")
)

type cachedOriginal struct {
	Version    int       `json:"version"`
	Key        string    `json:"key"`
	Size       int64     `json:"size"`
	Offset     int64     `json:"offset"`
	Complete   bool      `json:"complete"`
	Corrupt    bool      `json:"corrupt,omitempty"`
	AccessedAt time.Time `json:"accessed_at"`
}

func (r cachedOriginal) valid(key string) bool {
	return r.Key == key && r.Size >= 0 && r.Offset >= 0 && r.Offset <= r.Size && (!r.Complete || r.Offset == r.Size && !r.Corrupt) && !r.AccessedAt.IsZero()
}

type cachePins struct {
	Version int `json:"version"`
	// Owners allow two offline selections to retain the same original without
	// one selection's removal silently unpinning the other's content.
	Owners     map[string][]string               `json:"owners"`
	Selections map[string]ClientOfflineSelection `json:"selections,omitempty"`
}

type cacheEntry struct {
	cachedOriginal
	bytes  int64
	exists bool
}

func (e cacheEntry) budget() int64 {
	if e.Complete {
		return e.bytes
	}
	return max(e.bytes, e.Size)
}

type ClientCacheStatus struct {
	CachedBytes   int64 `json:"cached_bytes"`
	PartialBytes  int64 `json:"partial_bytes"`
	ReservedBytes int64 `json:"reserved_bytes"`
	PinnedBytes   int64 `json:"pinned_bytes"`
	CorruptBytes  int64 `json:"corrupt_bytes"`
	LimitBytes    int64 `json:"limit_bytes"`
	OverLimit     bool  `json:"over_limit"`
}

func (c *Client) cachePath(key, extension string) string {
	return filepath.Join(c.root, "cache", "objects", strings.TrimPrefix(key, "sha256:")+extension)
}

func (c *Client) cacheObjectLock(key string, shared bool) *lockfile.Locker {
	path := filepath.Join(c.root, "cache", "locks", strings.TrimPrefix(key, "sha256:")+".lock")
	if shared {
		return lockfile.NewShared(path)
	}
	return lockfile.New(path)
}

func (c *Client) cacheLock(ctx context.Context) (func(), error) {
	for _, name := range []string{"objects", "locks"} {
		if err := os.MkdirAll(filepath.Join(c.root, "cache", name), 0700); err != nil {
			return nil, err
		}
	}
	return lockfile.New(filepath.Join(c.root, "cache.lock")).Acquire(ctx)
}

func (c *Client) cachePinsLocked() (cachePins, map[string]bool, error) {
	pins := cachePins{Version: 1, Owners: map[string][]string{}}
	err := readClientJSON(filepath.Join(c.root, "cache-pins.json"), &pins)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return pins, nil, err
	}
	if pins.Version != 1 {
		return pins, nil, ErrSyncVersion
	}
	if pins.Owners == nil {
		pins.Owners = map[string][]string{}
	}
	keys := map[string]bool{}
	for owner, selection := range pins.Selections {
		if _, ok := pins.Owners[owner]; !ok || len(selection.Files)+len(selection.Filesets) == 0 {
			return pins, nil, ErrCacheMetadata
		}
		for _, id := range selection.Files {
			if id.IsZero() {
				return pins, nil, ErrCacheMetadata
			}
		}
		for _, id := range selection.Filesets {
			if id.IsZero() {
				return pins, nil, ErrCacheMetadata
			}
		}
	}
	for owner, members := range pins.Owners {
		if owner == "" {
			return pins, nil, ErrCacheMetadata
		}
		for _, key := range members {
			if _, err := blob.ParseKey(key); err != nil {
				return pins, nil, ErrCacheMetadata
			}
			keys[key] = true
		}
	}
	records, _, err := c.derivedRecordsLocked()
	if err != nil {
		return pins, nil, err
	}
	for _, record := range records {
		if keys[record.Job.BlobKey] && record.Job.State == "ready" {
			keys[record.ContentKey] = true
		}
	}
	return pins, keys, nil
}

// SetOfflineOriginals atomically replaces one selection's retained originals.
// An empty list removes that selection. Pinning is intent, not a claim that all
// selected content has already been downloaded.
func (c *Client) SetOfflineOriginals(ctx context.Context, owner string, keys []string) error {
	if owner == "" || len(owner) > 4096 {
		return ErrInvalidRequest
	}
	keys = slices.Clone(keys)
	for _, key := range keys {
		if _, err := blob.ParseKey(key); err != nil {
			return ErrInvalidRequest
		}
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	release, err := c.cacheLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	pins, _, err := c.cachePinsLocked()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		delete(pins.Owners, owner)
	} else {
		pins.Owners[owner] = keys
	}
	delete(pins.Selections, owner)
	return writeClientJSON(filepath.Join(c.root, "cache-pins.json"), pins)
}

func (c *Client) cacheEntriesLocked() ([]cacheEntry, error) {
	files, err := os.ReadDir(filepath.Join(c.root, "cache", "objects"))
	if err != nil {
		return nil, err
	}
	entries := []cacheEntry{}
	known := map[string]bool{}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		key := "sha256:" + strings.TrimSuffix(file.Name(), ".json")
		if _, err := blob.ParseKey(key); err != nil {
			return nil, ErrCacheMetadata
		}
		var record cachedOriginal
		if err := readClientJSON(c.cachePath(key, ".json"), &record); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCacheMetadata, err)
		}
		if record.Version != 1 {
			return nil, ErrSyncVersion
		}
		if !record.valid(key) {
			return nil, ErrCacheMetadata
		}
		entry := cacheEntry{cachedOriginal: record}
		info, err := os.Stat(c.cachePath(key, ".data"))
		if err == nil {
			if !info.Mode().IsRegular() {
				return nil, ErrCacheMetadata
			}
			entry.exists, entry.bytes = true, info.Size()
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		known[key] = true
		entries = append(entries, entry)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".data") && !known["sha256:"+strings.TrimSuffix(file.Name(), ".data")] {
			return nil, ErrCacheMetadata
		}
	}
	return entries, nil
}

func cacheUsage(entries []cacheEntry, pins map[string]bool, limit int64) (ClientCacheStatus, error) {
	status := ClientCacheStatus{LimitBytes: limit}
	var budget int64
	for _, entry := range entries {
		cost := entry.budget()
		if cost > math.MaxInt64-budget {
			return status, ErrCacheMetadata
		}
		budget += cost
		if entry.Complete {
			status.CachedBytes += entry.bytes
		} else {
			status.PartialBytes += entry.bytes
			status.ReservedBytes += cost - entry.bytes
		}
		if pins[entry.Key] {
			status.PinnedBytes += entry.bytes
		}
		if entry.Corrupt {
			status.CorruptBytes += entry.bytes
		}
	}
	status.OverLimit = budget > limit
	return status, nil
}

func (c *Client) CacheStatus(ctx context.Context) (ClientCacheStatus, error) {
	release, err := c.cacheLock(ctx)
	if err != nil {
		return ClientCacheStatus{}, err
	}
	defer release()
	settings, err := c.settings()
	if err != nil {
		return ClientCacheStatus{}, err
	}
	_, pins, err := c.cachePinsLocked()
	if err != nil {
		return ClientCacheStatus{}, err
	}
	entries, err := c.cacheEntriesLocked()
	if err != nil {
		return ClientCacheStatus{}, err
	}
	return cacheUsage(entries, pins, settings.CacheMaxBytes)
}

// PruneCache never enters local-blobs or pending. Reader and transfer leases
// are kernel locks, so eviction skips live users and survives process death.
func (c *Client) PruneCache(ctx context.Context) (ClientCacheStatus, error) {
	release, err := c.cacheEvictionLock(ctx)
	if err != nil {
		return ClientCacheStatus{}, err
	}
	defer release()
	return c.pruneCacheLocked(ctx, 0, "")
}

// reserve is additional admission budget; skip is the object being resumed.
func (c *Client) pruneCacheLocked(ctx context.Context, reserve int64, skip string) (ClientCacheStatus, error) {
	settings, err := c.settings()
	if err != nil {
		return ClientCacheStatus{}, err
	}
	_, pins, err := c.cachePinsLocked()
	if err != nil {
		return ClientCacheStatus{}, err
	}
	entries, err := c.cacheEntriesLocked()
	if err != nil {
		return ClientCacheStatus{}, err
	}
	status, err := cacheUsage(entries, pins, settings.CacheMaxBytes)
	if err != nil {
		return status, err
	}
	budget := status.CachedBytes + status.PartialBytes + status.ReservedBytes
	age, _ := time.ParseDuration(settings.CacheMaxAge)
	cutoff := time.Now().UTC().Add(-age)
	slices.SortFunc(entries, func(a, b cacheEntry) int {
		if order := a.AccessedAt.Compare(b.AccessedAt); order != 0 {
			return order
		}
		return strings.Compare(a.Key, b.Key)
	})
	retained := make([]cacheEntry, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return status, err
		}
		evict := entry.AccessedAt.Before(cutoff) || reserve > settings.CacheMaxBytes-budget
		if !evict || pins[entry.Key] || entry.Key == skip {
			retained = append(retained, entry)
			continue
		}
		unlock, acquired, err := c.cacheObjectLock(entry.Key, false).TryAcquire()
		if err != nil {
			return status, err
		}
		if !acquired {
			retained = append(retained, entry)
			continue
		}
		err = os.Remove(c.cachePath(entry.Key, ".data"))
		if err == nil || errors.Is(err, os.ErrNotExist) {
			err = os.Remove(c.cachePath(entry.Key, ".json"))
		}
		unlock()
		if err != nil {
			return status, err
		}
		budget -= entry.budget()
	}
	if err := syncClientDir(filepath.Join(c.root, "cache", "objects")); err != nil {
		return status, err
	}
	status, err = cacheUsage(retained, pins, settings.CacheMaxBytes)
	if err != nil {
		return status, err
	}
	if reserve > 0 && reserve > settings.CacheMaxBytes-budget {
		return status, ErrCacheQuota
	}
	return status, nil
}

func (c *Client) SetCacheLimits(ctx context.Context, maxAge time.Duration, maxBytes int64) (ClientCacheStatus, error) {
	if maxAge <= 0 || maxBytes <= 0 {
		return ClientCacheStatus{}, ErrInvalidRequest
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return ClientCacheStatus{}, err
	}
	settings, err := c.settings()
	if err == nil {
		settings.CacheMaxAge, settings.CacheMaxBytes = maxAge.String(), maxBytes
		err = writeClientJSON(filepath.Join(c.root, "client.json"), settings)
	}
	release()
	if err != nil {
		return ClientCacheStatus{}, err
	}
	return c.PruneCache(ctx)
}
