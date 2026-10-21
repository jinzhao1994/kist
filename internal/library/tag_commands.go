package library

import (
	"fmt"
	"slices"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func createTagPayload(q store.Queryer, name string, parent *domain.TagID) (oplog.CreateTagPayload, domain.Tag, error) {
	var p oplog.CreateTagPayload
	var tag domain.Tag
	if err := domain.ValidateName(name); err != nil {
		return p, tag, err
	}
	existing, taken, err := store.TagByName(q, name)
	if err != nil {
		return p, tag, err
	}
	if taken {
		return p, tag, &NameTakenError{Name: existing.Name, ExistingID: existing.ID.String()}
	}
	if parent != nil {
		if _, ok, err := store.TagByID(q, *parent); err != nil {
			return p, tag, err
		} else if !ok {
			return p, tag, fmt.Errorf("parent %s: %w", *parent, ErrNoSuchTag)
		}
	}
	p = oplog.CreateTagPayload{TagID: domain.NewTagID(), ParentID: parent, Name: name}
	tag, err = domain.NewTag(p.TagID, p.ParentID, p.Name)
	return p, tag, err
}

func renameTagPayload(q store.Queryer, id domain.TagID, name string) (*oplog.RenameTagPayload, error) {
	if err := domain.ValidateName(name); err != nil {
		return nil, err
	}
	current, ok, err := store.TagByID(q, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	if current.Name == name {
		return nil, nil
	}
	other, exists, err := store.TagByName(q, name)
	if err != nil {
		return nil, err
	}
	if exists && other.ID != id {
		return nil, &NameTakenError{Name: other.Name, ExistingID: other.ID.String()}
	}
	return &oplog.RenameTagPayload{TagID: id, Before: current.Name, After: name}, nil
}

func moveTagPayload(q store.Queryer, id domain.TagID, parent *domain.TagID) (*oplog.MoveTagPayload, error) {
	current, ok, err := store.TagByID(q, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	if parent != nil {
		if _, ok, err := store.TagByID(q, *parent); err != nil {
			return nil, err
		} else if !ok {
			return nil, fmt.Errorf("parent %s: %w", *parent, ErrNoSuchTag)
		}
		// Decisions stay in command generation, outside deterministic Apply.
		above, err := store.TagAncestors(q, *parent)
		if err != nil {
			return nil, err
		}
		if slices.Contains(above, id) {
			return nil, fmt.Errorf("%q cannot go under its own descendant: %w", current.Name, ErrTagCycle)
		}
	}
	if sameTagID(current.ParentID, parent) {
		return nil, nil
	}
	return &oplog.MoveTagPayload{TagID: id, OldParent: current.ParentID, NewParent: parent}, nil
}

func deleteTagPayload(q store.Queryer, id domain.TagID) (oplog.DeleteTagPayload, error) {
	current, ok, err := store.TagByID(q, id)
	if err != nil {
		return oplog.DeleteTagPayload{}, err
	}
	if !ok {
		return oplog.DeleteTagPayload{}, fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	return oplog.DeleteTagPayload{TagID: id, ParentID: current.ParentID, Name: current.Name}, nil
}
