package library

import (
	"context"
	"strings"
	"sync"
)

// ClientRequestResult is stable across retry and NAS acceptance. LocalID is an
// independent intent identity, never a mutable client-view operation ordinal.
type ClientRequestResult struct {
	LocalID string `json:"local_id,omitempty"`
	Noop    bool   `json:"noop"`
}

type clientRequestContextKey struct{}
type clientRequestScope struct {
	mu        sync.Mutex
	client    *Client
	request   Request
	active    bool
	attempted bool
	localID   string
	err       error
}

// ExecuteRequest durably deduplicates a local command and its no-op result.
// The callback may stage at most one intent, using the supplied context. Content
// preparation and network work must happen before entering this editing scope.
func (c *Client) ExecuteRequest(ctx context.Context, request Request, fn func(context.Context) error) (ClientRequestResult, error) {
	if !request.Valid() {
		return ClientRequestResult{}, ErrInvalidRequest
	}
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(requestContextKey{}) != nil || fn == nil {
		return ClientRequestResult{}, ErrRequestScope
	}
	request.Digest = strings.ToLower(request.Digest)
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return ClientRequestResult{}, err
	}
	defer release()
	state, err := c.queue.read(ctx)
	if err != nil {
		return ClientRequestResult{}, err
	}
	if previous, ok := state.requests[request.ID]; ok {
		if previous.Digest != request.Digest {
			return ClientRequestResult{}, ErrRequestConflict
		}
		if _, err := c.rebuildLocked(ctx); err != nil {
			return ClientRequestResult{}, err
		}
		return ClientRequestResult{LocalID: previous.LocalID, Noop: previous.LocalID == ""}, nil
	}
	if _, err := c.rebuildLocked(ctx); err != nil {
		return ClientRequestResult{}, err
	}
	scope := &clientRequestScope{client: c, request: request, active: true}
	defer func() { scope.mu.Lock(); scope.active = false; scope.mu.Unlock() }()
	callbackErr := fn(context.WithValue(ctx, clientRequestContextKey{}, scope))
	scope.mu.Lock()
	attempted, localID, writeErr := scope.attempted, scope.localID, scope.err
	scope.mu.Unlock()
	result := ClientRequestResult{LocalID: localID}
	if writeErr != nil {
		return result, writeErr
	}
	if callbackErr != nil {
		return result, callbackErr
	}
	if !attempted {
		if err := c.queue.appendLocked(ctx, "pending_request", pendingEvent{Request: &request}); err != nil {
			return result, err
		}
		result.Noop = true
	}
	return result, nil
}
