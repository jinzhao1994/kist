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
	p, err := createBoxPayload(name, description, start, end)
	if err != nil {
		return domain.Box{}, err
	}

	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Box{}, err
	}
	opID, err := l.write(ctx, oplog.ActionCreateBox, raw, oplog.OriginUser, writeOptions{})
	if err != nil {
		return domain.Box{}, err
	}
	b, _, err := l.Box(ctx, p.BoxID)
	if err != nil {
		return domain.Box{}, &WriteOutcomeError{OpID: opID, Cause: err}
	}
	return b, nil
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

	p := updateBoxPayload(current, f)
	if p.Empty() {
		return current, nil // nothing to record
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Box{}, err
	}
	opID, err := l.write(ctx, oplog.ActionUpdateBox, raw, oplog.OriginUser, writeOptions{})
	if err != nil {
		return domain.Box{}, err
	}
	b, _, err := l.Box(ctx, id)
	if err != nil {
		return domain.Box{}, &WriteOutcomeError{OpID: opID, Cause: err}
	}
	return b, nil
}

// CreateTag adds a node to the tag tree.
func (l *Library) CreateTag(ctx context.Context, name string, parent *domain.TagID) (domain.Tag, error) {
	p, tag, err := createTagPayload(l.db, name, parent)
	if err != nil {
		return domain.Tag{}, err
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Tag{}, err
	}
	if _, err := l.write(ctx, oplog.ActionCreateTag, raw, oplog.OriginUser, writeOptions{}); err != nil {
		return domain.Tag{}, err
	}
	return tag, nil
}

func (l *Library) RenameTag(ctx context.Context, id domain.TagID, name string) error {
	p, err := renameTagPayload(l.db, id, name)
	if err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionRenameTag, raw, oplog.OriginUser, writeOptions{})
	return err
}

// MoveTag reparents a subtree. In the database this is one row and one column,
// because there is no materialized path to cascade (spec 1.6).
func (l *Library) MoveTag(ctx context.Context, id domain.TagID, parent *domain.TagID) error {
	p, err := moveTagPayload(l.db, id, parent)
	if err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionMoveTag, raw, oplog.OriginUser, writeOptions{})
	return err
}

// DeleteTag removes a leaf that nothing is using. The refusal to cascade is
// deliberate (spec 1.6).
func (l *Library) DeleteTag(ctx context.Context, id domain.TagID) error {
	p, err := deleteTagPayload(l.db, id)
	if err != nil {
		return err
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionDeleteTag, raw, oplog.OriginUser, writeOptions{})
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
	raw, err := oplog.Marshal(oplog.TagBoxesPayload{TagID: tag, BoxIDs: changed})
	if err != nil {
		return 0, err
	}
	if _, err := l.write(ctx, oplog.ActionTagBoxes, raw, oplog.OriginUser, writeOptions{}); err != nil {
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
	raw, err := oplog.Marshal(oplog.UntagBoxesPayload{TagID: tag, BoxIDs: changed})
	if err != nil {
		return 0, err
	}
	if _, err := l.write(ctx, oplog.ActionUntagBoxes, raw, oplog.OriginUser, writeOptions{}); err != nil {
		return 0, err
	}
	return len(changed), nil
}

// filterBoxTags keeps the boxes for which the operation would actually change
// something. wantPresent is what the box must currently look like to qualify.
func (l *Library) filterBoxTags(ctx context.Context, tag domain.TagID, boxes []domain.BoxID, wantPresent bool) ([]domain.BoxID, error) {
	var changed []domain.BoxID
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		changed, err = filterBoxTagsInView(tx, tag, boxes, wantPresent)
		return err
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
		return domain.Box{}, &AmbiguousRefError{
			Ref: ref, Kind: "box", Candidates: boxCandidates(matches),
		}
	}
}

// ResolveTag accepts a full id or a name. Names are globally unique, so this
// one cannot be ambiguous (spec 1.6).
func (l *Library) ResolveTag(ref string) (domain.Tag, error) {
	if id, err := domain.ParseID(ref); err == nil {
		tag, ok, err := store.TagByID(l.db, domain.TagID{ID: id})
		if err != nil {
			return domain.Tag{}, err
		}
		if ok {
			return tag, nil
		}
	}
	tag, ok, err := store.TagByName(l.db, ref)
	if err != nil {
		return domain.Tag{}, err
	}
	if ok {
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
