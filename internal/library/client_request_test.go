package library

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientRequestReceiptSurvivesReopenSyncAndCallbackFailure(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	request := Request{ID: "client-request-one", Digest: strings.Repeat("a", 64)}
	proposal := syncBox(t, "request-local-box")
	callbackFailure := errors.New("response formatting failed")
	result, err := c.ExecuteRequest(ctx, request, func(scoped context.Context) error {
		if err := c.Stage(scoped, proposal); err != nil {
			return err
		}
		return callbackFailure
	})
	if !errors.Is(err, callbackFailure) || result.LocalID != proposal.ID {
		t.Fatalf("uncertain response=%+v %v", result, err)
	}
	other, err := OpenClient(ctx, c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	retry := func(context.Context) error { t.Fatal("retry executed callback"); return nil }
	result, err = other.ExecuteRequest(ctx, request, retry)
	if err != nil || result.LocalID != proposal.ID || result.Noop {
		t.Fatalf("reopened receipt=%+v %v", result, err)
	}
	if _, err := other.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	result, err = other.ExecuteRequest(ctx, request, retry)
	if err != nil || result.LocalID != proposal.ID {
		t.Fatalf("accepted receipt changed=%+v %v", result, err)
	}
	request.Digest = strings.Repeat("b", 64)
	if _, err := other.ExecuteRequest(ctx, request, retry); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("reused key=%v", err)
	}
}

func TestClientRequestNoopIsDurableAndDoesNotSync(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	request := Request{ID: "client-request-noop", Digest: strings.Repeat("A", 64)}
	result, err := c.ExecuteRequest(ctx, request, func(context.Context) error { return nil })
	if err != nil || !result.Noop || result.LocalID != "" {
		t.Fatalf("noop=%+v %v", result, err)
	}
	if err := c.Stage(ctx, syncBox(t, "later-independent-change")); err != nil {
		t.Fatal(err)
	}
	request.Digest = strings.ToLower(request.Digest)
	result, err = c.ExecuteRequest(ctx, request, func(context.Context) error { t.Fatal("noop replay changed later state"); return nil })
	if err != nil || !result.Noop {
		t.Fatalf("noop retry=%+v %v", result, err)
	}
	state, err := c.queue.read(ctx)
	if err != nil || len(state.entries) != 1 || len(state.requests) != 1 {
		t.Fatalf("receipt became intent: %+v %v", state, err)
	}
	synced, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || synced.Accepted != 1 {
		t.Fatalf("noop submitted to NAS: %+v %v", synced, err)
	}
}

func TestClientRequestFailedValidationCanRetryAndScopeExpires(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	request := Request{ID: "client-request-retry", Digest: strings.Repeat("c", 64)}
	var saved context.Context
	_, err := c.ExecuteRequest(ctx, request, func(scoped context.Context) error { saved = scoped; return c.Stage(scoped, SyncProposal{}) })
	if err == nil {
		t.Fatal("invalid proposal accepted")
	}
	result, err := c.ExecuteRequest(ctx, request, func(scoped context.Context) error { return c.Stage(scoped, syncBox(t, "valid-after-failure")) })
	if err != nil || result.LocalID == "" {
		t.Fatalf("valid retry=%+v %v", result, err)
	}
	if err := c.Stage(saved, syncBox(t, "expired-scope")); !errors.Is(err, ErrRequestScope) {
		t.Fatalf("expired scope=%v", err)
	}
}

func TestClientRequestConcurrentHandlesExecuteOnce(t *testing.T) {
	nas := openTestLibrary(t)
	first := newSyncClient(t, nas)
	second, err := OpenClient(t.Context(), first.root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	request := Request{ID: "concurrent-client-request", Digest: strings.Repeat("d", 64)}
	proposals := []SyncProposal{syncBox(t, "concurrent-first"), syncBox(t, "concurrent-second")}
	type answer struct {
		result ClientRequestResult
		err    error
	}
	answers := make(chan answer, 2)
	var calls atomic.Int32
	for i, client := range []*Client{first, second} {
		go func() {
			result, err := client.ExecuteRequest(t.Context(), request, func(scoped context.Context) error { calls.Add(1); return client.Stage(scoped, proposals[i]) })
			answers <- answer{result, err}
		}()
	}
	a, b := <-answers, <-answers
	if a.err != nil || b.err != nil || a.result != b.result || calls.Load() != 1 {
		t.Fatalf("duplicate execution: %+v %+v calls=%d", a, b, calls.Load())
	}
}
