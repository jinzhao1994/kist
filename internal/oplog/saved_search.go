package oplog

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"reflect"
)

// SavedSearchPayload includes both semantic states for strict undo.
// A missing Before creates; a missing After deletes.
type SavedSearchPayload struct {
	Before *domain.SavedSearch `json:"before,omitempty"`
	After  *domain.SavedSearch `json:"after,omitempty"`
}
type savedSearchOp struct{ action Action }

func init() {
	for _, action := range []Action{ActionCreateSavedSearch, ActionUpdateSavedSearch, ActionDeleteSavedSearch} {
		Register(action, savedSearchOp{action})
	}
}
func (h savedSearchOp) Apply(tx Tx, raw []byte) error {
	p, err := Payload[SavedSearchPayload](raw)
	if err != nil {
		return err
	}
	if (h.action == ActionCreateSavedSearch && (p.Before != nil || p.After == nil)) ||
		(h.action == ActionDeleteSavedSearch && (p.Before == nil || p.After != nil)) ||
		(h.action == ActionUpdateSavedSearch && (p.Before == nil || p.After == nil || p.Before.ID != p.After.ID)) {
		return fmt.Errorf("invalid saved search transition")
	}
	for _, state := range []*domain.SavedSearch{p.Before, p.After} {
		if state != nil {
			canonical, err := domain.NewSavedSearch(state.ID, state.Name, state.Definition)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(canonical, *state) {
				return fmt.Errorf("noncanonical saved search")
			}
		}
	}
	if p.Before != nil {
		current, ok, err := tx.GetSavedSearch(p.Before.ID)
		if err != nil {
			return err
		}
		if !ok || !reflect.DeepEqual(current, *p.Before) {
			return fmt.Errorf("saved search has changed: %w", ErrStaleBefore)
		}
	}
	if p.After != nil {
		other, ok, err := tx.SavedSearchByName(p.After.Name)
		if err != nil {
			return err
		}
		if ok && other.ID != p.After.ID {
			return fmt.Errorf("saved search name already exists: %w", ErrStaleBefore)
		}
	}
	if p.After == nil {
		return tx.DeleteSavedSearch(p.Before.ID)
	}
	if p.Before == nil {
		return tx.InsertSavedSearch(*p.After)
	}
	return tx.UpdateSavedSearch(*p.After)
}
func (h savedSearchOp) Inverse(raw []byte) (Action, []byte, error) {
	p, err := Payload[SavedSearchPayload](raw)
	if err != nil {
		return "", nil, err
	}
	p.Before, p.After = p.After, p.Before
	action := ActionUpdateSavedSearch
	if p.Before == nil {
		action = ActionCreateSavedSearch
	} else if p.After == nil {
		action = ActionDeleteSavedSearch
	}
	encoded, err := Marshal(p)
	return action, encoded, err
}

// Searches do not own content, so their references are not op content targets.
func (savedSearchOp) Targets([]byte) (Targets, error) { return Targets{}, nil }
