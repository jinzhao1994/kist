package library

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/store"
)

type ClientCacheRebuildResult struct {
	Verified int `json:"verified"`
	Partial  int `json:"partial"`
	Corrupt  int `json:"corrupt"`
	Missing  int `json:"missing"`
	Skipped  int `json:"skipped"`
}

// RebuildCache reconstructs disposable progress records from cached bytes and
// known original facts. Offline selections and protected local originals are
// never rewritten. Busy objects are explicitly reported, not claimed rebuilt.
func (c *Client) RebuildCache(ctx context.Context) (ClientCacheRebuildResult, error) {
	var result ClientCacheRebuildResult
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return result, err
	}
	_, err = c.rebuildLocked(ctx)
	var facts []store.BlobRow
	if err == nil {
		facts, err = store.AllBlobs(c.db)
	}
	release()
	if err != nil {
		return result, err
	}
	known := map[string]int64{}
	for _, fact := range facts {
		key, err := blob.ParseKey(fact.Key)
		if err != nil || key.Hex() != fact.SHA256 || fact.Size < 0 {
			return result, ErrInvariantViolated
		}
		known[fact.Key] = fact.Size
	}
	release, err = c.cacheLock(ctx)
	if err != nil {
		return result, err
	}
	_, derived, err := c.scanDerivedRecordsLocked(true)
	if err == nil {
		_, _, err = c.cachePinsLocked()
	}
	if err == nil {
		_, err = c.workspaceEntriesLocked()
	}
	if err == nil {
		for key, size := range derived {
			if previous, exists := known[key]; exists && previous != size {
				err = ErrCacheMetadata
				break
			}
			known[key] = size
		}
	}
	var files []os.DirEntry
	if err == nil {
		files, err = os.ReadDir(filepath.Join(c.root, "cache", "objects"))
	}
	release()
	if err != nil {
		return result, err
	}
	records, err := c.cacheObjectRecords()
	if err != nil {
		return result, err
	}
	keys := []string{}
	for _, r := range records {
		keys = append(keys, r.Key)
	}
	for _, file := range files {
		ext := filepath.Ext(file.Name())
		if ext != ".data" {
			continue
		}
		key := "sha256:" + strings.TrimSuffix(file.Name(), ext)
		if _, err := blob.ParseKey(key); err != nil {
			return result, ErrCacheMetadata
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range slices.Compact(keys) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		size, found := known[key]
		if !found {
			// A disposable preview mapping may be lost independently of its
			// bytes. Recover accounting without inventing an original/recipe.
			size = -1
		}
		unlock, acquired, err := c.cacheObjectLock(key, false).TryAcquire()
		if err != nil {
			return result, err
		}
		if !acquired {
			result.Skipped++
			continue
		}
		state, err := c.rebuildCachedOriginal(ctx, key, size)
		unlock()
		if err != nil {
			return result, err
		}
		switch state {
		case "verified":
			result.Verified++
		case "partial":
			result.Partial++
		case "corrupt":
			result.Corrupt++
		case "missing":
			result.Missing++
		}
	}
	return result, nil
}

func (c *Client) rebuildCachedOriginal(ctx context.Context, key string, size int64) (string, error) {
	var old cachedOriginal
	err := func() error { var e error; old, e = c.readCacheObject(key); return e }()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		var syntax *json.SyntaxError
		var value *json.UnmarshalTypeError
		if !errors.As(err, &syntax) && !errors.As(err, &value) {
			return "", err
		}
	}
	if err == nil && old.Version != 0 && old.Version != 1 {
		return "", ErrSyncVersion
	}
	valid := err == nil && old.Version == 1 && old.valid(key)
	if size < 0 && valid {
		size = old.Size
	}
	valid = valid && old.Size == size
	file, err := os.OpenFile(c.cachePath(key, ".data"), os.O_RDWR, 0600)
	if errors.Is(err, os.ErrNotExist) {
		release, err := c.cacheLock(ctx)
		if err != nil {
			return "", err
		}
		defer release()
		if err := c.deleteCacheObject(key); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "missing", syncClientDir(filepath.Join(c.root, "cache", "objects"))
	}
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", ErrCacheMetadata
	}
	length := info.Size()
	if size < 0 {
		// With neither a catalog fact nor a valid index, only a full hash
		// match can prove these bytes complete. Otherwise retain them as
		// corrupt, not resumable, until explicitly fetched again or pruned.
		size = length
	}
	lostConfirmed := valid && length < old.Offset
	if valid && !old.Complete && !old.Corrupt && length > old.Offset {
		if err := file.Truncate(old.Offset); err != nil {
			return "", err
		}
		length = old.Offset
	}
	record := cachedOriginal{Version: 1, Key: key, Size: size, Offset: min(length, size), AccessedAt: time.Now().UTC()}
	if valid {
		record.AccessedAt = old.AccessedAt
	}
	state := "partial"
	if length == size {
		err := verifyCachedFile(ctx, file, key, size)
		if err == nil {
			record.Complete, state = true, "verified"
		} else if errors.Is(err, blob.ErrIntegrityMismatch) {
			record.Corrupt = true
		} else {
			return "", err
		}
	} else if length > size || lostConfirmed || valid && old.Corrupt {
		record.Corrupt = true
	}
	if record.Corrupt {
		record.Offset, state = 0, "corrupt"
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := c.saveCacheRecord(ctx, record); err != nil {
		return "", err
	}
	return state, nil
}
