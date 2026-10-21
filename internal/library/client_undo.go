package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// Undo appends an inverse intent using a stable NAS or local reference. It does
// not erase a pending operation; both the original and its inverse remain in
// history and synchronize in dependency order.
func (c *Client) Undo(ctx context.Context, target SyncReference) (SyncReference, error) {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return SyncReference{}, err
	}
	defer release()
	proposal := SyncProposal{Version: 1, ID: domain.NewID().String(), OccurredAt: time.Now().UTC(), Origin: "user", Undo: &target}
	result, err := c.stageCommandLocked(ctx, scope, proposal)
	if errors.Is(err, oplog.ErrStaleBefore) || store.IsStateConflict(err) {
		err = fmt.Errorf("%w: %v", ErrUndoBlocked, err)
		if scope != nil {
			scope.err = err
		}
	}
	return result, err
}
