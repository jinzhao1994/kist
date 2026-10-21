package library

import (
	"context"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// commandLock shares the enclosing request lock when present. Holding the scope
// mutex also prevents concurrent callbacks from generating two different edits.
func (c *Client) commandLock(ctx context.Context) (*clientRequestScope, func(), error) {
	if scope, ok := ctx.Value(clientRequestContextKey{}).(*clientRequestScope); ok {
		scope.mu.Lock()
		if !scope.active || scope.client != c || scope.attempted {
			scope.mu.Unlock()
			return nil, nil, ErrRequestScope
		}
		return scope, scope.mu.Unlock, nil
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err := c.rebuildLocked(ctx); err != nil {
		release()
		return nil, nil, err
	}
	return nil, release, nil
}

func (c *Client) commandWriteLocked(ctx context.Context, scope *clientRequestScope, action oplog.Action, payload any) error {
	_, err := c.commandIntentLocked(ctx, scope, action, payload)
	return err
}

func (c *Client) commandIntentLocked(ctx context.Context, scope *clientRequestScope, action oplog.Action, payload any, files ...SyncFileState) (SyncReference, error) {
	raw, err := oplog.Marshal(payload)
	if err != nil {
		return SyncReference{}, err
	}
	p := SyncProposal{Version: 1, ID: domain.NewID().String(), OccurredAt: time.Now().UTC(), Action: string(action), Origin: "user", Payload: raw, Files: files}
	return c.stageCommandLocked(ctx, scope, p)
}

func (c *Client) stageCommandLocked(ctx context.Context, scope *clientRequestScope, p SyncProposal) (SyncReference, error) {
	var request *Request
	if scope != nil {
		scope.attempted = true
		scope.localID = p.ID
		request = &scope.request
	}
	err := c.stageLocked(ctx, p, request)
	if scope != nil {
		scope.err = err
	}
	return SyncReference{LocalID: p.ID}, err
}

func (c *Client) CreateBox(ctx context.Context, name, description string, start, end domain.LocalDate) (domain.Box, error) {
	p, err := createBoxPayload(name, description, start, end)
	if err != nil {
		return domain.Box{}, err
	}
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return domain.Box{}, err
	}
	defer release()
	if err := c.commandWriteLocked(ctx, scope, oplog.ActionCreateBox, p); err != nil {
		return domain.Box{}, err
	}
	result, found, err := store.DisplayBox(c.db, p.BoxID)
	if err == nil && !found {
		err = ErrInvariantViolated
	}
	return result, err
}

func (c *Client) UpdateBox(ctx context.Context, id domain.BoxID, fields BoxFields) (domain.Box, error) {
	if err := domain.ValidateName(fields.Name); err != nil {
		return domain.Box{}, err
	}
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return domain.Box{}, err
	}
	defer release()
	current, found, err := store.DisplayBox(c.db, id)
	if err != nil {
		return domain.Box{}, err
	}
	if !found {
		return domain.Box{}, fmt.Errorf("%s: %w", id, ErrNoSuchBox)
	}
	p := updateBoxPayload(current, fields)
	if p.Empty() {
		return current, nil
	}
	if err := c.commandWriteLocked(ctx, scope, oplog.ActionUpdateBox, p); err != nil {
		return domain.Box{}, err
	}
	result, found, err := store.DisplayBox(c.db, id)
	if err == nil && !found {
		err = ErrInvariantViolated
	}
	return result, err
}

// SetFileset changes only the configuration, returning an independent local
// intent reference. A zero reference means that no configuration changed.
func (c *Client) SetFileset(ctx context.Context, box domain.BoxID, id domain.FolderID, before, after *domain.Fileset) (SyncReference, error) {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return SyncReference{}, err
	}
	defer release()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return SyncReference{}, err
	}
	p, err := filesetCommandPayload(store.NewViewTx(tx, time.Time{}), box, id, before, after)
	rollbackErr := tx.Rollback()
	if err != nil {
		return SyncReference{}, err
	}
	if rollbackErr != nil {
		return SyncReference{}, rollbackErr
	}
	if domain.EqualFileset(before, after) {
		return SyncReference{}, nil
	}
	return c.commandIntentLocked(ctx, scope, oplog.ActionSetFileset, p)
}

// CreateFolder adds an empty directory using the same hierarchy constraints as
// the NAS. The result is a local intent reference, not a NAS operation number.
func (c *Client) CreateFolder(ctx context.Context, box domain.BoxID, parent domain.FolderID, name string) (SyncReference, error) {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return SyncReference{}, err
	}
	defer release()
	b, found, err := store.DisplayBox(c.db, box)
	if err != nil {
		return SyncReference{}, err
	}
	if !found {
		return SyncReference{}, ErrNoSuchBox
	}
	folders, err := store.FoldersOfBox(c.db, box)
	if err != nil {
		return SyncReference{}, err
	}
	p, err := createFolderPayload(b, folders, parent, name)
	if err != nil {
		return SyncReference{}, err
	}
	return c.commandIntentLocked(ctx, scope, oplog.ActionCreateFolder, p)
}
