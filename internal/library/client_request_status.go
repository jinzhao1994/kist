package library

import (
	"context"
	"strings"
)

// RequestStatus recovers the durable local receipt without rerunning a command.
// Its identity remains local even after NAS acceptance or conflict resolution.
func (c *Client) RequestStatus(ctx context.Context, key string) (ClientRequestResult, bool, error) {
	return c.requestStatus(ctx, key, "")
}

// CheckRequest validates the original business input before expensive preparation.
func (c *Client) CheckRequest(ctx context.Context, request Request) (ClientRequestResult, bool, error) {
	if !request.Valid() {
		return ClientRequestResult{}, false, ErrInvalidRequest
	}
	return c.requestStatus(ctx, request.ID, strings.ToLower(request.Digest))
}

func (c *Client) requestStatus(ctx context.Context, key, expected string) (ClientRequestResult, bool, error) {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return ClientRequestResult{}, false, ErrRequestScope
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return ClientRequestResult{}, false, err
	}
	defer release()
	state, err := c.queue.read(ctx)
	if err != nil {
		return ClientRequestResult{}, false, err
	}
	receipt, found := state.requests[key]
	if !found {
		return ClientRequestResult{}, false, nil
	}
	if expected != "" && receipt.Digest != expected {
		return ClientRequestResult{}, false, ErrRequestConflict
	}
	return ClientRequestResult{LocalID: receipt.LocalID, Noop: receipt.LocalID == ""}, true, nil
}
