package library

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jinzhao1994/kist/internal/derive"
)

func TestVideoOnDemandDedupAndIndependentEviction(t *testing.T) {
	l, a, b := mediaFixture(t)
	v := l.VideoPlayback()
	v.probe = func(context.Context, string) (float64, error) { return 13, nil }
	var renders atomic.Int32
	v.render = func(ctx context.Context, input, output string, duration float64, index int) error {
		renders.Add(1)
		return os.WriteFile(output, []byte(strings.Repeat(string(rune('a'+index)), 32)), 0600)
	}
	ctx := t.Context()
	s, err := v.Source(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if s.Segments() != 3 || s.SegmentDuration(2) != 1 || !strings.Contains(s.Playlist(func(int) string { return "segment" }), "#EXT-X-ENDLIST") {
		t.Fatal(s)
	}
	if renders.Load() != 0 {
		t.Fatal("playlist generated video")
	}
	f, err := v.OpenSegment(ctx, a, s.Version(), 2)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(raw) != strings.Repeat("c", 32) {
		t.Fatal(string(raw), err)
	}
	f, err = v.OpenSegment(ctx, b, s.Version(), 2)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if renders.Load() != 1 {
		t.Fatal("shared content was rendered again")
	}
	if _, err = v.OpenSegment(ctx, a, "stale", 0); !errors.Is(err, ErrVideoChanged) {
		t.Fatal(err)
	}
	if _, err = v.OpenSegment(ctx, a, s.Version(), 3); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	// Keep a thumbnail outside the video cache throughout all pruning.
	thumb := l.Media()
	if _, err = thumb.Request(ctx, a, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err = thumb.work(ctx); err != nil {
		t.Fatal(err)
	}
	if err = updateInstanceConfig(ctx, l.layout.Root, "", map[string]any{"video.cache_max_bytes": int64(32)}); err != nil {
		t.Fatal(err)
	}
	held, err := v.OpenSegment(ctx, a, s.Version(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = v.OpenSegment(ctx, a, s.Version(), 0); !errors.Is(err, ErrCacheQuota) {
		t.Fatalf("evicted active response: %v", err)
	}
	if err = v.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = held.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	held.Close()
	if err = v.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = v.cached(ctx, s, 2); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if status, err := thumb.Status(ctx, a, derive.Thumbnail); err != nil || status.State != "ready" {
		t.Fatalf("cleared thumbnail: %+v %v", status, err)
	}
	f, err = v.OpenSegment(ctx, a, s.Version(), 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	db, _ := l.runtimeStore()
	if _, err = db.DB.Exec(`UPDATE video_segments SET accessed_at='2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	unlock, err := v.cacheLock().Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = v.pruneLocked(ctx, 0, false)
	unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = v.cached(ctx, s, 0); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("TTL not enforced", err)
	}
}

func TestVideoCancellationDoesNotPublish(t *testing.T) {
	l, id, _ := mediaFixture(t)
	v := l.VideoPlayback()
	v.probe = func(context.Context, string) (float64, error) { return 6, nil }
	s, err := v.Source(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	v.render = func(ctx context.Context, input, output string, duration float64, index int) error {
		if err := os.WriteFile(output, []byte("partial"), 0600); err != nil {
			return err
		}
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}
	if _, err = v.OpenSegment(ctx, id, s.Version(), 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = v.cached(t.Context(), s, 0); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial output published", err)
	}
}
