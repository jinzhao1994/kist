package library

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func seedCachedOriginal(t *testing.T, c *Client, content string, accessed time.Time) string {
	t.Helper()
	release, err := c.cacheLock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	key := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
	if err := os.WriteFile(c.cachePath(key, ".data"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeClientJSON(c.cachePath(key, ".json"), cachedOriginal{Version: 1, Key: key, Size: int64(len(content)), Offset: int64(len(content)), Complete: true, AccessedAt: accessed}); err != nil {
		t.Fatal(err)
	}
	return key
}

func cacheTestClient(t *testing.T) *Client {
	t.Helper()
	return newSyncClient(t, openTestLibrary(t))
}

func TestClientCacheAgeLRUAndPendingProtection(t *testing.T) {
	c := cacheTestClient(t)
	now := time.Now().UTC()
	old := seedCachedOriginal(t, c, "old", now.Add(-3*time.Hour))
	middle := seedCachedOriginal(t, c, "four", now.Add(-2*time.Hour))
	newest := seedCachedOriginal(t, c, "fresh", now.Add(-time.Hour))
	local, err := c.PutOriginal(t.Context(), strings.NewReader("unsynced original"))
	if err != nil {
		t.Fatal(err)
	}
	p := syncBox(t, "cache-pending-protection")
	if err := c.Stage(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	status, err := c.SetCacheLimits(t.Context(), 24*time.Hour, 9)
	if err != nil || status.CachedBytes != 9 || status.OverLimit {
		t.Fatalf("LRU usage=%+v err=%v", status, err)
	}
	if _, err := os.Stat(c.cachePath(old, ".data")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("oldest object was not evicted")
	}
	status, err = c.SetCacheLimits(t.Context(), 90*time.Minute, 100)
	if err != nil || status.CachedBytes != 5 {
		t.Fatalf("age usage=%+v err=%v", status, err)
	}
	if _, err := os.Stat(c.cachePath(middle, ".data")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expired object survived")
	}
	if _, err := os.Stat(c.cachePath(newest, ".data")); err != nil {
		t.Fatal(err)
	}
	f, err := c.blobs.Get(t.Context(), local.Key)
	if err != nil {
		t.Fatal("eviction removed protected local content", err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(data) != "unsynced original" {
		t.Fatal("eviction damaged protected local content")
	}
	state, err := c.queue.read(t.Context())
	if err != nil || len(state.entries) != 1 || state.entries[0].Proposal.ID != p.ID {
		t.Fatal("eviction changed pending history")
	}
}

func TestClientCachePinsAndActiveLeasesCanExceedReducedQuota(t *testing.T) {
	c := cacheTestClient(t)
	key := seedCachedOriginal(t, c, "protected", time.Now().UTC().Add(-48*time.Hour))
	for _, owner := range []string{"first-selection", "second-selection"} {
		if err := c.SetOfflineOriginals(t.Context(), owner, []string{key}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.SetOfflineOriginals(t.Context(), "first-selection", nil); err != nil {
		t.Fatal(err)
	}
	status, err := c.SetCacheLimits(t.Context(), time.Hour, 1)
	if err != nil || !status.OverLimit || status.PinnedBytes != 9 {
		t.Fatalf("pinned quota=%+v err=%v", status, err)
	}
	lease, err := c.cacheObjectLock(key, true).Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetOfflineOriginals(t.Context(), "second-selection", nil); err != nil {
		lease()
		t.Fatal(err)
	}
	other, err := OpenClient(t.Context(), c.root)
	if err != nil {
		lease()
		t.Fatal(err)
	}
	defer other.Close()
	status, err = other.PruneCache(t.Context())
	if err != nil || status.CachedBytes != 9 || !status.OverLimit {
		lease()
		t.Fatalf("evicted active reader: %+v %v", status, err)
	}
	lease()
	status, err = other.PruneCache(t.Context())
	if err != nil || status.CachedBytes != 0 || status.OverLimit {
		t.Fatalf("did not evict released object: %+v %v", status, err)
	}
}

func TestClientCacheAdmissionCountsPartialReservations(t *testing.T) {
	c := cacheTestClient(t)
	key := seedCachedOriginal(t, c, "part-whole", time.Now().UTC())
	if _, err := c.SetCacheLimits(t.Context(), time.Hour, 10); err != nil {
		t.Fatal(err)
	}
	release, err := c.cacheLock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(c.cachePath(key, ".data"), 4); err != nil {
		release()
		t.Fatal(err)
	}
	if err := writeClientJSON(c.cachePath(key, ".json"), cachedOriginal{Version: 1, Key: key, Size: 10, Offset: 4, AccessedAt: time.Now().UTC()}); err != nil {
		release()
		t.Fatal(err)
	}
	release()
	status, err := c.CacheStatus(t.Context())
	if err != nil || status.PartialBytes != 4 || status.ReservedBytes != 6 {
		t.Fatalf("partial reservation=%+v %v", status, err)
	}
	lease, err := c.cacheObjectLock(key, false).Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer lease()
	release, err = c.cacheLock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.pruneCacheLocked(t.Context(), 1, "")
	release()
	if !errors.Is(err, ErrCacheQuota) {
		t.Fatalf("admitted download beyond reserved quota: %v", err)
	}
}
