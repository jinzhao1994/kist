package library

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

var (
	ErrNoSuchBox    = errors.New("no such box")
	ErrNoSuchTag    = errors.New("no such tag")
	ErrAmbiguousRef = errors.New("the reference matches more than one thing")
	ErrTagNameTaken = errors.New("a tag with that name already exists")
	ErrTagCycle     = errors.New("that move would put a tag inside its own subtree")
)

// CreateBox creates a box and its root folder.
//
// Both ids are minted here, not in Apply — that is the difference between a
// command and an event (spec 2.3.1). A replay hands the same ids back, so
// every reference into this box survives a rebuild.
func (l *Library) CreateBox(ctx context.Context, name, description string, start, end domain.LocalDate) (domain.Box, error) {
	if err := domain.ValidateName(name); err != nil {
		return domain.Box{}, err
	}
	p := oplog.CreateBoxPayload{
		BoxID:        domain.NewBoxID(),
		RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{
			Name: name, Description: description, StartAt: start, EndAt: end,
		},
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Box{}, err
	}
	if _, err := l.write(ctx, oplog.ActionCreateBox, raw, oplog.SourceUser, nil); err != nil {
		return domain.Box{}, err
	}
	b, _, err := l.Box(ctx, p.BoxID)
	return b, err
}

// UpdateBox rewrites a box's user-editable fields. The before state is read
// here and written into the payload, which is what makes Inverse a pure swap
// and strict undo possible (spec 2.3, 2.5).
func (l *Library) UpdateBox(ctx context.Context, id domain.BoxID, f BoxFields) (domain.Box, error) {
	if err := domain.ValidateName(f.Name); err != nil {
		return domain.Box{}, err
	}
	current, ok, err := l.Box(ctx, id)
	if err != nil {
		return domain.Box{}, err
	}
	if !ok {
		return domain.Box{}, fmt.Errorf("%s: %w", id, ErrNoSuchBox)
	}

	p := oplog.UpdateBoxPayload{
		BoxID: id,
		Before: oplog.BoxFields{
			Name: current.Name, Description: current.Description,
			StartAt: current.StartAt, EndAt: current.EndAt,
		},
		After: f,
	}
	if p.Before == p.After {
		return current, nil // nothing to record
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Box{}, err
	}
	if _, err := l.write(ctx, oplog.ActionUpdateBox, raw, oplog.SourceUser, nil); err != nil {
		return domain.Box{}, err
	}
	b, _, err := l.Box(ctx, id)
	return b, err
}

// CreateTag adds a node to the tag tree.
func (l *Library) CreateTag(ctx context.Context, name string, parent *domain.TagID) (domain.Tag, error) {
	if err := domain.ValidateName(name); err != nil {
		return domain.Tag{}, err
	}
	// Checked here rather than left to the unique index so the user gets a
	// sentence instead of a constraint violation — and so no op is written.
	if existing, ok := l.tags.ByName(name); ok {
		return domain.Tag{}, fmt.Errorf("%q is already a tag (%s): %w", existing.Name, existing.ID, ErrTagNameTaken)
	}
	if parent != nil {
		if _, ok := l.tags.ByID(*parent); !ok {
			return domain.Tag{}, fmt.Errorf("parent %s: %w", *parent, ErrNoSuchTag)
		}
	}

	p := oplog.CreateTagPayload{TagID: domain.NewTagID(), ParentID: parent, Name: name}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Tag{}, err
	}
	if _, err := l.write(ctx, oplog.ActionCreateTag, raw, oplog.SourceUser, nil); err != nil {
		return domain.Tag{}, err
	}
	tag, _ := l.tags.ByID(p.TagID)
	return tag, nil
}

func (l *Library) RenameTag(ctx context.Context, id domain.TagID, name string) error {
	if err := domain.ValidateName(name); err != nil {
		return err
	}
	current, ok := l.tags.ByID(id)
	if !ok {
		return fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	if current.Name == name {
		return nil
	}
	if other, exists := l.tags.ByName(name); exists && other.ID != id {
		return fmt.Errorf("%q is already a tag (%s): %w", other.Name, other.ID, ErrTagNameTaken)
	}

	raw, err := oplog.Marshal(oplog.RenameTagPayload{TagID: id, Before: current.Name, After: name})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionRenameTag, raw, oplog.SourceUser, nil)
	return err
}

// MoveTag reparents a subtree. In the database this is one row and one column,
// because there is no materialized path to cascade (spec 1.6).
func (l *Library) MoveTag(ctx context.Context, id domain.TagID, parent *domain.TagID) error {
	current, ok := l.tags.ByID(id)
	if !ok {
		return fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	if parent != nil {
		if _, ok := l.tags.ByID(*parent); !ok {
			return fmt.Errorf("parent %s: %w", *parent, ErrNoSuchTag)
		}
		// A loop would make Descendants recurse until the stack gives out.
		// Deciding this here, not in Apply, is invariant 3.
		if l.tags.WouldCycle(id, *parent) {
			return fmt.Errorf("%q cannot go under its own descendant: %w", current.Name, ErrTagCycle)
		}
	}
	if sameTagID(current.ParentID, parent) {
		return nil
	}

	raw, err := oplog.Marshal(oplog.MoveTagPayload{TagID: id, OldParent: current.ParentID, NewParent: parent})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionMoveTag, raw, oplog.SourceUser, nil)
	return err
}

// DeleteTag removes a leaf that nothing is using. The refusal to cascade is
// deliberate (spec 1.6).
func (l *Library) DeleteTag(ctx context.Context, id domain.TagID) error {
	current, ok := l.tags.ByID(id)
	if !ok {
		return fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	raw, err := oplog.Marshal(oplog.DeleteTagPayload{
		TagID: id, ParentID: current.ParentID, Name: current.Name,
	})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionDeleteTag, raw, oplog.SourceUser, nil)
	return err
}

// TagBoxes attaches one tag to many boxes as a single op, and returns how many
// boxes actually changed.
//
// Boxes that already carry the tag are filtered out before the op is built. If
// they were left in, the inverse would strip the tag from boxes this op never
// touched — an undo doing more than the op did.
func (l *Library) TagBoxes(ctx context.Context, tag domain.TagID, boxes []domain.BoxID) (int, error) {
	changed, err := l.filterBoxTags(ctx, tag, boxes, false)
	if err != nil || len(changed) == 0 {
		return 0, err
	}
	raw, err := oplog.Marshal(oplog.TagBoxesPayload{TagID: tag, BoxIDs: changed, Source: "user"})
	if err != nil {
		return 0, err
	}
	if _, err := l.write(ctx, oplog.ActionTagBoxes, raw, oplog.SourceUser, nil); err != nil {
		return 0, err
	}
	return len(changed), nil
}

// UntagBoxes is the mirror image.
func (l *Library) UntagBoxes(ctx context.Context, tag domain.TagID, boxes []domain.BoxID) (int, error) {
	changed, err := l.filterBoxTags(ctx, tag, boxes, true)
	if err != nil || len(changed) == 0 {
		return 0, err
	}
	raw, err := oplog.Marshal(oplog.UntagBoxesPayload{TagID: tag, BoxIDs: changed, Source: "user"})
	if err != nil {
		return 0, err
	}
	if _, err := l.write(ctx, oplog.ActionUntagBoxes, raw, oplog.SourceUser, nil); err != nil {
		return 0, err
	}
	return len(changed), nil
}

// filterBoxTags keeps the boxes for which the operation would actually change
// something. wantPresent is what the box must currently look like to qualify.
func (l *Library) filterBoxTags(ctx context.Context, tag domain.TagID, boxes []domain.BoxID, wantPresent bool) ([]domain.BoxID, error) {
	if _, ok := l.tags.ByID(tag); !ok {
		return nil, fmt.Errorf("%s: %w", tag, ErrNoSuchTag)
	}
	var changed []domain.BoxID
	seen := map[domain.BoxID]bool{}
	err := l.readTx(ctx, func(vtx *store.ViewTx) error {
		for _, id := range boxes {
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, ok, err := vtx.GetBox(id); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("%s: %w", id, ErrNoSuchBox)
			}
			has, err := vtx.HasBoxTag(id, tag)
			if err != nil {
				return err
			}
			if has == wantPresent {
				changed = append(changed, id)
			}
		}
		return nil
	})
	return changed, err
}

// ResolveBox accepts a full id, an id prefix, or an exact name.
//
// Box names are not unique — the schema gives boxes.name_fold an ordinary
// index, not a unique one — so an ambiguous name is reported with its
// candidates rather than resolved by picking one.
func (l *Library) ResolveBox(ctx context.Context, ref string) (domain.Box, error) {
	if id, err := domain.ParseID(ref); err == nil {
		b, ok, err := l.Box(ctx, domain.BoxID{ID: id})
		if err != nil {
			return domain.Box{}, err
		}
		if !ok {
			return domain.Box{}, fmt.Errorf("%s: %w", ref, ErrNoSuchBox)
		}
		return b, nil
	}

	all, err := l.Boxes(ctx, store.ListBoxesOptions{})
	if err != nil {
		return domain.Box{}, err
	}
	fold := domain.Fold(ref)
	var matches []domain.Box
	for _, b := range all {
		if b.NameFold == fold || strings.HasPrefix(b.ID.String(), ref) {
			matches = append(matches, b)
		}
	}
	switch len(matches) {
	case 0:
		return domain.Box{}, fmt.Errorf("%q: %w", ref, ErrNoSuchBox)
	case 1:
		return matches[0], nil
	default:
		var sb strings.Builder
		for _, b := range matches {
			fmt.Fprintf(&sb, "\n  %s  %s", b.ID, b.Name)
		}
		return domain.Box{}, fmt.Errorf("%q matches %d boxes; use an id instead:%s: %w",
			ref, len(matches), sb.String(), ErrAmbiguousRef)
	}
}

// ResolveTag accepts a full id or a name. Names are globally unique, so this
// one cannot be ambiguous (spec 1.6).
func (l *Library) ResolveTag(ref string) (domain.Tag, error) {
	if id, err := domain.ParseID(ref); err == nil {
		if tag, ok := l.tags.ByID(domain.TagID{ID: id}); ok {
			return tag, nil
		}
	}
	if tag, ok := l.tags.ByName(ref); ok {
		return tag, nil
	}
	return domain.Tag{}, fmt.Errorf("%q: %w", ref, ErrNoSuchTag)
}

func sameTagID(a, b *domain.TagID) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
