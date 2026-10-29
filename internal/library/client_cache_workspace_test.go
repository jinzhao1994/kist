package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientCacheWorkspaceReservationAndLiveLease(t *testing.T) {
	c := cacheTestClient(t)
	ctx := t.Context()
	if _, err := c.SetCacheLimits(ctx, time.Hour, 100); err != nil {
		t.Fatal(err)
	}
	original, err := c.PutOriginal(ctx, strings.NewReader("protected pending bytes"))
	if err != nil {
		t.Fatal(err)
	}
	old := seedCachedOriginal(t, c, strings.Repeat("x", 80), time.Now().UTC())
	w, err := c.newCacheWorkspace(ctx, 60)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err = os.Stat(c.cachePath(old, ".data")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("reservation did not evict LRU", err)
	}
	if err = os.Mkdir(filepath.Join(w.path(), "scratch"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(w.path(), "scratch", "intermediate"), []byte(strings.Repeat("x", 20)), 0600); err != nil {
		t.Fatal(err)
	}
	other, err := OpenClient(ctx, c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	usage, err := other.CacheStatus(ctx)
	if err != nil || usage.PartialBytes != 20 || usage.ReservedBytes != 40 || usage.CachedBytes != 0 {
		t.Fatal(usage, err)
	}
	if _, err = other.newCacheWorkspace(ctx, 41); !errors.Is(err, ErrCacheQuota) {
		t.Fatal("overcommitted reservation", err)
	}
	second, err := other.newCacheWorkspace(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	if err = second.Close(); err != nil {
		t.Fatal(err)
	}
	usage, err = other.SetCacheLimits(ctx, time.Nanosecond, 10)
	if err != nil || !usage.OverLimit || usage.PartialBytes != 20 || usage.ReservedBytes != 40 {
		t.Fatal("live workspace evicted", usage, err)
	}
	if _, err = os.Stat(filepath.Join(w.path(), "scratch", "intermediate")); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	usage, err = other.CacheStatus(ctx)
	if err != nil || usage.PartialBytes != 0 || usage.ReservedBytes != 0 || usage.OverLimit {
		t.Fatal(usage, err)
	}
	if _, err = c.blobs.Stat(ctx, original.Key); err != nil {
		t.Fatal("cleanup touched protected original", err)
	}
}

func TestClientCacheWorkspaceRecoversAbandonedAndDamagedReservations(t *testing.T) {
	c := cacheTestClient(t)
	ctx := t.Context()
	w, err := c.newCacheWorkspace(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(w.path(), "temporary"), []byte("scratch"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := c.cacheDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.Exec(`UPDATE cache_workspaces SET budget=-1 WHERE id=?`, w.record.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = c.CacheStatus(ctx); !errors.Is(err, ErrCacheMetadata) {
		t.Fatal("active damage silently ignored", err)
	}
	// Releasing the kernel lease simulates a dead converter; no elapsed timer or
	// process identity is needed to distinguish its scratch from an active task.
	w.release()
	if _, err = c.RebuildCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(w.path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if _, err = c.readWorkspace(w.record.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	usage, err := c.CacheStatus(ctx)
	if err != nil || usage.ReservedBytes != 0 || usage.PartialBytes != 0 {
		t.Fatal(usage, err)
	}
	// A crash after writing the reservation but before creating its directory is
	// likewise recoverable and cannot hold quota forever.
	w, err = c.newCacheWorkspace(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(w.path()); err != nil {
		t.Fatal(err)
	}
	w.release()
	usage, err = c.PruneCache(ctx)
	if err != nil || usage.ReservedBytes != 0 {
		t.Fatal(usage, err)
	}
}

func TestClientCacheWorkspaceStopsOversizedConversion(t *testing.T) {
	c := cacheTestClient(t)
	w, err := c.newCacheWorkspace(t.Context(), 20)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err = w.run(ctx, func(ctx context.Context) error {
		if err := os.WriteFile(filepath.Join(w.path(), "output"), []byte(strings.Repeat("x", 21)), 0600); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, ErrCacheQuota) {
		t.Fatal(err)
	}
	usage, err := c.CacheStatus(t.Context())
	if err != nil || usage.PartialBytes != 21 || usage.ReservedBytes != 0 {
		t.Fatal("overshoot excluded from usage", usage, err)
	}
	// A fast writer may finish before the periodic sample. Publication must still
	// be refused by the final size check.
	if err = w.run(t.Context(), func(context.Context) error { return nil }); !errors.Is(err, ErrCacheQuota) {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(w.path(), "output")); err != nil {
		t.Fatal(err)
	}
	if err = w.run(t.Context(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	cancelled, stop := context.WithCancel(t.Context())
	stop()
	if err = w.run(cancelled, func(ctx context.Context) error { return ctx.Err() }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
