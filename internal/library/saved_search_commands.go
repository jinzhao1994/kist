package library

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
	"reflect"
)

func savedSearchUpdatePayload(q store.Queryer, expected domain.SavedSearch, name string, definition domain.SearchDefinition) (*oplog.SavedSearchPayload, error) {
	id := expected.ID
	before, err := domain.NewSavedSearch(id, expected.Name, expected.Definition)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	current, ok, err := store.SavedSearchByID(q, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNoSuchSavedSearch
	}
	if !reflect.DeepEqual(current, before) {
		return nil, oplog.ErrStaleBefore
	}
	after, err := domain.NewSavedSearch(id, name, definition)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if reflect.DeepEqual(before, after) {
		return nil, nil
	}
	return &oplog.SavedSearchPayload{Before: &before, After: &after}, nil
}
