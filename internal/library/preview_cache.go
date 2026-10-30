package library

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

func previewCacheRecipe(recipe string) bool {
	spec, ok := derive.Lookup(recipe)
	return ok && !strings.HasPrefix(spec.MediaType, "video/")
}

func savePreviewCacheTx(tx *sql.Tx, j MediaJob, accessed time.Time) error {
	if !previewCacheRecipe(j.Recipe) || j.State != "ready" || j.Result.Bytes <= 0 {
		return nil
	}
	_, err := tx.Exec(`INSERT INTO preview_cache VALUES(?,?,?,?,?) ON CONFLICT(blob_key,recipe) DO UPDATE SET generation=excluded.generation,bytes=excluded.bytes,accessed_at=excluded.accessed_at`, j.BlobKey, j.Recipe, j.Generation, j.Result.Bytes, accessed.UnixNano())
	return err
}

// adoptPreviewCache indexes existing successful outputs once, without touching
// their job state or history. Unknown legacy recipes remain unmanaged.
func adoptPreviewCache(db interface {
	Import(context.Context, string, func(*sql.Tx) error) error
}) error {
	return db.Import(context.Background(), "preview-cache-index", func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT ` + mediaColumns + ` FROM background_jobs WHERE kind='media' AND state='ready'`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var jobs []MediaJob
		for rows.Next() {
			j, err := scanMediaJob(rows)
			if err != nil {
				return err
			}
			jobs = append(jobs, j)
		}
		if err = rows.Err(); err != nil {
			return err
		}
		rows.Close()
		now := time.Now()
		for _, j := range jobs {
			if err = savePreviewCacheTx(tx, j, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *Media) contentLease(j MediaJob, shared bool) (*lockfile.Locker, error) {
	_, path, err := m.paths(j)
	if err != nil {
		return nil, err
	}
	path = filepath.Join(filepath.Dir(path), j.Recipe+".lock")
	if shared {
		return lockfile.NewShared(path), nil
	}
	return lockfile.New(path), nil
}

// MediaContent protects a completed output from eviction until all consumers,
// including sync transfers, have closed it. It retains os.File's read/seek API.
type MediaContent struct {
	*os.File
	media   *Media
	job     MediaJob
	release func()
	once    sync.Once
	err     error
}

func (f *MediaContent) Close() error {
	f.once.Do(func() {
		f.err = f.File.Close()
		defer f.release()
		if !previewCacheRecipe(f.job.Recipe) {
			return
		}
		unlock, err := f.media.state.Acquire(context.Background())
		if err == nil {
			db, e := f.media.database()
			err = e
			if err == nil {
				_, err = db.DB.Exec(`UPDATE preview_cache SET accessed_at=? WHERE blob_key=? AND recipe=? AND generation=?`, time.Now().UnixNano(), f.job.BlobKey, f.job.Recipe, f.job.Generation)
			}
			unlock()
		}
		f.err = errors.Join(f.err, err)
	})
	return f.err
}

// PrunePreviews removes only known, unleased derived bytes. The background job
// and its attempt history are retained; the ordinary missing-output path will
// regenerate evicted previews when they are requested again.
func (m *Media) PrunePreviews(ctx context.Context) error {
	unlock, err := m.state.Acquire(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return m.prunePreviewsLocked(ctx, 0, nil)
}

func (m *Media) prunePreviewsLocked(ctx context.Context, incoming int64, skip *MediaJob) error {
	cfg, err := config.Load(m.library.layout.ConfigPath())
	if err != nil {
		return err
	}
	age, err := config.ParseCacheRetention(cfg.Previews.CacheMaxAge)
	if err != nil {
		return err
	}
	limit := int64(cfg.Previews.CacheMaxBytes)
	if limit > 0 && incoming > limit {
		return ErrCacheQuota
	}
	db, err := m.database()
	if err != nil {
		return err
	}
	rows, err := db.DB.QueryContext(ctx, `SELECT blob_key,recipe,generation,bytes,accessed_at FROM preview_cache ORDER BY accessed_at,blob_key,recipe`)
	if err != nil {
		return err
	}
	type entry struct {
		job            MediaJob
		size, accessed int64
	}
	var entries []entry
	var total int64
	for rows.Next() {
		var e entry
		if err = rows.Scan(&e.job.BlobKey, &e.job.Recipe, &e.job.Generation, &e.size, &e.accessed); err != nil {
			break
		}
		if !previewCacheRecipe(e.job.Recipe) || skip != nil && e.job.BlobKey == skip.BlobKey && e.job.Recipe == skip.Recipe {
			continue
		}
		if e.size <= 0 || e.size > math.MaxInt64-total {
			err = ErrCacheMetadata
			break
		}
		total += e.size
		entries = append(entries, e)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}
		_, path, err := m.paths(e.job)
		if err != nil {
			return err
		}
		_, statErr := os.Stat(path)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		if errors.Is(statErr, os.ErrNotExist) {
			// No content needs a lease. Its parent directory may also have
			// been removed; publication will index any new output afterward.
			if _, err = db.DB.ExecContext(ctx, `DELETE FROM preview_cache WHERE blob_key=? AND recipe=?`, e.job.BlobKey, e.job.Recipe); err != nil {
				return err
			}
			total -= e.size
			continue
		}
		expired := age > 0 && time.Since(time.Unix(0, e.accessed)) >= age
		over := limit > 0 && (incoming > limit || total > limit-incoming)
		if statErr == nil && !expired && !over {
			continue
		}
		lease, err := m.contentLease(e.job, false)
		if err != nil {
			return err
		}
		release, ok, err := lease.TryAcquire()
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		if err == nil {
			_, err = db.DB.ExecContext(ctx, `DELETE FROM preview_cache WHERE blob_key=? AND recipe=?`, e.job.BlobKey, e.job.Recipe)
		}
		release()
		if err != nil {
			return err
		}
		total -= e.size
	}
	if incoming > 0 && limit > 0 && (incoming > limit || total > limit-incoming) {
		return ErrCacheQuota
	}
	return nil
}

func (m *Media) maintainPreviews(ctx context.Context) {
	for ctx.Err() == nil {
		if err := m.PrunePreviews(ctx); err != nil && ctx.Err() == nil && !lockfile.IsBusy(err) {
			m.library.log.Error("preview cache maintenance", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}
