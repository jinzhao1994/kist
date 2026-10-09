package library

import (
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
)

func mediaFixture(t *testing.T) (*Library, domain.FileID, domain.FileID) {
	t.Helper()
	l, b := openWithBox(t)
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "a.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 32, 16))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	raw, _ := os.ReadFile(filepath.Join(dir, "a.png"))
	if err = os.WriteFile(filepath.Join(dir, "b.png"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = l.Import(t.Context(), dir, ImportOptions{Box: b}); err != nil {
		t.Fatal(err)
	}
	files := listAll(t, l, b)
	return l, files[0].ID, files[1].ID
}

func TestMediaDurabilityDedupAndMissingOutput(t *testing.T) {
	l, a, b := mediaFixture(t)
	m := l.Media()
	ctx := t.Context()
	j, err := m.Request(ctx, a, derive.Thumbnail, false)
	if err != nil {
		t.Fatal(err)
	}
	other := l.Media()
	same, err := other.Request(ctx, b, derive.Thumbnail, false)
	if err != nil || same.Generation != j.Generation {
		t.Fatalf("dedup: %+v %v", same, err)
	}
	if err = other.work(ctx); err != nil {
		t.Fatal(err)
	}
	ready, f, err := m.Open(ctx, a, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if ready.State != "ready" || ready.Result.Width != 32 || ready.Result.Height != 16 {
		t.Fatalf("%+v", ready)
	}
	_, path, _ := m.paths(ready)
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	missing, err := m.Status(ctx, b, derive.Thumbnail)
	if err != nil || missing.State != "missing" {
		t.Fatalf("%+v %v", missing, err)
	}
	next, err := m.Request(ctx, a, derive.Thumbnail, false)
	if err != nil || next.Generation == j.Generation {
		t.Fatalf("%+v %v", next, err)
	}
	// A worker death leaves running; the successor regenerates it.
	if _, ok, err := m.claim(ctx); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if err = other.work(ctx); err != nil {
		t.Fatal(err)
	}
	ready, err = m.Status(ctx, a, derive.Thumbnail)
	if err != nil || ready.State != "ready" {
		t.Fatalf("%+v %v", ready, err)
	}
	// Media records do not append archive operations or change deep verification.
	report, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Total() != 0 {
		t.Fatalf("verify: %+v", report)
	}
}

func TestMediaCancelRetryDiscardsOldGeneration(t *testing.T) {
	l, id, _ := mediaFixture(t)
	m := l.Media()
	ctx := t.Context()
	started := make(chan struct{})
	finish := make(chan struct{})
	done := make(chan error, 1)
	m.render = func(ctx context.Context, _, out, _ string) (derive.Result, error) {
		close(started)
		<-finish
		// Deliberately ignore cancellation to test the publication guard.
		err := os.WriteFile(out, []byte("stale"), 0600)
		return derive.Result{Bytes: 5}, err
	}
	m.Request(ctx, id, derive.Thumbnail, false)
	j, ok, err := m.claim(ctx)
	if err != nil || !ok {
		t.Fatal(err)
	}
	go func() { done <- m.generate(ctx, j) }()
	<-started
	cancelled, err := l.Media().Cancel(ctx, id, derive.Thumbnail)
	if err != nil || cancelled.State != "cancelled" {
		t.Fatalf("%+v %v", cancelled, err)
	}
	fresh, err := l.Media().Request(ctx, id, derive.Thumbnail, true)
	if err != nil {
		t.Fatal(err)
	}
	close(finish)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	current, _ := m.Status(ctx, id, derive.Thumbnail)
	if current.Generation != fresh.Generation || current.State != "queued" {
		t.Fatalf("stale publish: %+v", current)
	}
	if _, _, err = m.Open(ctx, id, derive.Thumbnail); !errors.Is(err, ErrMediaNotReady) {
		t.Fatal(err)
	}
	if err = l.Media().work(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMediaBlockedAutomaticallyRetriesAndShutdownRecovers(t *testing.T) {
	l, id, _ := mediaFixture(t)
	m := l.Media()
	ctx := t.Context()
	m.render = func(context.Context, string, string, string) (derive.Result, error) {
		return derive.Result{}, &derive.Failure{Code: "dependency_missing", Cause: errors.New("missing")}
	}
	m.Request(ctx, id, derive.Thumbnail, false)
	if err := m.work(ctx); err != nil {
		t.Fatal(err)
	}
	j, _ := m.Status(ctx, id, derive.Thumbnail)
	if j.State != "blocked" {
		t.Fatalf("%+v", j)
	}
	j.UpdatedAt = time.Now().Add(-time.Minute)
	if err := m.save(j); err != nil {
		t.Fatal(err)
	}
	if err := l.Media().work(ctx); err != nil {
		t.Fatal(err)
	}
	j, _ = m.Status(ctx, id, derive.Thumbnail)
	if j.State != "ready" {
		t.Fatalf("%+v", j)
	}
	m.Request(ctx, id, derive.Preview, false)
	started := make(chan struct{})
	m.render = func(ctx context.Context, _, _, _ string) (derive.Result, error) {
		close(started)
		<-ctx.Done()
		return derive.Result{}, ctx.Err()
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { m.Run(runCtx); close(done) }()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown stuck")
	}
	j, _ = m.Status(ctx, id, derive.Preview)
	if j.State != "running" {
		t.Fatalf("shutdown lost task: %+v", j)
	}
	if err := l.Media().work(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMediaWorkersSerializeAndDoNotHoldArchiveLock(t *testing.T) {
	l, id, _ := mediaFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var calls atomic.Int32
	started := make(chan struct{})
	unblock := make(chan struct{})
	render := func(ctx context.Context, _, out, _ string) (derive.Result, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-unblock:
		case <-ctx.Done():
			return derive.Result{}, ctx.Err()
		}
		return derive.Result{Bytes: 2}, os.WriteFile(out, []byte("ok"), 0600)
	}
	a, b := l.Media(), l.Media()
	a.render = render
	b.render = render
	a.Request(ctx, id, derive.Thumbnail, false)
	done := make(chan struct{}, 2)
	go func() { a.Run(ctx); done <- struct{}{} }()
	<-started
	go func() { b.Run(ctx); done <- struct{}{} }()
	if _, err := l.CreateBox(ctx, "during render", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	close(unblock)
	deadline := time.Now().Add(3 * time.Second)
	for {
		j, _ := a.Status(ctx, id, derive.Thumbnail)
		if j.State == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	<-done
	if calls.Load() != 1 {
		t.Fatalf("duplicate render: %d", calls.Load())
	}
}

func TestMediaPathsRetainCollisionSuffixAndRejectTraversal(t *testing.T) {
	l := openTestLibrary(t)
	m := l.Media()
	base := MediaJob{BlobKey: "sha256:" + strings.Repeat("a", 64), Recipe: derive.Thumbnail}
	_, a, err := m.paths(base)
	if err != nil {
		t.Fatal(err)
	}
	base.BlobKey += ".1"
	_, b, err := m.paths(base)
	if err != nil || a == b || !strings.Contains(b, ".1") {
		t.Fatalf("%q %q %v", a, b, err)
	}
	base.Recipe = "../../evil"
	if _, _, err = m.paths(base); err == nil {
		t.Fatal("accepted traversal")
	}
}
