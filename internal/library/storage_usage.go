package library

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// StorageUsage measures local file lengths, not allocated or shared filesystem
// blocks. Projection copies and upload staging are excluded. A concurrent write
// may change the result during the scan; no archive or cache state is modified.
type StorageUsage struct {
	OriginalsBytes int64 `json:"originals_bytes"`
	MetadataBytes  int64 `json:"metadata_bytes"`
	PreviewsBytes  int64 `json:"previews_bytes"`
}

func (l *Library) StorageUsage(ctx context.Context) (StorageUsage, error) {
	return measureStorage(ctx, l.layout.Root, false, nil)
}

func (c *Client) StorageUsage(ctx context.Context) (StorageUsage, error) {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return StorageUsage{}, ErrRequestScope
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return StorageUsage{}, err
	}
	_, previews, err := c.derivedRecordsLocked()
	release()
	if err != nil {
		return StorageUsage{}, err
	}
	return measureStorage(ctx, c.root, true, previews)
}

func measureStorage(ctx context.Context, root string, client bool, previews map[string]int64) (StorageUsage, error) {
	var result StorageUsage
	// Scanning only these roots avoids following user-facing projection links.
	paths := []string{"blobs", "ops", "derived"}
	if client {
		paths = []string{"local-blobs", "catalog", "pending", "cache/objects", "cache/derived"}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && (name == "kist.toml" || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db-shm")) {
			paths = append(paths, name)
		}
	}
	for _, name := range paths {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(name)), func(path string, entry fs.DirEntry, err error) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if errors.Is(err, os.ErrNotExist) {
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
				return nil
			}
			target := &result.MetadataBytes
			switch name {
			case "blobs", "local-blobs":
				target = &result.OriginalsBytes
			case "derived":
				target = &result.PreviewsBytes
			case "cache/objects":
				ext := filepath.Ext(entry.Name())
				if ext == ".data" || ext == ".part" {
					target = &result.OriginalsBytes
					if _, ok := previews["sha256:"+strings.TrimSuffix(entry.Name(), ext)]; ok {
						target = &result.PreviewsBytes
					}
				}
			}
			if info.Size() < 0 || info.Size() > math.MaxInt64-*target {
				return ErrInvariantViolated
			}
			*target += info.Size()
			return nil
		})
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
