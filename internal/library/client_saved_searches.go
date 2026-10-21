package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func (c *Client) SavedSearches(ctx context.Context) ([]domain.SavedSearch, error) {
	var result []domain.SavedSearch
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = store.SavedSearches(tx.Queryer())
		return err
	})
	return result, err
}

func (c *Client) CreateSavedSearch(ctx context.Context, name string, definition domain.SearchDefinition) (domain.SavedSearch, error) {
	search, err := domain.NewSavedSearch(domain.NewSavedSearchID(), name, definition)
	if err != nil {
		return search, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return search, err
	}
	defer release()
	err = c.commandWriteLocked(ctx, scope, oplog.ActionCreateSavedSearch, oplog.SavedSearchPayload{After: &search})
	return search, err
}

func (c *Client) UpdateSavedSearch(ctx context.Context, expected domain.SavedSearch, name string, definition domain.SearchDefinition) error {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	payload, err := savedSearchUpdatePayload(c.db, expected, name, definition)
	if err != nil || payload == nil {
		return err
	}
	return c.commandWriteLocked(ctx, scope, oplog.ActionUpdateSavedSearch, payload)
}

func (c *Client) DeleteSavedSearch(ctx context.Context, id domain.SavedSearchID) error {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	before, found, err := store.SavedSearchByID(c.db, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNoSuchSavedSearch
	}
	return c.commandWriteLocked(ctx, scope, oplog.ActionDeleteSavedSearch, oplog.SavedSearchPayload{Before: &before})
}
