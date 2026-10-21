package library

import (
	"errors"
	"io"
	"testing"
	"time"
)

func TestClientContentRetainsCacheLease(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	content := "cached video bytes"
	file := importOneFile(t, nas, box, "clip.mp4", content)
	c := newSyncClient(t, nas)
	if _, _, err := c.OpenContent(ctx, file.ID); !errors.Is(err, ErrNotCached) {
		t.Fatalf("uncached=%v", err)
	}
	if err := c.CacheOriginal(ctx, file.BlobKey, &cacheTestSource{nas: nas, content: content}); err != nil {
		t.Fatal(err)
	}
	got, handle, err := c.OpenContent(ctx, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if got.ID != file.ID {
		t.Fatal("wrong file")
	}
	status, err := c.SetCacheLimits(ctx, time.Hour, 1)
	if err != nil || !status.OverLimit || status.CachedBytes != int64(len(content)) {
		t.Fatalf("active content evicted: %+v %v", status, err)
	}
	data, err := io.ReadAll(handle)
	if err != nil || string(data) != content {
		t.Fatalf("bytes=%q %v", data, err)
	}
	handle.Close()
	status, err = c.PruneCache(ctx)
	if err != nil || status.CachedBytes != 0 {
		t.Fatalf("prune=%+v %v", status, err)
	}
}
