package library

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
)

func claimClientMedia(t *testing.T, m *ClientMedia, id domain.FileID) MediaJob {
	t.Helper()
	j, err := m.resolve(t.Context(), id, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	j, err = m.read(j)
	if err != nil {
		t.Fatal(err)
	}
	j.UpdatedAt = time.Now().UTC().Add(-time.Minute)
	if err = m.save(j); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := m.claim(t.Context())
	if err != nil || !ok {
		t.Fatal(claimed, ok, err)
	}
	return claimed
}

func TestClientMediaQueueOfflineAndEviction(t *testing.T) {
	nas, id, _ := mediaFixture(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	source := &derivedTestSource{testSyncPeer: &testSyncPeer{nas: nas}, media: nas.Media()}
	providerCalls := 0
	provider := func(context.Context) (DerivedSource, error) { providerCalls++; return source, nil }
	m := c.Media(provider)
	if j, err := m.Status(ctx, id, derive.Thumbnail); err != nil || j.State != "missing" {
		t.Fatal(j, err)
	}
	first, err := m.Request(ctx, id, derive.Thumbnail, false)
	if err != nil {
		t.Fatal(err)
	}
	if j, err := m.Request(ctx, id, derive.Thumbnail, false); err != nil || j.Generation != first.Generation {
		t.Fatal(j, err)
	}
	if providerCalls != 0 {
		t.Fatal("request contacted NAS")
	}
	// A new coordinator recovers the persisted request, including interrupted work.
	m = c.Media(provider)
	claimed := claimClientMedia(t, m, id)
	m = c.Media(provider)
	recovered, ok, err := m.claim(ctx)
	if err != nil || !ok || recovered.Generation != claimed.Generation {
		t.Fatal(recovered, ok, err)
	}
	if err = m.transfer(ctx, recovered); err != nil {
		t.Fatal(err)
	}
	ready, err := m.Status(ctx, id, derive.Thumbnail)
	if err != nil || ready.State != "ready" {
		t.Fatal(ready, err)
	}
	source.offline = true
	calls := providerCalls
	_, r, err := m.Open(ctx, id, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := io.ReadAll(r); err != nil || int64(len(data)) != ready.Result.Bytes {
		t.Fatal(len(data), err)
	}
	r.Close()
	if j, err := m.Request(ctx, id, derive.Thumbnail, true); err != nil || j.State != "ready" {
		t.Fatal(j, err)
	}
	if providerCalls != calls {
		t.Fatal("cached request contacted NAS")
	}
	if _, err = c.SetCacheLimits(ctx, time.Hour, 1); err != nil {
		t.Fatal(err)
	}
	if j, err := m.Status(ctx, id, derive.Thumbnail); err != nil || j.State != "missing" {
		t.Fatal(j, err)
	}
	if _, _, err = m.Open(ctx, id, derive.Thumbnail); !errors.Is(err, ErrNotCached) {
		t.Fatal(err)
	}
	if _, err = m.Request(ctx, id, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err = m.transfer(ctx, claimClientMedia(t, m, id)); err != nil {
		t.Fatal(err)
	}
	if j, err := m.Status(ctx, id, derive.Thumbnail); err != nil || j.State != "blocked" || j.Reason != "not_cached" {
		t.Fatal(j, err)
	}
	source.offline = false
	if _, err = c.SetCacheLimits(ctx, time.Hour, 10<<20); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Request(ctx, id, derive.Thumbnail, true); err != nil {
		t.Fatal(err)
	}
	if err = m.transfer(ctx, claimClientMedia(t, m, id)); err != nil {
		t.Fatal(err)
	}
	if j, err := m.Status(ctx, id, derive.Thumbnail); err != nil || j.State != "ready" {
		t.Fatal(j, err)
	}
}

func TestClientMediaCancelStopsTransferAndPreservesNewRequest(t *testing.T) {
	nas, id, _ := mediaFixture(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	started := make(chan struct{})
	m := c.Media(func(ctx context.Context) (DerivedSource, error) { close(started); <-ctx.Done(); return nil, ctx.Err() })
	if _, err := m.Request(ctx, id, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	j := claimClientMedia(t, m, id)
	done := make(chan error, 1)
	go func() { done <- m.transfer(ctx, j) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("transfer did not start")
	}
	cancelled, err := m.Cancel(ctx, id, derive.Thumbnail)
	if err != nil || cancelled.State != "cancelled" {
		t.Fatal(cancelled, err)
	}
	next, err := m.Request(ctx, id, derive.Thumbnail, true)
	if err != nil || next.Generation == j.Generation {
		t.Fatal(next, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop transfer")
	}
	if current, err := m.Status(ctx, id, derive.Thumbnail); err != nil || current.Generation != next.Generation || current.State != "queued" {
		t.Fatal(current, err)
	}
}

func TestClientMediaDamagedQueueCanBeRequestedAgain(t *testing.T) {
	nas, id, _ := mediaFixture(t)
	c := newSyncClient(t, nas)
	m := c.Media(func(context.Context) (DerivedSource, error) { t.Fatal("unexpected network"); return nil, nil })
	j, err := m.Request(t.Context(), id, derive.Thumbnail, false)
	if err != nil {
		t.Fatal(err)
	}
	path, err := m.path(j)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(`{"state":`), 0600); err != nil {
		t.Fatal(err)
	}
	next, err := m.Request(t.Context(), id, derive.Thumbnail, false)
	if err != nil || next.State != "queued" || next.Generation == j.Generation {
		t.Fatal(next, err)
	}
}
