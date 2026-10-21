package library

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// ClientRenderRequest identifies a leased conversion, never arbitrary output
// paths. It is passed to a supervised worker without connection credentials.
type ClientRenderRequest struct {
	Root        string `json:"root"`
	WorkspaceID string `json:"workspace_id"`
	Original    string `json:"original"`
	Recipe      string `json:"recipe"`
}
type ClientRenderer func(context.Context, ClientRenderRequest) (derive.Result, error)

func localContentPath(reader io.ReadSeekCloser) (string, error) {
	switch r := reader.(type) {
	case *cachedReader:
		return r.Name(), nil
	case *localOriginalReader:
		if f, ok := r.ReadSeekCloser.(*os.File); ok {
			return f.Name(), nil
		}
	}
	return "", ErrInvariantViolated
}

// GenerateDerived uses only locally available originals. The renderer owns
// process supervision; it must wait for all writers before returning.
func (c *Client) GenerateDerived(ctx context.Context, original, recipe string, render ClientRenderer) (SyncDerived, error) {
	if render == nil {
		return SyncDerived{}, ErrNotCached
	}
	if d, r, err := c.OpenDerived(ctx, original, recipe); err == nil {
		return d, r.Close()
	} else if !errors.Is(err, ErrNotCached) {
		return d, err
	}
	path, err := c.derivedPath(original, recipe)
	if err != nil {
		return SyncDerived{}, err
	}
	release, err := lockfile.New(filepath.Join(c.root, "cache", "locks", filepath.Base(path)+".lock")).Acquire(ctx)
	if err != nil {
		return SyncDerived{}, err
	}
	defer release()
	if d, r, err := c.OpenDerived(ctx, original, recipe); err == nil {
		return d, r.Close()
	} else if !errors.Is(err, ErrNotCached) {
		return d, err
	}
	reader, err := c.OpenOriginal(ctx, original)
	if err != nil {
		return SyncDerived{}, err
	}
	defer reader.Close()
	input, err := localContentPath(reader)
	if err != nil {
		return SyncDerived{}, err
	}
	budget, err := derive.WorkspaceBudget(ctx, input, recipe)
	if err != nil {
		return SyncDerived{}, err
	}
	settings, err := c.settings()
	if err != nil {
		return SyncDerived{}, err
	}
	budget = min(budget, settings.CacheMaxBytes)
	w, err := c.newCacheWorkspace(ctx, budget)
	if errors.Is(err, ErrCacheQuota) {
		// The first admission has already pruned eligible LRU entries. Protected
		// originals and live transfers keep their reservation; use only what remains.
		usage, e := c.CacheStatus(ctx)
		if e != nil {
			return SyncDerived{}, e
		}
		available := usage.LimitBytes - usage.CachedBytes - usage.PartialBytes - usage.ReservedBytes
		if available > 0 {
			w, err = c.newCacheWorkspace(ctx, min(budget, available))
		}
	}
	if err != nil {
		return SyncDerived{}, err
	}
	defer w.Close()
	result, err := render(ctx, ClientRenderRequest{Root: c.root, WorkspaceID: w.record.ID, Original: original, Recipe: recipe})
	if err != nil {
		return SyncDerived{}, err
	}
	if err = reader.Close(); err != nil {
		return SyncDerived{}, err
	}
	return c.publishDerived(ctx, w, original, recipe, result)
}

// RenderWorkspace runs inside the supervised conversion process. Its shared
// lease survives loss of the requesting process until rendering has stopped.
func (c *Client) RenderWorkspace(ctx context.Context, request ClientRenderRequest) (derive.Result, error) {
	var result derive.Result
	id, err := domain.ParseID(request.WorkspaceID)
	if err != nil || id.String() != request.WorkspaceID || request.Root != c.root {
		return result, ErrInvalidRequest
	}
	if _, err = c.derivedPath(request.Original, request.Recipe); err != nil {
		return result, err
	}
	lease, err := c.workspaceUseLock(request.WorkspaceID).Acquire(ctx)
	if err != nil {
		return result, err
	}
	defer lease()
	unlock, err := c.cacheLock(ctx)
	if err != nil {
		return result, err
	}
	var record cacheWorkspaceRecord
	err = readClientJSON(filepath.Join(c.workspaceRoot(), request.WorkspaceID+".json"), &record)
	if err == nil && (record.Version != 1 || record.ID != request.WorkspaceID || record.Budget <= 0 || record.CreatedAt.IsZero()) {
		err = ErrCacheMetadata
	}
	unlock()
	if err != nil {
		return result, err
	}
	w := &cacheWorkspace{client: c, record: record}
	info, err := os.Lstat(w.path())
	if err != nil {
		return result, err
	}
	if !info.IsDir() {
		return result, ErrCacheMetadata
	}
	reader, err := c.OpenOriginal(ctx, request.Original)
	if err != nil {
		return result, err
	}
	defer reader.Close()
	input, err := localContentPath(reader)
	if err != nil {
		return result, err
	}
	err = w.run(ctx, func(ctx context.Context) error {
		var e error
		result, e = (derive.Renderer{}).Render(ctx, input, filepath.Join(w.path(), "output"), request.Recipe)
		return e
	})
	return result, err
}
