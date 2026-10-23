package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestCacheMaintenanceStartupAndPeriodicOffline(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	box, err := nas.CreateBox(ctx, "cache", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	data := "disposable cached content"
	path := filepath.Join(t.TempDir(), "cache.txt")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Import(ctx, path, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := nas.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	server := clientServerFromNAS(t, nas)
	client := server.client
	// No remote connection is configured. Only a cache source is supplied here.
	if _, err := client.SetCacheLimits(ctx, time.Nanosecond, 1024); err != nil {
		t.Fatal(err)
	}
	cache := func() {
		t.Helper()
		if err := client.CacheOriginal(ctx, files[0].BlobKey, previewOriginalSource{nas, data}); err != nil {
			t.Fatal(err)
		}
	}
	waitBytes := func(want int64) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			status, err := client.CacheStatus(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if status.CachedBytes == want {
				return
			}
			time.Sleep(time.Millisecond * 10)
		}
		t.Fatalf("cache did not reach %d bytes", want)
	}
	cache()
	runCtx, cancel := context.WithCancel(ctx)
	ticks := make(chan time.Time)
	done := make(chan struct{})
	go func() { server.runCacheMaintenance(runCtx, ticks); close(done) }()
	defer func() { cancel(); <-done }()
	waitBytes(0)
	cache()
	waitBytes(int64(len(data)))
	select {
	case ticks <- time.Now():
	case <-time.After(5 * time.Second):
		t.Fatal("maintenance did not wait for next interval")
	}
	waitBytes(0)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("maintenance did not stop")
	}
}
