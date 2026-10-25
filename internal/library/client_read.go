package library

import (
	"context"
	"database/sql"
	"time"

	"github.com/jinzhao1994/kist/internal/store"
)

// readView refreshes the durable overlay under the edit lock, then reads one
// SQLite snapshot. Scoped reads share the surrounding request's lock and may
// observe its completed command without attempting another mutation.
func (c *Client) readView(ctx context.Context, fn func(*store.ViewTx) error) error {
	if view, ok := ctx.Value(clientViewContextKey{}).(*clientViewScope); ok {
		view.mu.Lock()
		defer view.mu.Unlock()
		if !view.active || view.client != c {
			return ErrRequestScope
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return fn(view.tx)
	}
	if scope, ok := ctx.Value(clientRequestContextKey{}).(*clientRequestScope); ok {
		scope.mu.Lock()
		defer scope.mu.Unlock()
		if !scope.active || scope.client != c {
			return ErrRequestScope
		}
	} else {
		release, err := c.lock.Acquire(ctx)
		if err != nil {
			return err
		}
		defer release()
		if err := c.refreshViewLocked(ctx); err != nil {
			return err
		}
	}
	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(store.NewViewTx(tx, time.Time{}))
}
