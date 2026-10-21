package library

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
)

func TestClientCacheRebuildPreservesPinsAndProtectedOriginals(t *testing.T) {
	nas, box := openWithBox(t)
	content := "cached NAS original"
	f := importOneFile(t, nas, box, "cached.txt", content)
	c := newSyncClient(t, nas)
	source := &cacheTestSource{nas: nas, content: content}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); err != nil {
		t.Fatal(err)
	}
	if err := c.SetOfflineOriginals(t.Context(), "trip", []string{f.BlobKey}); err != nil {
		t.Fatal(err)
	}
	pinsBefore, err := os.ReadFile(filepath.Join(c.root, "cache-pins.json"))
	if err != nil {
		t.Fatal(err)
	}
	protected, err := c.PutOriginal(t.Context(), strings.NewReader("not yet on NAS"))
	if err != nil {
		t.Fatal(err)
	}
	p := syncBox(t, "rebuild-cache-pending")
	if err := c.Stage(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(c.cachePath(f.BlobKey, ".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CacheStatus(t.Context()); !errors.Is(err, ErrCacheMetadata) {
		t.Fatalf("orphan bytes counted as zero: %v", err)
	}
	result, err := c.RebuildCache(t.Context())
	if err != nil || result.Verified != 1 {
		t.Fatalf("rebuild=%+v %v", result, err)
	}
	reader, err := c.OpenCachedOriginal(t.Context(), f.BlobKey)
	if err != nil {
		t.Fatal(err)
	}
	result, err = c.RebuildCache(t.Context())
	reader.Close()
	if err != nil || result.Skipped != 1 || result.Verified != 0 {
		t.Fatalf("rebuilt leased object: %+v %v", result, err)
	}
	if err := os.WriteFile(c.cachePath(f.BlobKey, ".json"), []byte("{broken index"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = c.RebuildCache(t.Context())
	if err != nil || result.Verified != 1 {
		t.Fatalf("damaged index recovery=%+v %v", result, err)
	}
	pinsAfter, err := os.ReadFile(filepath.Join(c.root, "cache-pins.json"))
	if err != nil || string(pinsBefore) != string(pinsAfter) {
		t.Fatal("cache rebuild changed offline selections")
	}
	local, err := c.blobs.Get(t.Context(), protected.Key)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := io.ReadAll(local)
	local.Close()
	if err != nil || string(bytes) != "not yet on NAS" {
		t.Fatal("cache rebuild damaged protected original")
	}
	state, err := c.queue.read(t.Context())
	if err != nil || len(state.entries) != 1 || state.entries[0].Proposal.ID != p.ID {
		t.Fatal("cache rebuild changed pending history")
	}
}

func TestClientCacheRebuildReportsCorruptBytesAndAllowsRetry(t *testing.T) {
	nas, box := openWithBox(t)
	content := "verified cache content"
	f := importOneFile(t, nas, box, "content.txt", content)
	c := newSyncClient(t, nas)
	if _, err := c.SetCacheLimits(t.Context(), time.Hour, int64(len(content))); err != nil {
		t.Fatal(err)
	}
	source := &cacheTestSource{nas: nas, content: content}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.cachePath(f.BlobKey, ".data"), []byte(strings.Repeat("x", len(content)+7)), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := c.RebuildCache(t.Context())
	if err != nil || result.Corrupt != 1 || result.Verified != 0 {
		t.Fatalf("corruption not reported: %+v %v", result, err)
	}
	if _, err := c.OpenCachedOriginal(t.Context(), f.BlobKey); !errors.Is(err, blob.ErrIntegrityMismatch) {
		t.Fatalf("corrupt content presented as uncached: %v", err)
	}
	status, err := c.CacheStatus(t.Context())
	if err != nil || status.CorruptBytes != int64(len(content)+7) {
		t.Fatalf("corrupt bytes unaccounted: %+v %v", status, err)
	}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); err != nil {
		t.Fatalf("verified refetch failed: %v", err)
	}
	if err := os.Remove(c.cachePath(f.BlobKey, ".data")); err != nil {
		t.Fatal(err)
	}
	result, err = c.RebuildCache(t.Context())
	if err != nil || result.Missing != 1 {
		t.Fatalf("missing cached content=%+v %v", result, err)
	}
	if _, err := c.OpenCachedOriginal(t.Context(), f.BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatalf("missing cache=%v", err)
	}
}

func TestClientCacheRebuildRetainsOnlyConfirmedPartialBytes(t *testing.T) {
	nas, box := openWithBox(t)
	content := "partially transferred content"
	f := importOneFile(t, nas, box, "partial.txt", content)
	c := newSyncClient(t, nas)
	source := &cacheTestSource{nas: nas, content: content, cut: 7}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	file, err := os.OpenFile(c.cachePath(f.BlobKey, ".data"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString("unconfirmed")
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.RebuildCache(t.Context())
	if err != nil || result.Partial != 1 {
		t.Fatalf("partial recovery=%+v %v", result, err)
	}
	info, err := os.Stat(c.cachePath(f.BlobKey, ".data"))
	if err != nil || info.Size() != 7 {
		t.Fatalf("unconfirmed bytes retained: %v %v", info, err)
	}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); err != nil {
		t.Fatal(err)
	}
	record, err := c.cacheRecord(f.BlobKey)
	if err != nil {
		t.Fatal(err)
	}
	record.Version = 2
	if err := writeClientJSON(c.cachePath(f.BlobKey, ".json"), record); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RebuildCache(t.Context()); !errors.Is(err, ErrSyncVersion) {
		t.Fatalf("rewrote future cache metadata: %v", err)
	}
}
