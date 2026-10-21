package library

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClientRequestLookupRecoversStableReceipt(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	request := Request{ID: "lost-http-response", Digest: strings.Repeat("a", 64)}
	if _, found, err := c.CheckRequest(ctx, request); err != nil || found {
		t.Fatalf("absent=%v %v", found, err)
	}
	proposal := syncBox(t, "lookup-box")
	responseLost := errors.New("response lost")
	expected, err := c.ExecuteRequest(ctx, request, func(scoped context.Context) error {
		if err := c.Stage(scoped, proposal); err != nil {
			return err
		}
		return responseLost
	})
	if !errors.Is(err, responseLost) {
		t.Fatal(err)
	}
	other, err := OpenClient(ctx, c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	assertReceipt := func() {
		t.Helper()
		got, found, err := other.RequestStatus(ctx, request.ID)
		if err != nil || !found || got != expected {
			t.Fatalf("receipt=%+v %v %v", got, found, err)
		}
	}
	assertReceipt()
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	assertReceipt()
	upper := request
	upper.Digest = strings.ToUpper(upper.Digest)
	if _, found, err := other.CheckRequest(ctx, upper); err != nil || !found {
		t.Fatalf("case normalization=%v %v", found, err)
	}
	changed := request
	changed.Digest = strings.Repeat("b", 64)
	if _, _, err := other.CheckRequest(ctx, changed); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("changed request=%v", err)
	}
	noop := Request{ID: "noop-request-lookup", Digest: strings.Repeat("c", 64)}
	if _, err := c.ExecuteRequest(ctx, noop, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	got, found, err := other.CheckRequest(ctx, noop)
	if err != nil || !found || !got.Noop || got.LocalID != "" {
		t.Fatalf("noop=%+v %v %v", got, found, err)
	}
	if _, _, err := other.CheckRequest(ctx, Request{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid=%v", err)
	}
}
