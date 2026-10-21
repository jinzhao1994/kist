package library

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

type cacheWorkspaceRecord struct {
	Version   int       `json:"version"`
	ID        string    `json:"id"`
	Budget    int64     `json:"budget"`
	CreatedAt time.Time `json:"created_at"`
}

// cacheWorkspace reserves disposable conversion scratch space. Its OS lease
// protects both the directory and reservation across independent processes.
type cacheWorkspace struct {
	client   *Client
	record   cacheWorkspaceRecord
	release  func()
	once     sync.Once
	closeErr error
}

func (c *Client) workspaceRoot() string { return filepath.Join(c.root, "cache", "workspaces") }
func (c *Client) workspaceLock(id string) *lockfile.Locker {
	return lockfile.New(filepath.Join(c.root, "cache", "locks", "workspace-"+id+".lock"))
}
func (c *Client) workspaceUseLock(id string) *lockfile.Locker {
	return lockfile.NewShared(filepath.Join(c.root, "cache", "locks", "workspace-"+id+".lock"))
}
func (w *cacheWorkspace) path() string { return filepath.Join(w.client.workspaceRoot(), w.record.ID) }

// newCacheWorkspace admits the full reservation before a converter can write.
// Callers must close it after stopping and joining every writer.
func (c *Client) newCacheWorkspace(ctx context.Context, budget int64) (*cacheWorkspace, error) {
	if budget <= 0 {
		return nil, ErrInvalidRequest
	}
	initialize, err := c.cacheLock(ctx)
	if err != nil {
		return nil, err
	}
	initialize()
	record := cacheWorkspaceRecord{Version: 1, ID: domain.NewID().String(), Budget: budget, CreatedAt: time.Now().UTC()}
	lease, err := c.workspaceUseLock(record.ID).Acquire(ctx)
	if err != nil {
		return nil, err
	}
	unlock, err := c.cacheEvictionLock(ctx)
	if err != nil {
		lease()
		return nil, err
	}
	defer unlock()
	if _, err = c.pruneCacheLocked(ctx, budget, ""); err != nil {
		lease()
		return nil, err
	}
	root := c.workspaceRoot()
	if err = os.MkdirAll(root, 0700); err != nil {
		lease()
		return nil, err
	}
	w := &cacheWorkspace{client: c, record: record, release: lease}
	if err = writeClientJSON(filepath.Join(root, record.ID+".json"), record); err == nil {
		err = os.Mkdir(w.path(), 0700)
	}
	if err == nil {
		err = syncClientDir(root)
	}
	if err != nil {
		// Leave any incomplete disposable creation for the next cache scan.
		lease()
		return nil, err
	}
	return w, nil
}

func (w *cacheWorkspace) Close() error {
	// Cleanup is not cancelled along with rendering. It never touches an original.
	unlock, err := w.client.cacheLock(context.Background())
	if err != nil {
		w.once.Do(func() { w.closeErr = err; w.release() })
		return w.closeErr
	}
	defer unlock()
	return w.closeLocked()
}

func (w *cacheWorkspace) closeLocked() error {
	w.once.Do(func() {
		w.release()
		release, acquired, err := w.client.workspaceLock(w.record.ID).TryAcquire()
		if err != nil {
			w.closeErr = err
			return
		}
		if !acquired {
			return
		} // A supervised converter still owns the directory.
		defer release()
		w.closeErr = w.client.removeWorkspaceLocked(w.record.ID)
	})
	return w.closeErr
}

func (c *Client) removeWorkspaceLocked(id string) error {
	if err := os.RemoveAll(filepath.Join(c.workspaceRoot(), id)); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(c.workspaceRoot(), id+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncClientDir(c.workspaceRoot())
}

// workspaceBytes counts the entire tree, including external converter scratch.
// Symlinks are counted without following them outside the managed directory.
func workspaceBytes(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
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
		if info.Size() < 0 || info.Size() > math.MaxInt64-size {
			return ErrCacheMetadata
		}
		size += info.Size()
		return nil
	})
	return size, err
}

// workspaceEntriesLocked reaps only workspaces whose exclusive kernel lease
// can be acquired. No PID guesses, elapsed-time heuristics or blocking lock
// acquisition may delete a converter's active files.
func (c *Client) workspaceEntriesLocked() ([]cacheEntry, error) {
	root := c.workspaceRoot()
	files, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, ".client-") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		parsed, err := domain.ParseID(id)
		if err != nil || parsed.String() != id {
			return nil, ErrCacheMetadata
		}
		if !file.IsDir() && name != id+".json" {
			return nil, ErrCacheMetadata
		}
		ids[id] = true
	}
	entries := []cacheEntry{}
	for id := range ids {
		release, acquired, err := c.workspaceLock(id).TryAcquire()
		if err != nil {
			return nil, err
		}
		if acquired {
			err = c.removeWorkspaceLocked(id)
			release()
			if err != nil {
				return nil, err
			}
			continue
		}
		var record cacheWorkspaceRecord
		if err = readClientJSON(filepath.Join(root, id+".json"), &record); err != nil {
			return nil, ErrCacheMetadata
		}
		if record.Version != 1 {
			return nil, ErrSyncVersion
		}
		if record.ID != id || record.Budget <= 0 || record.CreatedAt.IsZero() {
			return nil, ErrCacheMetadata
		}
		size, err := workspaceBytes(filepath.Join(root, id))
		if err != nil {
			return nil, err
		}
		entries = append(entries, cacheEntry{cachedOriginal: cachedOriginal{Key: "workspace:" + id, Size: record.Budget, AccessedAt: record.CreatedAt}, bytes: size, exists: true, workspace: true})
	}
	return entries, nil
}

// run bounds the complete scratch tree while a converter is alive and again
// before publication. As with existing external media tools, cancellation is
// sampled; a short-lived overshoot is measured, never excluded from accounting.
func (w *cacheWorkspace) run(ctx context.Context, render func(context.Context) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	watched := make(chan struct{})
	var watchErr error
	check := func() error {
		size, err := workspaceBytes(w.path())
		if err != nil {
			return err
		}
		if size > w.record.Budget {
			return ErrCacheQuota
		}
		return nil
	}
	go func() {
		defer close(watched)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := check(); err != nil {
					watchErr = err
					cancel()
					return
				}
			}
		}
	}()
	err := render(runCtx)
	close(done)
	cancel()
	<-watched
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if watchErr != nil {
		return watchErr
	}
	if checkErr := check(); checkErr != nil {
		return checkErr
	}
	return err
}
