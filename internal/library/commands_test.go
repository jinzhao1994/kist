package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestCreateBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()

	start, _ := domain.ParseLocalDate("2019-08")
	b, err := l.CreateBox(ctx, "云南 2019", "the long way round", start, domain.LocalDate{})
	if err != nil {
		t.Fatalf("CreateBox: %v", err)
	}
	if b.ID.IsZero() || b.Name != "云南 2019" || b.StartAt.String() != "2019-08" {
		t.Errorf("box = %+v", b)
	}
	if b.CreatedAt.IsZero() {
		t.Error("CreatedAt was not filled in from the op")
	}

	// Exactly one root folder, per spec 1.4 and the partial unique index.
	var folders int
	l.db.QueryRow(`SELECT count(*) FROM folders WHERE box_id = ?`, b.ID).Scan(&folders)
	if folders != 1 {
		t.Errorf("the box has %d folders, want exactly 1", folders)
	}
}

func TestCreateBoxRejectsABadName(t *testing.T) {
	l := openTestLibrary(t)
	if _, err := l.CreateBox(context.Background(), "  ", "", domain.LocalDate{}, domain.LocalDate{}); err == nil {
		t.Error("a blank box name was accepted")
	}
	ops, _ := l.Ops(context.Background(), store.ListOpsOptions{Limit: 10})
	if len(ops) != 0 {
		t.Errorf("a rejected command still wrote %d op(s)", len(ops))
	}
}

func TestUpdateBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "before", "", domain.LocalDate{}, domain.LocalDate{})

	end, _ := domain.ParseLocalDate("2019-08-20")
	got, err := l.UpdateBox(ctx, b.ID, oplog.BoxFields{Name: "after", Description: "noted", EndAt: end})
	if err != nil {
		t.Fatalf("UpdateBox: %v", err)
	}
	if got.Name != "after" || got.Description != "noted" || got.EndAt.String() != "2019-08-20" {
		t.Errorf("box = %+v", got)
	}
	if !got.CreatedAt.Equal(b.CreatedAt) {
		t.Errorf("CreatedAt moved from %v to %v", b.CreatedAt, got.CreatedAt)
	}
	if !got.UpdatedAt.After(b.UpdatedAt) {
		t.Errorf("UpdatedAt did not move: %v then %v", b.UpdatedAt, got.UpdatedAt)
	}
}

func TestUpdateBoxOnAMissingBox(t *testing.T) {
	l := openTestLibrary(t)
	_, err := l.UpdateBox(context.Background(), domain.NewBoxID(), oplog.BoxFields{Name: "x"})
	if !errors.Is(err, ErrNoSuchBox) {
		t.Errorf("error = %v, want ErrNoSuchBox", err)
	}
}

func TestCreateTagAndTree(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()

	places, err := l.CreateTag(ctx, "地点", nil)
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	yunnan, err := l.CreateTag(ctx, "云南", &places.ID)
	if err != nil {
		t.Fatalf("CreateTag child: %v", err)
	}

	// The in-memory tree must track the database without a reload.
	if got, ok := l.Tags().ByName("云南"); !ok || got.ID != yunnan.ID {
		t.Error("the tag tree does not know about 云南")
	}
	if kids := l.Tags().Children(places.ID); len(kids) != 1 {
		t.Errorf("地点 has %d children in the tree, want 1", len(kids))
	}
}

func TestCreateTagRejectsADuplicateName(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	parent, _ := l.CreateTag(ctx, "地点", nil)

	if _, err := l.CreateTag(ctx, "云南", &parent.ID); err != nil {
		t.Fatal(err)
	}
	// spec 1.6: the hierarchy is not a namespace, so this collides even under
	// a different parent.
	if _, err := l.CreateTag(ctx, "云南", nil); !errors.Is(err, ErrTagNameTaken) {
		t.Errorf("error = %v, want ErrTagNameTaken", err)
	}
}

func TestCreateTagFoldsBeforeCheckingUniqueness(t *testing.T) {
	// Folding is what makes "unique" true rather than a claim (spec 1.6).
	l := openTestLibrary(t)
	ctx := context.Background()
	if _, err := l.CreateTag(ctx, "Passport", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := l.CreateTag(ctx, "passport", nil); !errors.Is(err, ErrTagNameTaken) {
		t.Errorf("a case variant was accepted: %v", err)
	}
}

func TestRenameAndMoveTag(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	places, _ := l.CreateTag(ctx, "地点", nil)
	people, _ := l.CreateTag(ctx, "人物", nil)
	yunnan, _ := l.CreateTag(ctx, "云南", &places.ID)

	if err := l.RenameTag(ctx, yunnan.ID, "云南省"); err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if _, ok := l.Tags().ByName("云南省"); !ok {
		t.Error("the tree did not pick up the rename")
	}
	if _, ok := l.Tags().ByName("云南"); ok {
		t.Error("the old name still resolves")
	}

	if err := l.MoveTag(ctx, yunnan.ID, &people.ID); err != nil {
		t.Fatalf("MoveTag: %v", err)
	}
	if kids := l.Tags().Children(people.ID); len(kids) != 1 {
		t.Errorf("人物 has %d children after the move, want 1", len(kids))
	}
	if kids := l.Tags().Children(places.ID); len(kids) != 0 {
		t.Errorf("地点 still has %d children after the move", len(kids))
	}
}

func TestMoveTagRefusesACycle(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	places, _ := l.CreateTag(ctx, "地点", nil)
	yunnan, _ := l.CreateTag(ctx, "云南", &places.ID)

	if err := l.MoveTag(ctx, places.ID, &yunnan.ID); !errors.Is(err, ErrTagCycle) {
		t.Errorf("error = %v, want ErrTagCycle", err)
	}
	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 100})
	for _, o := range ops {
		if o.Action == oplog.ActionMoveTag {
			t.Error("a rejected move still wrote an op")
		}
	}
}

func TestDeleteTag(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "临时", nil)

	if err := l.DeleteTag(ctx, tag.ID); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	if _, ok := l.Tags().ByID(tag.ID); ok {
		t.Error("the tree still knows about the deleted tag")
	}
}

func TestDeleteTagRefusesWhenInUse(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	b, _ := l.CreateBox(ctx, "trip", "", domain.LocalDate{}, domain.LocalDate{})
	if _, err := l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID}); err != nil {
		t.Fatal(err)
	}

	if err := l.DeleteTag(ctx, tag.ID); !errors.Is(err, oplog.ErrTagInUse) {
		t.Errorf("error = %v, want ErrTagInUse", err)
	}
}

func TestTagBoxes(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	var boxes []domain.BoxID
	for _, n := range []string{"一", "二", "三"} {
		b, _ := l.CreateBox(ctx, n, "", domain.LocalDate{}, domain.LocalDate{})
		boxes = append(boxes, b.ID)
	}

	n, err := l.TagBoxes(ctx, tag.ID, boxes)
	if err != nil {
		t.Fatalf("TagBoxes: %v", err)
	}
	if n != 3 {
		t.Errorf("TagBoxes changed %d boxes, want 3", n)
	}

	// One op, three targets (spec 2.2.1).
	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 1})
	if ops[0].Action != oplog.ActionTagBoxes {
		t.Fatalf("last op is %q", ops[0].Action)
	}
	var targets int
	l.db.QueryRow(`SELECT count(*) FROM op_boxes WHERE op_id = ?`, ops[0].ID).Scan(&targets)
	if targets != 3 {
		t.Errorf("op_boxes has %d rows for the batch, want 3", targets)
	}

	// The count cache followed along.
	if got := l.Tags().Count(tag.ID); got != 3 {
		t.Errorf("tag count = %d, want 3", got)
	}
}

func TestTagBoxesSkipsBoxesThatAlreadyHaveIt(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	a, _ := l.CreateBox(ctx, "a", "", domain.LocalDate{}, domain.LocalDate{})
	b, _ := l.CreateBox(ctx, "b", "", domain.LocalDate{}, domain.LocalDate{})

	l.TagBoxes(ctx, tag.ID, []domain.BoxID{a.ID})
	n, err := l.TagBoxes(ctx, tag.ID, []domain.BoxID{a.ID, b.ID})
	if err != nil {
		t.Fatalf("TagBoxes: %v", err)
	}
	if n != 1 {
		t.Errorf("TagBoxes reported %d changes, want 1 (a already had the tag)", n)
	}

	// Ops does not carry payloads (spec 2.2.1); read the whole row.
	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 1})
	row, ok, err := store.GetOp(l.db, ops[0].ID)
	if err != nil || !ok {
		t.Fatalf("GetOp: %v ok=%v", err, ok)
	}
	p, err := oplog.Payload[oplog.TagBoxesPayload](row.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.BoxIDs) != 1 || p.BoxIDs[0] != b.ID {
		t.Errorf("the payload names %v; it must name only what actually changed", p.BoxIDs)
	}
}

func TestTagBoxesWritesNoOpWhenNothingChanges(t *testing.T) {
	// An op that changes nothing is noise in the history and undoes to
	// nothing. Better to report zero.
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	b, _ := l.CreateBox(ctx, "a", "", domain.LocalDate{}, domain.LocalDate{})
	l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})

	before, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 100})
	n, err := l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})
	if err != nil {
		t.Fatalf("TagBoxes: %v", err)
	}
	if n != 0 {
		t.Errorf("TagBoxes reported %d changes, want 0", n)
	}
	after, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 100})
	if len(after) != len(before) {
		t.Errorf("a no-op tagging wrote %d extra op(s)", len(after)-len(before))
	}
}

func TestUntagBoxes(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	b, _ := l.CreateBox(ctx, "a", "", domain.LocalDate{}, domain.LocalDate{})
	l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})

	n, err := l.UntagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})
	if err != nil {
		t.Fatalf("UntagBoxes: %v", err)
	}
	if n != 1 {
		t.Errorf("UntagBoxes reported %d changes, want 1", n)
	}
	if got := l.Tags().Count(tag.ID); got != 0 {
		t.Errorf("tag count = %d after untagging, want 0", got)
	}
}

func TestResolveBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "云南 2019", "", domain.LocalDate{}, domain.LocalDate{})

	for _, ref := range []string{b.ID.String(), b.ID.String()[:8], "云南 2019"} {
		got, err := l.ResolveBox(ctx, ref)
		if err != nil {
			t.Errorf("ResolveBox(%q): %v", ref, err)
			continue
		}
		if got.ID != b.ID {
			t.Errorf("ResolveBox(%q) found %s, want %s", ref, got.ID, b.ID)
		}
	}

	if _, err := l.ResolveBox(ctx, "没有这个 box"); !errors.Is(err, ErrNoSuchBox) {
		t.Errorf("error = %v, want ErrNoSuchBox", err)
	}
}

func TestResolveBoxReportsAmbiguity(t *testing.T) {
	// Box names are not unique (there is no unique index on boxes.name_fold),
	// so picking one arbitrarily would silently act on the wrong archive.
	l := openTestLibrary(t)
	ctx := context.Background()
	l.CreateBox(ctx, "重名", "", domain.LocalDate{}, domain.LocalDate{})
	l.CreateBox(ctx, "重名", "", domain.LocalDate{}, domain.LocalDate{})

	_, err := l.ResolveBox(ctx, "重名")
	if !errors.Is(err, ErrAmbiguousRef) {
		t.Fatalf("error = %v, want ErrAmbiguousRef", err)
	}
	// The message must let the user act on it.
	if !strings.Contains(err.Error(), "重名") {
		t.Errorf("error = %q, want it to name the candidates", err)
	}
}

func TestResolveTag(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "护照", nil)

	got, err := l.ResolveTag("护照")
	if err != nil || got.ID != tag.ID {
		t.Errorf("ResolveTag by name = %+v, %v", got, err)
	}
	if got, err := l.ResolveTag(tag.ID.String()); err != nil || got.ID != tag.ID {
		t.Errorf("ResolveTag by id = %+v, %v", got, err)
	}
	if _, err := l.ResolveTag("没有"); !errors.Is(err, ErrNoSuchTag) {
		t.Errorf("error = %v, want ErrNoSuchTag", err)
	}
}
