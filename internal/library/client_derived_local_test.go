package library

import (
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

func localPhotoFixture(t *testing.T) (*Client, domain.File) {
	t.Helper()
	c := cacheTestClient(t)
	ctx := t.Context()
	box, err := c.CreateBox(ctx, "offline photos", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "photo.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 32, 16)))
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Import(ctx, path, ClientImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	return c, files[0]
}

func TestClientLocalPreviewQueueWithoutNAS(t *testing.T) {
	c, f := localPhotoFixture(t)
	ctx := t.Context()
	before, _, err := c.LocalHistory(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	storage, err := c.ProtectedStorage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := c.Media(func(context.Context) (DerivedSource, error) {
		t.Error("local conversion contacted NAS")
		return nil, io.ErrClosedPipe
	}, c.RenderWorkspace)
	if _, err = m.Request(ctx, f.ID, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err = m.transfer(ctx, claimClientMedia(t, m, f.ID)); err != nil {
		t.Fatal(err)
	}
	j, r, err := m.Open(ctx, f.ID, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if j.State != "ready" || j.Result.Engine != "go-imaging-1.6.2" {
		t.Fatal(j)
	}
	config, format, err := image.DecodeConfig(r)
	r.Close()
	if err != nil || format != "jpeg" || config.Width != 32 || config.Height != 16 {
		t.Fatal(config, format, err)
	}
	usage, err := c.CacheStatus(ctx)
	if err != nil || usage.CachedBytes != j.Result.Bytes || usage.PartialBytes != 0 || usage.ReservedBytes != 0 {
		t.Fatal(usage, err)
	}
	after, _, err := c.LocalHistory(ctx, "", 100)
	if err != nil || len(after) != len(before) {
		t.Fatal("preview appended intent", after, err)
	}
	protected, err := c.ProtectedStorage(ctx)
	if err != nil || protected != storage {
		t.Fatal("preview changed protected storage", protected, storage, err)
	}
	// The confirmed catalog has never seen this pending photo. Cache index
	// recovery still obtains the output's size/hash from its disposable mapping.
	d, err := c.derivedRecord(f.BlobKey, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.deleteCacheObject(d.ContentKey); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := c.RebuildCache(ctx)
	if err != nil || rebuilt.Verified != 1 {
		t.Fatal(rebuilt, err)
	}
	if _, err = c.SetCacheLimits(ctx, time.Hour, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err = m.Open(ctx, f.ID, derive.Thumbnail); !errors.Is(err, ErrNotCached) {
		t.Fatal(err)
	}
	original, err := c.OpenOriginal(ctx, f.BlobKey)
	if err != nil {
		t.Fatal("preview eviction lost pending photo", err)
	}
	original.Close()
}

func TestClientLocalPreviewPublicationQuotaAndDedup(t *testing.T) {
	c, f := localPhotoFixture(t)
	ctx := t.Context()
	if _, err := c.SetCacheLimits(ctx, time.Hour, 64); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GenerateDerived(ctx, f.BlobKey, derive.Thumbnail, c.RenderWorkspace); !errors.Is(err, ErrCacheQuota) {
		t.Fatal("oversized preview accepted", err)
	}
	usage, err := c.CacheStatus(ctx)
	if err != nil || usage.CachedBytes != 0 || usage.PartialBytes != 0 || usage.ReservedBytes != 0 {
		t.Fatal(usage, err)
	}
	if _, err = c.SetCacheLimits(ctx, time.Hour, 10<<20); err != nil {
		t.Fatal(err)
	}
	first, err := c.GenerateDerived(ctx, f.BlobKey, derive.Thumbnail, c.RenderWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.GenerateDerived(ctx, f.BlobKey, derive.Preview, c.RenderWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentKey != second.ContentKey {
		t.Fatal("small image output did not deduplicate", first, second)
	}
	usage, err = c.CacheStatus(ctx)
	if err != nil || usage.CachedBytes != first.Job.Result.Bytes || usage.ReservedBytes != 0 {
		t.Fatal(usage, err)
	}
	if _, err = c.GenerateDerived(ctx, f.BlobKey, derive.Preview, func(context.Context, ClientRenderRequest) (derive.Result, error) {
		t.Fatal("cached preview rerendered")
		return derive.Result{}, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestClientWorkspaceWorkerLeaseSurvivesParentCleanup(t *testing.T) {
	c := cacheTestClient(t)
	ctx := t.Context()
	w, err := c.newCacheWorkspace(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	child, err := c.workspaceUseLock(w.record.ID).Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = c.PruneCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(w.path()); err != nil {
		t.Fatal("parent cleanup removed active worker directory", err)
	}
	child()
	if _, err = c.PruneCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(w.path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("abandoned worker not reclaimed", err)
	}
}

func TestClientLocalPreviewSameHashAndInvalidResult(t *testing.T) {
	c, f := localPhotoFixture(t)
	ctx := t.Context()
	copyRenderer := func(ctx context.Context, request ClientRenderRequest) (derive.Result, error) {
		reader, err := c.OpenOriginal(ctx, request.Original)
		if err != nil {
			return derive.Result{}, err
		}
		defer reader.Close()
		output, err := os.Create(filepath.Join(c.workspaceRoot(), request.WorkspaceID, "output"))
		if err != nil {
			return derive.Result{}, err
		}
		size, err := io.Copy(output, reader)
		closeErr := output.Close()
		if err != nil {
			return derive.Result{}, err
		}
		if closeErr != nil {
			return derive.Result{}, closeErr
		}
		return derive.Result{Bytes: size, Engine: "test-copy"}, nil
	}
	invalid := func(ctx context.Context, request ClientRenderRequest) (derive.Result, error) {
		result, err := copyRenderer(ctx, request)
		result.Bytes++
		return result, err
	}
	if _, err := c.GenerateDerived(ctx, f.BlobKey, derive.Thumbnail, invalid); err == nil {
		t.Fatal("unverified result was published")
	}
	usage, err := c.CacheStatus(ctx)
	if err != nil || usage.CachedBytes != 0 || usage.PartialBytes != 0 || usage.ReservedBytes != 0 {
		t.Fatal(usage, err)
	}
	// Use a short deadline to catch trying to upgrade our own original reader
	// lease to an exclusive output lease before closing it.
	deadline, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	d, err := c.GenerateDerived(deadline, f.BlobKey, derive.Thumbnail, copyRenderer)
	if err != nil || d.ContentKey != f.BlobKey {
		t.Fatal(d, err)
	}
}
