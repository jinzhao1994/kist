package oplog

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func createTag(t *testing.T, tx Tx, parent *domain.TagID, name string) domain.TagID {
	t.Helper()
	p := CreateTagPayload{TagID: domain.NewTagID(), ParentID: parent, Name: name}
	raw, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(t, tx, ActionCreateTag, raw); err != nil {
		t.Fatalf("create tag %q: %v", name, err)
	}
	return p.TagID
}

func TestCreateTagApply(t *testing.T) {
	tx := newFakeTx(opTime)
	id := createTag(t, tx, nil, "人物")

	tag, ok, _ := tx.GetTag(id)
	if !ok {
		t.Fatal("the tag was not created")
	}
	if tag.Name != "人物" || tag.NameFold != domain.Fold("人物") {
		t.Errorf("tag = %+v", tag)
	}
	if tag.ParentID != nil {
		t.Error("a root tag should have no parent")
	}
}

func TestCreateTagUnderParent(t *testing.T) {
	tx := newFakeTx(opTime)
	people := createTag(t, tx, nil, "人物")
	dad := createTag(t, tx, &people, "爸")

	tag, _, _ := tx.GetTag(dad)
	if tag.ParentID == nil || *tag.ParentID != people {
		t.Errorf("ParentID = %v, want %v", tag.ParentID, people)
	}
}

func TestCreateTagInverseIsDelete(t *testing.T) {
	tx := newFakeTx(opTime)
	p := CreateTagPayload{TagID: domain.NewTagID(), Name: "临时"}
	raw, _ := Marshal(p)
	apply(t, tx, ActionCreateTag, raw)

	action, inverse := inverseOf(t, ActionCreateTag, raw)
	if action != ActionDeleteTag {
		t.Fatalf("inverse action = %q, want delete_tag", action)
	}
	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	if _, ok, _ := tx.GetTag(p.TagID); ok {
		t.Error("the tag survived its deletion")
	}
}

func TestDeleteTagRefusesWithChildren(t *testing.T) {
	// spec 1.6: cascade delete in archiving software can wipe a whole
	// classification with one misclick, and the payload needed to undo that
	// would be enormous.
	tx := newFakeTx(opTime)
	people := createTag(t, tx, nil, "人物")
	createTag(t, tx, &people, "爸")

	raw, _ := Marshal(DeleteTagPayload{TagID: people, Name: "人物"})
	err := apply(t, tx, ActionDeleteTag, raw)
	if err == nil {
		t.Fatal("deleting a tag with children succeeded")
	}
	if !errors.Is(err, ErrTagInUse) {
		t.Errorf("error = %v, want ErrTagInUse", err)
	}
}

func TestDeleteTagRefusesWhenAttached(t *testing.T) {
	tx := newFakeTx(opTime)
	tag := createTag(t, tx, nil, "云南")
	box, raw := createBoxPayload(t, "trip")
	apply(t, tx, ActionCreateBox, raw)
	tx.AddBoxTag(box.BoxID, tag, "user")

	delRaw, _ := Marshal(DeleteTagPayload{TagID: tag, Name: "云南"})
	err := apply(t, tx, ActionDeleteTag, delRaw)
	if err == nil {
		t.Fatal("deleting a tag that is still attached succeeded")
	}
	if !errors.Is(err, ErrTagInUse) {
		t.Errorf("error = %v, want ErrTagInUse", err)
	}
}

func TestRenameTagApplyAndInverse(t *testing.T) {
	tx := newFakeTx(opTime)
	id := createTag(t, tx, nil, "旧名")

	raw, _ := Marshal(RenameTagPayload{TagID: id, Before: "旧名", After: "新名"})
	if err := apply(t, tx, ActionRenameTag, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	tag, _, _ := tx.GetTag(id)
	if tag.Name != "新名" || tag.NameFold != domain.Fold("新名") {
		t.Errorf("after rename: %+v", tag)
	}

	action, inv := inverseOf(t, ActionRenameTag, raw)
	if action != ActionRenameTag {
		t.Errorf("inverse action = %q, want rename_tag", action)
	}
	if err := apply(t, tx, action, inv); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	tag, _, _ = tx.GetTag(id)
	if tag.Name != "旧名" {
		t.Errorf("after undo the name is %q", tag.Name)
	}
}

func TestRenameTagRefusesAStaleBefore(t *testing.T) {
	tx := newFakeTx(opTime)
	id := createTag(t, tx, nil, "实际名字")

	raw, _ := Marshal(RenameTagPayload{TagID: id, Before: "别的名字", After: "无所谓"})
	if err := apply(t, tx, ActionRenameTag, raw); !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want ErrStaleBefore", err)
	}
}

func TestMoveTagIsOneFieldChange(t *testing.T) {
	// spec 1.6: no materialized path column, so moving a subtree is a single
	// parent_id write no matter how big the subtree is.
	tx := newFakeTx(opTime)
	places := createTag(t, tx, nil, "地点")
	people := createTag(t, tx, nil, "人物")
	yunnan := createTag(t, tx, &places, "云南")
	createTag(t, tx, &yunnan, "大理")
	createTag(t, tx, &yunnan, "丽江")

	raw, _ := Marshal(MoveTagPayload{TagID: yunnan, OldParent: &places, NewParent: &people})
	if err := apply(t, tx, ActionMoveTag, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	moved, _, _ := tx.GetTag(yunnan)
	if moved.ParentID == nil || *moved.ParentID != people {
		t.Errorf("ParentID = %v, want %v", moved.ParentID, people)
	}
	// The children were not touched: they still point at 云南, which is the
	// whole reason the payload is only three fields.
	n, _ := tx.CountTagChildren(yunnan)
	if n != 2 {
		t.Errorf("云南 has %d children after the move, want 2", n)
	}
}

func TestMoveTagToRoot(t *testing.T) {
	tx := newFakeTx(opTime)
	places := createTag(t, tx, nil, "地点")
	yunnan := createTag(t, tx, &places, "云南")

	raw, _ := Marshal(MoveTagPayload{TagID: yunnan, OldParent: &places, NewParent: nil})
	if err := apply(t, tx, ActionMoveTag, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	moved, _, _ := tx.GetTag(yunnan)
	if moved.ParentID != nil {
		t.Errorf("ParentID = %v, want nil", moved.ParentID)
	}

	action, inv := inverseOf(t, ActionMoveTag, raw)
	if action != ActionMoveTag {
		t.Errorf("inverse action = %q", action)
	}
	if err := apply(t, tx, action, inv); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	moved, _, _ = tx.GetTag(yunnan)
	if moved.ParentID == nil || *moved.ParentID != places {
		t.Errorf("after undo ParentID = %v, want %v", moved.ParentID, places)
	}
}

func TestMoveTagRefusesAStaleParent(t *testing.T) {
	tx := newFakeTx(opTime)
	a := createTag(t, tx, nil, "a")
	b := createTag(t, tx, nil, "b")
	child := createTag(t, tx, &a, "child")

	wrong := b
	raw, _ := Marshal(MoveTagPayload{TagID: child, OldParent: &wrong, NewParent: nil})
	if err := apply(t, tx, ActionMoveTag, raw); !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want ErrStaleBefore", err)
	}
}

func TestTagOpsTouchNoBoxes(t *testing.T) {
	// The tag tree belongs to no box (spec 2.2.1), so these ops produce no
	// op_boxes rows. Returning an empty Targets is the honest answer.
	tx := newFakeTx(opTime)
	id := createTag(t, tx, nil, "孤立")

	cases := []struct {
		action  Action
		payload any
	}{
		{ActionCreateTag, CreateTagPayload{TagID: id, Name: "孤立"}},
		{ActionDeleteTag, DeleteTagPayload{TagID: id, Name: "孤立"}},
		{ActionRenameTag, RenameTagPayload{TagID: id, Before: "a", After: "b"}},
		{ActionMoveTag, MoveTagPayload{TagID: id}},
	}
	for _, c := range cases {
		raw, _ := Marshal(c.payload)
		h, _ := Lookup(c.action)
		got, err := h.Targets(raw)
		if err != nil {
			t.Fatalf("Targets(%s): %v", c.action, err)
		}
		if len(got.Boxes) != 0 || len(got.Files) != 0 {
			t.Errorf("Targets(%s) = %+v, want empty", c.action, got)
		}
	}
}
