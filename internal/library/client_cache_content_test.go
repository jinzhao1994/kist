package library

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
)

type cacheTestSource struct {
	nas     *Library
	content string
	cut     int
	offsets []int64
}

func (s *cacheTestSource) Capabilities(ctx context.Context) (SyncCapabilities, error) {
	return (&testSyncPeer{nas: s.nas}).Capabilities(ctx)
}
func (s *cacheTestSource) OpenOriginal(_ context.Context, _ string, _ int64, offset int64) (io.ReadCloser, error) {
	s.offsets = append(s.offsets, offset)
	content := s.content[offset:]
	if s.cut > 0 {
		content = content[:min(s.cut, len(content))]
		s.cut = 0
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func TestClientCacheResumesAndLeasesVerifiedContent(t *testing.T) {
	nas, box := openWithBox(t)
	content := "original from a traveling laptop"
	f := importOneFile(t, nas, box, "trip.txt", content)
	c := newSyncClient(t, nas)
	source := &cacheTestSource{nas: nas, content: content, cut: 7}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("interrupted download=%v", err)
	}
	if _, err := c.OpenCachedOriginal(t.Context(), f.BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatalf("served incomplete bytes: %v", err)
	}
	status, err := c.CacheStatus(t.Context())
	if err != nil || status.PartialBytes != 7 || status.ReservedBytes != int64(len(content)-7) {
		t.Fatalf("partial usage=%+v %v", status, err)
	}
	// Model a crash after writing data but before recording the next offset.
	partial, err := os.OpenFile(c.cachePath(f.BlobKey, ".data"), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = partial.WriteString("unconfirmed tail")
	partial.Close()
	if err != nil {
		t.Fatal(err)
	}
	other, err := OpenClient(t.Context(), c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := other.CacheOriginal(t.Context(), f.BlobKey, source); err != nil {
		t.Fatal(err)
	}
	if len(source.offsets) != 2 || source.offsets[0] != 0 || source.offsets[1] != 7 {
		t.Fatalf("wrong resume offsets: %v", source.offsets)
	}
	reader, err := other.OpenCachedOriginal(t.Context(), f.BlobKey)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := io.ReadAll(reader)
	if err != nil || string(bytes) != content {
		reader.Close()
		t.Fatalf("cached content=%q %v", bytes, err)
	}
	status, err = c.SetCacheLimits(t.Context(), time.Hour, 1)
	if err != nil || !status.OverLimit || status.CachedBytes != int64(len(content)) {
		reader.Close()
		t.Fatalf("evicted active reader: %+v %v", status, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	status, err = c.PruneCache(t.Context())
	if err != nil || status.CachedBytes != 0 {
		t.Fatalf("released cache not evicted: %+v %v", status, err)
	}
	if _, err := other.OpenCachedOriginal(t.Context(), f.BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatalf("evicted object=%v", err)
	}
}

func TestClientCacheRejectsQuotaAndCorruption(t *testing.T) {
	nas, box := openWithBox(t)
	content := "verified original"
	f := importOneFile(t, nas, box, "original.txt", content)
	c := newSyncClient(t, nas)
	source := &cacheTestSource{nas: nas, content: strings.Repeat("!", len(content))}
	if _, err := c.SetCacheLimits(t.Context(), time.Hour, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); !errors.Is(err, ErrCacheQuota) {
		t.Fatalf("admitted oversized download: %v", err)
	}
	if len(source.offsets) != 0 {
		t.Fatal("download began before quota admission")
	}
	if _, err := c.SetCacheLimits(t.Context(), time.Hour, 100); err != nil {
		t.Fatal(err)
	}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); !errors.Is(err, blob.ErrIntegrityMismatch) {
		t.Fatalf("published corrupt download: %v", err)
	}
	record, err := c.cacheRecord(f.BlobKey)
	if err != nil || record.Complete || record.Offset != 0 {
		t.Fatalf("corrupt transfer not reset: %+v %v", record, err)
	}
	source.content = content
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.cachePath(f.BlobKey, ".data"), []byte(strings.Repeat("x", len(content))), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.OpenCachedOriginal(t.Context(), f.BlobKey); !errors.Is(err, blob.ErrIntegrityMismatch) {
		t.Fatalf("damaged bytes treated as uncached: %v", err)
	}
	if err := os.Remove(c.cachePath(f.BlobKey, ".data")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.OpenCachedOriginal(t.Context(), f.BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatalf("missing cached bytes treated as missing NAS original: %v", err)
	}
	if err := c.CacheOriginal(t.Context(), f.BlobKey, source); err != nil {
		t.Fatalf("missing cache could not be downloaded again: %v", err)
	}
}
