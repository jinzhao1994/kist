package library

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// ClientMedia keeps disposable requests on disk. The provider is resolved for
// each transfer so another process can rotate the connection credentials.
type ClientMedia struct {
	client        *Client
	render        ClientRenderer
	source        func(context.Context) (DerivedSource, error)
	state, worker *lockfile.Locker
}

func (c *Client) Media(source func(context.Context) (DerivedSource, error), renderers ...ClientRenderer) *ClientMedia {
	var renderer ClientRenderer
	if len(renderers) > 0 {
		renderer = renderers[0]
	}
	return &ClientMedia{client: c, source: source, render: renderer, state: lockfile.New(filepath.Join(c.root, "client-media-state.lock")), worker: lockfile.New(filepath.Join(c.root, "client-media-worker.lock"))}
}

func (m *ClientMedia) resolve(ctx context.Context, id domain.FileID, recipe string) (MediaJob, error) {
	if _, ok := derive.Lookup(recipe); !ok {
		return MediaJob{}, ErrInvalidRequest
	}
	f, found, err := m.client.File(ctx, id)
	if err != nil {
		return MediaJob{}, err
	}
	if !found {
		return MediaJob{}, ErrNoSuchFile
	}
	return MediaJob{BlobKey: f.BlobKey, Recipe: recipe, State: "missing"}, nil
}

func (m *ClientMedia) path(j MediaJob) (string, error) {
	path, err := m.client.derivedPath(j.BlobKey, j.Recipe)
	if err != nil {
		return "", err
	}
	return filepath.Join(m.client.root, "cache", "media-jobs", filepath.Base(path)), nil
}

func (m *ClientMedia) read(j MediaJob) (MediaJob, error) {
	path, err := m.path(j)
	if err != nil {
		return j, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return j, nil
	}
	if err != nil {
		return j, err
	}
	defer f.Close()
	var got MediaJob
	// Queue records are disposable and cannot redirect a download. A truncated
	// or mismatched record is requested again without blocking unrelated work.
	err = json.NewDecoder(io.LimitReader(f, 16<<10)).Decode(&got)
	if err != nil || got.BlobKey != j.BlobKey || got.Recipe != j.Recipe || got.Generation == "" || got.UpdatedAt.IsZero() {
		return j, nil
	}
	switch got.State {
	case "queued", "running", "ready", "blocked", "failed", "cancelled":
		return got, nil
	}
	return j, nil
}

func (m *ClientMedia) save(j MediaJob) error {
	path, err := m.path(j)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return writeClientJSON(path, j)
}

// cached checks availability without hashing a potentially large video on each
// status poll. Open still verifies the bytes and holds a lease while serving.
func (m *ClientMedia) cached(ctx context.Context, j MediaJob) (MediaJob, error) {
	unlock, err := m.client.cacheLock(ctx)
	if err != nil {
		return j, err
	}
	defer unlock()
	d, err := m.client.derivedRecord(j.BlobKey, j.Recipe)
	if errors.Is(err, os.ErrNotExist) {
		return j, ErrNotCached
	}
	if err != nil {
		return j, err
	}
	if d.Job.State != "ready" {
		return j, ErrNotCached
	}
	r, err := m.client.cacheRecord(d.ContentKey)
	if errors.Is(err, os.ErrNotExist) {
		return j, ErrNotCached
	}
	if err != nil {
		return j, err
	}
	if r.Corrupt {
		return j, blob.ErrIntegrityMismatch
	}
	if !r.Complete {
		return j, ErrNotCached
	}
	st, err := os.Stat(m.client.cachePath(d.ContentKey, ".data"))
	if errors.Is(err, os.ErrNotExist) {
		return j, ErrNotCached
	}
	if err != nil {
		return j, err
	}
	if st.Size() != r.Size || r.Size != d.Job.Result.Bytes {
		return j, blob.ErrIntegrityMismatch
	}
	return d.Job, nil
}

func (m *ClientMedia) statusLocked(ctx context.Context, j MediaJob) (MediaJob, error) {
	cached, err := m.cached(ctx, j)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, ErrNotCached) {
		return j, err
	}
	got, err := m.read(j)
	if got.State == "ready" {
		return j, err
	} // The output was evicted, not lost on NAS.
	return got, err
}

func (m *ClientMedia) Status(ctx context.Context, id domain.FileID, recipe string) (MediaJob, error) {
	j, err := m.resolve(ctx, id, recipe)
	if err != nil {
		return j, err
	}
	unlock, err := m.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer unlock()
	return m.statusLocked(ctx, j)
}

func (m *ClientMedia) Request(ctx context.Context, id domain.FileID, recipe string, retry bool) (MediaJob, error) {
	j, err := m.resolve(ctx, id, recipe)
	if err != nil {
		return j, err
	}
	return m.requestJob(ctx, j, retry)
}

func (m *ClientMedia) requestJob(ctx context.Context, j MediaJob, retry bool) (MediaJob, error) {
	unlock, err := m.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer unlock()
	j, err = m.statusLocked(ctx, j)
	if err != nil {
		return j, err
	}
	if j.State != "missing" && !(retry && (j.State == "failed" || j.State == "blocked" || j.State == "cancelled")) {
		return j, nil
	}
	j.State = "queued"
	j.Generation = rand.Text()
	j.Reason = ""
	j.UpdatedAt = time.Now().UTC()
	j.Result = derive.Result{}
	return j, m.save(j)
}

func (m *ClientMedia) Cancel(ctx context.Context, id domain.FileID, recipe string) (MediaJob, error) {
	j, err := m.resolve(ctx, id, recipe)
	if err != nil {
		return j, err
	}
	unlock, err := m.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer unlock()
	j, err = m.statusLocked(ctx, j)
	if err != nil {
		return j, err
	}
	if j.State == "queued" || j.State == "running" || j.State == "blocked" {
		j.State = "cancelled"
		j.Generation = rand.Text()
		j.Reason = ""
		j.UpdatedAt = time.Now().UTC()
		err = m.save(j)
	}
	return j, err
}

func (m *ClientMedia) Open(ctx context.Context, id domain.FileID, recipe string) (MediaJob, io.ReadSeekCloser, error) {
	j, err := m.resolve(ctx, id, recipe)
	if err != nil {
		return j, nil, err
	}
	d, r, err := m.client.OpenDerived(ctx, j.BlobKey, j.Recipe)
	if err != nil {
		return j, nil, err
	}
	return d.Job, r, nil
}

// Run serializes workers across processes. A successor resumes running jobs;
// neither startup nor polling contact NAS until a request has been enqueued.
func (m *ClientMedia) Run(ctx context.Context) {
	for ctx.Err() == nil {
		unlock, err := m.worker.Acquire(ctx)
		if err == nil {
			err = m.work(ctx)
			unlock()
		}
		if err != nil && ctx.Err() == nil && !lockfile.IsBusy(err) {
			slog.Error("client media worker", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (m *ClientMedia) claim(ctx context.Context) (MediaJob, bool, error) {
	unlock, err := m.state.Acquire(ctx)
	if err != nil {
		return MediaJob{}, false, err
	}
	defer unlock()
	root := filepath.Join(m.client.root, "cache", "media-jobs")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return MediaJob{}, false, nil
	}
	if err != nil {
		return MediaJob{}, false, err
	}
	var pending []MediaJob
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		parts := strings.SplitN(strings.TrimSuffix(entry.Name(), ".json"), ".", 2)
		if len(parts) != 2 {
			continue
		}
		j := MediaJob{BlobKey: "sha256:" + parts[0], Recipe: parts[1], State: "missing"}
		path, e := m.path(j)
		if e != nil || path != filepath.Join(root, entry.Name()) {
			continue
		}
		j, e = m.read(j)
		if e != nil {
			return j, false, e
		}
		if j.State == "running" || j.State == "queued" && time.Since(j.UpdatedAt) >= time.Second || j.State == "blocked" && j.Reason == "dependency_missing" && time.Since(j.UpdatedAt) >= 30*time.Second {
			pending = append(pending, j)
		}
	}
	if len(pending) == 0 {
		return MediaJob{}, false, nil
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].UpdatedAt.Before(pending[j].UpdatedAt) })
	j := pending[0]
	j.State = "running"
	j.UpdatedAt = time.Now().UTC()
	err = m.save(j)
	return j, err == nil, err
}

func (m *ClientMedia) work(ctx context.Context) error {
	for ctx.Err() == nil {
		j, ok, err := m.claim(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err = m.transfer(ctx, j); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (m *ClientMedia) transfer(ctx context.Context, j MediaJob) error {
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				unlock, err := m.state.Acquire(runCtx)
				if err != nil {
					cancel()
					return
				}
				current, err := m.read(j)
				unlock()
				if err != nil || current.Generation != j.Generation || current.State != "running" {
					cancel()
					return
				}
			}
		}
	}()
	var d SyncDerived
	// Avoid even reading credentials on a usable local cache hit.
	cached, r, err := m.client.OpenDerived(runCtx, j.BlobKey, j.Recipe)
	if err == nil {
		d = cached
		err = r.Close()
	} else if errors.Is(err, ErrNotCached) {
		d, err = m.client.GenerateDerived(runCtx, j.BlobKey, j.Recipe, m.render)
		localErr := err
		var conversionFailure *derive.Failure
		if errors.Is(err, ErrNotCached) || errors.As(err, &conversionFailure) {
			var source DerivedSource
			source, err = m.source(runCtx)
			if err == nil {
				d, err = m.client.CacheDerived(runCtx, j.BlobKey, j.Recipe, true, source)
			}
			if err != nil && conversionFailure != nil {
				err = localErr
			}
		}
	}

	cancel()
	<-done
	if ctx.Err() != nil {
		return ctx.Err()
	}
	unlock, e := m.state.Acquire(ctx)
	if e != nil {
		return e
	}
	defer unlock()
	current, e := m.read(j)
	if e != nil {
		return e
	}
	if current.Generation != j.Generation || current.State != "running" {
		return nil
	}
	j.UpdatedAt = time.Now().UTC()
	if err != nil {
		j.State = "blocked"
		j.Reason = "not_cached"
		switch {
		case errors.Is(err, ErrCacheQuota):
			j.Reason = "cache_quota"
		case errors.Is(err, blob.ErrIntegrityMismatch):
			j.State = "failed"
			j.Reason = "blob_integrity_mismatch"
		case errors.Is(err, ErrCacheMetadata):
			j.State = "failed"
			j.Reason = "cache_metadata"
		case errors.Is(err, ErrSyncVersion):
			j.Reason = "sync_version"
		default:
			var conversion *derive.Failure
			if errors.As(err, &conversion) {
				j.Reason = conversion.Code
				if j.Reason != "dependency_missing" {
					j.State = "failed"
				}
			}
		}
	} else {
		j.State = d.Job.State
		j.Reason = d.Job.Reason
		j.Result = d.Job.Result
		if j.State == "running" {
			j.State = "queued"
		}
	}
	return m.save(j)
}

// DescribeSyncDerived queues a preview by immutable content identity.
func (m *ClientMedia) DescribeSyncDerived(ctx context.Context, key, recipe string, retry bool) (SyncDerived, error) {
	if _, ok := derive.Lookup(recipe); !ok {
		return SyncDerived{}, ErrInvalidRequest
	}
	if _, err := m.client.originalSize(ctx, key); err != nil {
		return SyncDerived{}, err
	}
	job, err := m.requestJob(ctx, MediaJob{BlobKey: key, Recipe: recipe, State: "missing"}, retry)
	if err != nil || job.State != "ready" {
		return SyncDerived{Job: job}, err
	}
	d, r, err := m.client.OpenDerived(ctx, key, recipe)
	if r != nil {
		r.Close()
	}
	return d, err
}
