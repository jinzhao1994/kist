package oplog

import (
	"errors"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

var opTime = time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)

func createBoxPayload(t *testing.T, name string) (CreateBoxPayload, []byte) {
	t.Helper()
	p := CreateBoxPayload{
		BoxID:        domain.NewBoxID(),
		RootFolderID: domain.NewFolderID(),
		BoxFields:    BoxFields{Name: name},
	}
	raw, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, raw
}

func apply(t *testing.T, tx Tx, action Action, raw []byte) error {
	t.Helper()
	h, err := Lookup(action)
	if err != nil {
		t.Fatal(err)
	}
	return h.Apply(tx, raw)
}

func inverseOf(t *testing.T, a Action, raw []byte) (Action, []byte) {
	t.Helper()
	h, err := Lookup(a)
	if err != nil {
		t.Fatal(err)
	}
	action, payload, err := h.Inverse(raw)
	if err != nil {
		t.Fatalf("Inverse(%s): %v", a, err)
	}
	return action, payload
}

func TestCreateBoxApply(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "云南 2019")

	if err := apply(t, tx, ActionCreateBox, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	b, ok, _ := tx.GetBox(p.BoxID)
	if !ok {
		t.Fatal("the box was not created")
	}
	if b.Name != "云南 2019" || b.NameFold != domain.Fold("云南 2019") {
		t.Errorf("box = %+v", b)
	}
	if !b.CreatedAt.Equal(opTime) {
		t.Errorf("CreatedAt = %v, want the op's ts %v", b.CreatedAt, opTime)
	}

	// spec 1.4: every box has exactly one root folder, and its id must come
	// from the payload — not be minted during Apply, or a replay would hand
	// out fresh ids and break every files.folder_id pointing at it.
	root, ok := tx.folders[p.RootFolderID]
	if !ok {
		t.Fatal("the root folder was not created")
	}
	if root.ParentID != nil || root.Name != "" {
		t.Errorf("root folder = %+v", root)
	}
	if b.RootFolderID != p.RootFolderID {
		t.Errorf("box points at root %s, want %s", b.RootFolderID, p.RootFolderID)
	}
}

func TestCreateBoxIsDeterministic(t *testing.T) {
	// Invariant 3: applying the same payload to two empty transactions must
	// produce identical state. This is the property `kist rebuild` rests on.
	_, raw := createBoxPayload(t, "同一个 payload")

	a, b := newFakeTx(opTime), newFakeTx(opTime)
	if err := apply(t, a, ActionCreateBox, raw); err != nil {
		t.Fatal(err)
	}
	if err := apply(t, b, ActionCreateBox, raw); err != nil {
		t.Fatal(err)
	}

	if len(a.boxes) != 1 || len(b.boxes) != 1 {
		t.Fatalf("box counts %d and %d", len(a.boxes), len(b.boxes))
	}
	for id, boxA := range a.boxes {
		boxB, ok := b.boxes[id]
		if !ok {
			t.Fatalf("box %s exists in one replay but not the other", id)
		}
		if boxA != boxB {
			t.Errorf("two replays produced different boxes:\n  %+v\n  %+v", boxA, boxB)
		}
	}
}

func TestCreateBoxInverseIsDeleteBox(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "regrettable")
	apply(t, tx, ActionCreateBox, raw)

	action, inverse := inverseOf(t, ActionCreateBox, raw)
	if action != ActionDeleteBox {
		t.Fatalf("inverse action = %q, want %q", action, ActionDeleteBox)
	}

	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	if _, ok, _ := tx.GetBox(p.BoxID); ok {
		t.Error("the box survived its own deletion")
	}
	if len(tx.folders) != 0 {
		t.Errorf("%d folders were left behind", len(tx.folders))
	}
}

func TestDeleteBoxInverseRestoresEverything(t *testing.T) {
	// Undo of an undo. The payload must carry enough to rebuild the box
	// byte-for-byte, including the root folder id (spec 2.3).
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "resurrect me")
	apply(t, tx, ActionCreateBox, raw)

	delAction, delPayload := inverseOf(t, ActionCreateBox, raw)
	apply(t, tx, delAction, delPayload)

	backAction, backPayload := inverseOf(t, ActionDeleteBox, delPayload)
	if backAction != ActionCreateBox {
		t.Fatalf("inverse of delete = %q, want create_box", backAction)
	}
	if err := apply(t, tx, backAction, backPayload); err != nil {
		t.Fatalf("re-creating: %v", err)
	}

	b, ok, _ := tx.GetBox(p.BoxID)
	if !ok {
		t.Fatal("the box did not come back")
	}
	if b.Name != "resurrect me" {
		t.Errorf("name came back as %q", b.Name)
	}
	if _, ok := tx.folders[p.RootFolderID]; !ok {
		t.Error("the root folder came back with a different id")
	}
}

func TestDeleteBoxRefusesWhenTagged(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "tagged")
	apply(t, tx, ActionCreateBox, raw)
	tag, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	tx.InsertTag(tag)
	tx.AddBoxTag(p.BoxID, tag.ID)

	action, inverse := inverseOf(t, ActionCreateBox, raw)
	err := apply(t, tx, action, inverse)
	if err == nil {
		t.Fatal("deleting a tagged box succeeded")
	}
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want it to wrap ErrStaleBefore", err)
	}
}

// delete_box removes the one folder its payload records, not every folder of
// the box. A box that somehow holds a subfolder has to fail loudly: the
// inverse only re-creates the root, so a bulk delete would be an op its own
// undo cannot reverse.
func TestDeleteBoxRefusesWhenTheRootHasSubfolders(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "nested")
	apply(t, tx, ActionCreateBox, raw)

	sub, err := domain.NewFolder(domain.NewFolderID(), p.RootFolderID, "2019")
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.InsertFolder(sub); err != nil {
		t.Fatal(err)
	}

	action, inverse := inverseOf(t, ActionCreateBox, raw)
	err = apply(t, tx, action, inverse)
	if err == nil {
		t.Fatal("deleting a box with a subfolder succeeded; the subfolder is gone and undo cannot bring it back")
	}
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want it to wrap ErrStaleBefore", err)
	}
}

func TestUpdateBoxApplyAndInverse(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "old name")
	apply(t, tx, ActionCreateBox, raw)

	start, _ := domain.ParseLocalDate("2019-08")
	up := UpdateBoxPayload{
		BoxID:  p.BoxID,
		Before: BoxFields{Name: "old name"},
		After:  BoxFields{Name: "new name", Description: "with a note", StartAt: start},
	}
	upRaw, _ := Marshal(up)

	if err := apply(t, tx, ActionUpdateBox, upRaw); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	b, _, _ := tx.GetBox(p.BoxID)
	if b.Name != "new name" || b.Description != "with a note" || b.StartAt.String() != "2019-08" {
		t.Errorf("after update: %+v", b)
	}
	if b.NameFold != domain.Fold("new name") {
		t.Errorf("NameFold was not regenerated: %q", b.NameFold)
	}

	action, invRaw := inverseOf(t, ActionUpdateBox, upRaw)
	if action != ActionUpdateBox {
		t.Errorf("inverse of update_box = %q, want update_box", action)
	}
	if err := apply(t, tx, action, invRaw); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	b, _, _ = tx.GetBox(p.BoxID)
	if b.Name != "old name" || b.Description != "" || !b.StartAt.IsZero() {
		t.Errorf("after undo: %+v, want the original state back", b)
	}
}

func TestUpdateBoxRefusesAStaleBefore(t *testing.T) {
	// spec 2.5, strict mode: "I thought the undo worked but it only restored
	// half of it" is far more dangerous in archiving software than "the undo
	// failed".
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "actual name")
	apply(t, tx, ActionCreateBox, raw)

	stale := UpdateBoxPayload{
		BoxID:  p.BoxID,
		Before: BoxFields{Name: "something else entirely"},
		After:  BoxFields{Name: "whatever"},
	}
	staleRaw, _ := Marshal(stale)

	err := apply(t, tx, ActionUpdateBox, staleRaw)
	if err == nil {
		t.Fatal("an update with a stale before-state was accepted")
	}
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want it to wrap ErrStaleBefore", err)
	}
	b, _, _ := tx.GetBox(p.BoxID)
	if b.Name != "actual name" {
		t.Errorf("the box was modified anyway: %q", b.Name)
	}
}

func TestUpdateBoxIgnoresUpdatedAtWhenComparing(t *testing.T) {
	// spec 1.5.1 ④: updated_at is maintained derived state, not semantic
	// state. If the before-comparison included it, every undo would fail.
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "name")
	apply(t, tx, ActionCreateBox, raw)

	// Simulate time passing and a touch bumping updated_at.
	b := tx.boxes[p.BoxID]
	b.UpdatedAt = opTime.Add(72 * time.Hour)
	tx.boxes[p.BoxID] = b

	up := UpdateBoxPayload{
		BoxID:  p.BoxID,
		Before: BoxFields{Name: "name"},
		After:  BoxFields{Name: "renamed"},
	}
	upRaw, _ := Marshal(up)
	if err := apply(t, tx, ActionUpdateBox, upRaw); err != nil {
		t.Fatalf("a bumped updated_at blocked a legitimate update: %v", err)
	}
}

func TestBoxTargets(t *testing.T) {
	// Invariant 4: op_boxes must be reconstructible from the payload alone.
	p, raw := createBoxPayload(t, "targeted")

	for _, action := range []Action{ActionCreateBox, ActionDeleteBox, ActionUpdateBox} {
		payload := raw
		if action == ActionUpdateBox {
			payload, _ = Marshal(UpdateBoxPayload{BoxID: p.BoxID})
		}
		h, _ := Lookup(action)
		got, err := h.Targets(payload)
		if err != nil {
			t.Fatalf("Targets(%s): %v", action, err)
		}
		if len(got.Boxes) != 1 || got.Boxes[0] != p.BoxID {
			t.Errorf("Targets(%s).Boxes = %v, want [%v]", action, got.Boxes, p.BoxID)
		}
	}
}
