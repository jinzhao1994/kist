package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
)

type OriginalSource interface {
	Capabilities(context.Context) (SyncCapabilities, error)
	OpenOriginal(context.Context, string, int64, int64) (io.ReadCloser, error)
}

func (c *Client) originalSize(ctx context.Context, key string) (int64, error) {
	if _, err := blob.ParseKey(key); err != nil {
		return 0, ErrInvalidRequest
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	if _, err := c.rebuildLocked(ctx); err != nil {
		return 0, err
	}
	var size int64
	err = c.db.QueryRowContext(ctx, `SELECT size FROM blobs WHERE key=?`, key).Scan(&size)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidRequest
	}
	if size < 0 {
		return 0, ErrInvalidRequest
	}
	return size, err
}

func (c *Client) cacheRecord(key string) (cachedOriginal, error) {
	var record cachedOriginal
	err := readClientJSON(c.cachePath(key, ".json"), &record)
	if err != nil {
		return record, err
	}
	if record.Version != 1 {
		return record, ErrSyncVersion
	}
	if !record.valid(key) {
		return record, ErrCacheMetadata
	}
	return record, nil
}

func (c *Client) saveCacheRecord(ctx context.Context, record cachedOriginal) error {
	release, err := c.cacheLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	return writeClientJSON(c.cachePath(record.Key, ".json"), record)
}

func verifyCachedFile(ctx context.Context, file io.ReadSeeker, key string, size int64) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, cacheContextReader{ctx: ctx, reader: file})
	if err != nil {
		return err
	}
	parsed, err := blob.ParseKey(key)
	if err != nil || n != size || hex.EncodeToString(hash.Sum(nil)) != parsed.Hex() {
		return blob.ErrIntegrityMismatch
	}
	_, err = file.Seek(0, io.SeekStart)
	return err
}

// CacheOriginal reserves the complete size before receiving any bytes. The
// exclusive object lease remains held across network I/O; the global cache
// lock is only held while changing metadata or making quota decisions.
func (c *Client) CacheOriginal(ctx context.Context, key string, source OriginalSource) error {
	size, err := c.originalSize(ctx, key)
	if err != nil {
		return err
	}
	initialize, err := c.cacheLock(ctx)
	if err != nil {
		return err
	}
	initialize()
	unlock, err := c.cacheObjectLock(key, false).Acquire(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	release, err := c.cacheEvictionLock(ctx)
	if err != nil {
		return err
	}
	record, err := c.cacheRecord(key)
	var previousBudget int64
	if errors.Is(err, os.ErrNotExist) {
		record = cachedOriginal{Version: 1, Key: key, Size: size, AccessedAt: time.Now().UTC()}
	} else if err != nil {
		release()
		return err
	} else {
		if record.Size != size {
			release()
			return ErrCacheMetadata
		}
		previousBudget = size
		if record.Corrupt {
			// Explicit retry may discard invalid cache bytes before admission;
			// otherwise an oversized damaged file could consume the very quota
			// required to repair itself. Its offline selection remains intact.
			file, openErr := os.OpenFile(c.cachePath(key, ".data"), os.O_RDWR, 0600)
			if openErr != nil && !errors.Is(openErr, os.ErrNotExist) {
				release()
				return openErr
			}
			if file != nil {
				resetErr := file.Truncate(0)
				if resetErr == nil {
					resetErr = file.Sync()
				}
				closeErr := file.Close()
				if resetErr != nil || closeErr != nil {
					release()
					return errors.Join(resetErr, closeErr)
				}
			}
			record.Corrupt, record.Complete, record.Offset = false, false, 0
		}
		if _, statErr := os.Stat(c.cachePath(key, ".data")); errors.Is(statErr, os.ErrNotExist) {
			if record.Complete {
				previousBudget = 0
			}
			record.Complete, record.Offset = false, 0
		} else if statErr != nil {
			release()
			return statErr
		}
	}
	usage, err := c.pruneCacheLocked(ctx, max(int64(0), size-previousBudget), key)
	if err != nil {
		release()
		return err
	}
	if usage.OverLimit && !record.Complete {
		release()
		return ErrCacheQuota
	}
	record.AccessedAt = time.Now().UTC()
	err = writeClientJSON(c.cachePath(key, ".json"), record)
	release()
	if err != nil {
		return err
	}
	file, err := os.OpenFile(c.cachePath(key, ".data"), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if record.Complete {
		return verifyCachedFile(ctx, file, key, size)
	}
	if info.Size() < record.Offset {
		return blob.ErrIntegrityMismatch
	}
	if info.Size() > record.Offset {
		if err := file.Truncate(record.Offset); err != nil {
			return err
		}
		if err := file.Sync(); err != nil {
			return err
		}
	}
	if _, err := file.Seek(record.Offset, io.SeekStart); err != nil {
		return err
	}
	if record.Offset < size {
		caps, err := source.Capabilities(ctx)
		if err != nil {
			return err
		}
		settings, err := c.settings()
		if err != nil {
			return err
		}
		if caps.LibraryID != settings.LibraryID {
			return ErrSyncConflict
		}
		if err := ValidateSyncCapabilities(caps); err != nil {
			return err
		}
		reader, err := source.OpenOriginal(ctx, key, size, record.Offset)
		if err != nil {
			return err
		}
		defer reader.Close()
		buffer := make([]byte, 8<<20)
		for record.Offset < size {
			want := min(int64(len(buffer)), size-record.Offset)
			n, readErr := io.ReadFull(cacheContextReader{ctx, reader}, buffer[:want])
			if n > 0 {
				written, err := file.Write(buffer[:n])
				if err != nil {
					return err
				}
				if written != n {
					return io.ErrShortWrite
				}
				if err := file.Sync(); err != nil {
					return err
				}
				record.Offset += int64(n)
				record.AccessedAt = time.Now().UTC()
				if err := c.saveCacheRecord(ctx, record); err != nil {
					return err
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					return io.ErrUnexpectedEOF
				}
				return readErr
			}
		}
		var extra [1]byte
		if n, err := reader.Read(extra[:]); n != 0 || err != io.EOF {
			if err != nil && err != io.EOF {
				return err
			}
			return blob.ErrIntegrityMismatch
		}
	}
	if err := verifyCachedFile(ctx, file, key, size); err != nil {
		if errors.Is(err, blob.ErrIntegrityMismatch) {
			// Invalid cache bytes are not an original. Reset only this disposable
			// transfer; preserve offline pins and every protected local original.
			if resetErr := file.Truncate(0); resetErr != nil {
				return errors.Join(err, resetErr)
			}
			if resetErr := file.Sync(); resetErr != nil {
				return errors.Join(err, resetErr)
			}
			record.Offset = 0
			return errors.Join(err, c.saveCacheRecord(ctx, record))
		}
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	record.Complete, record.AccessedAt = true, time.Now().UTC()
	return c.saveCacheRecord(ctx, record)
}

// OpenCachedOriginal distinguishes absent/incomplete content from invalid bytes.
// A returned reader holds a shared kernel lease until Close.
func (c *Client) OpenCachedOriginal(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	if _, err := blob.ParseKey(key); err != nil {
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
	reader, err := c.openCachedLeased(ctx, key, release)
	if err != nil {
		release()
	}
	return reader, err
}

func (c *Client) openCachedLeased(ctx context.Context, key string, release func()) (io.ReadSeekCloser, error) {
	unlock, err := c.cacheLock(ctx)
	if err != nil {
		return nil, err
	}
	record, err := c.cacheRecord(key)
	if err == nil && record.Corrupt {
		unlock()
		return nil, blob.ErrIntegrityMismatch
	}
	if errors.Is(err, os.ErrNotExist) || err == nil && !record.Complete {
		unlock()
		return nil, ErrNotCached
	}
	if err != nil {
		unlock()
		return nil, err
	}
	record.AccessedAt = time.Now().UTC()
	err = writeClientJSON(c.cachePath(key, ".json"), record)
	unlock()
	if err != nil {
		return nil, err
	}
	file, err := os.Open(c.cachePath(key, ".data"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotCached
	}
	if err != nil {
		return nil, err
	}
	if err := verifyCachedFile(ctx, file, key, record.Size); err != nil {
		file.Close()
		return nil, err
	}
	return &cachedReader{File: file, client: c, key: key, release: release}, nil
}

type cachedReader struct {
	*os.File
	client  *Client
	key     string
	release func()
	once    sync.Once
	err     error
}

func (r *cachedReader) Close() error {
	r.once.Do(func() {
		defer r.release()
		r.err = r.File.Close()
		unlock, err := r.client.cacheLock(context.Background())
		if err != nil {
			r.err = errors.Join(r.err, err)
			return
		}
		defer unlock()
		record, err := r.client.cacheRecord(r.key)
		if err == nil {
			record.AccessedAt = time.Now().UTC()
			err = writeClientJSON(r.client.cachePath(r.key, ".json"), record)
		}
		r.err = errors.Join(r.err, err)
	})
	return r.err
}

type cacheContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r cacheContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
