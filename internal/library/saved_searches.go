package library

import (
	"context"
	"errors"
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
	"reflect"
)

var ErrNoSuchSavedSearch = errors.New("no such saved search")

func (l *Library) SavedSearches(ctx context.Context) ([]domain.SavedSearch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return store.SavedSearches(l.db)
}
func (l *Library) CreateSavedSearch(ctx context.Context, name string, definition domain.SearchDefinition) (domain.SavedSearch, error) {
	s, err := domain.NewSavedSearch(domain.NewSavedSearchID(), name, definition)
	if err != nil {
		return s, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	raw, err := oplog.Marshal(oplog.SavedSearchPayload{After: &s})
	if err != nil {
		return s, err
	}
	_, err = l.write(ctx, oplog.ActionCreateSavedSearch, raw, oplog.OriginUser, writeOptions{})
	return s, err
}
func (l *Library) UpdateSavedSearch(ctx context.Context, expected domain.SavedSearch, name string, definition domain.SearchDefinition) error {
	id := expected.ID
	before, err := domain.NewSavedSearch(id, expected.Name, expected.Definition)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	current, ok, err := store.SavedSearchByID(l.db, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoSuchSavedSearch
	}
	if !reflect.DeepEqual(current, before) {
		return oplog.ErrStaleBefore
	}
	after, err := domain.NewSavedSearch(id, name, definition)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if reflect.DeepEqual(before, after) {
		return nil
	}
	raw, err := oplog.Marshal(oplog.SavedSearchPayload{Before: &before, After: &after})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionUpdateSavedSearch, raw, oplog.OriginUser, writeOptions{})
	return err
}
func (l *Library) DeleteSavedSearch(ctx context.Context, id domain.SavedSearchID) error {
	before, ok, err := store.SavedSearchByID(l.db, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoSuchSavedSearch
	}
	raw, err := oplog.Marshal(oplog.SavedSearchPayload{Before: &before})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionDeleteSavedSearch, raw, oplog.OriginUser, writeOptions{})
	return err
}
