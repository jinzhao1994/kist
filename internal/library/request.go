package library

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// Request is a client-generated key and a digest of the complete business input.
type Request = journal.Request

var (
	ErrInvalidRequest  = errors.New("invalid request key or digest")
	ErrRequestConflict = errors.New("this request key was already used for different input")
	ErrRequestScope    = errors.New("a request must perform at most one library write in its callback")
)

// RequestResult remains stable across retries, restarts and rebuilds.
type RequestResult struct {
	OpID int64 `json:"op_id"`
}

type requestContextKey struct{}

type requestScope struct {
	mu        sync.Mutex
	library   *Library
	request   Request
	active    bool
	attempted bool
	opID      int64
	err       error
}

// ExecuteRequest serializes lookup and execution with every library writer.
// fn must make at most one business write and use the supplied context. Prepare
// expensive external inputs before entering this scope. No-op success gets a
// receipt, so retrying after a later state change cannot create a new effect.
func (l *Library) ExecuteRequest(ctx context.Context, req Request, fn func(context.Context) error) (RequestResult, error) {
	if !req.Valid() {
		return RequestResult{}, ErrInvalidRequest
	}
	req.Digest = strings.ToLower(req.Digest)
	if ctx.Value(requestContextKey{}) != nil || fn == nil {
		return RequestResult{}, ErrRequestScope
	}
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return RequestResult{}, err
	}
	defer release()
	if err := l.catchUpLocked(ctx); err != nil {
		return RequestResult{}, err
	}
	digest, opID, found, err := store.RequestOp(l.db, req.ID)
	if err != nil {
		return RequestResult{}, err
	}
	if found {
		if digest != req.Digest {
			return RequestResult{}, ErrRequestConflict
		}
		return RequestResult{OpID: opID}, nil
	}
	scope := &requestScope{library: l, request: req, active: true}
	defer func() { scope.mu.Lock(); scope.active = false; scope.mu.Unlock() }()
	scoped := context.WithValue(ctx, requestContextKey{}, scope)
	callbackErr := fn(scoped)
	scope.mu.Lock()
	attempted, opID, writeErr := scope.attempted, scope.opID, scope.err
	scope.mu.Unlock()
	if writeErr != nil {
		return RequestResult{OpID: opID}, writeErr
	}
	if callbackErr != nil {
		if opID != 0 {
			return RequestResult{OpID: opID}, &WriteOutcomeError{OpID: opID, Cause: callbackErr}
		}
		return RequestResult{}, callbackErr
	}
	if !attempted {
		opID, err = l.write(scoped, oplog.ActionRequestReceipt, []byte(`{}`), oplog.OriginMaintenance, writeOptions{})
		if err != nil {
			return RequestResult{OpID: opID}, err
		}
	}
	return RequestResult{OpID: opID}, nil
}

// RequestStatus recovers pending log entries before looking up a receipt. It
// never reruns a command and is safe after a response or connection was lost.
func (l *Library) RequestStatus(ctx context.Context, key string) (RequestResult, bool, error) {
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return RequestResult{}, false, err
	}
	defer release()
	if err := l.catchUpLocked(ctx); err != nil {
		return RequestResult{}, false, err
	}
	_, opID, found, err := store.RequestOp(l.db, key)
	return RequestResult{OpID: opID}, found, err
}
