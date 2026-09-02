package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

// buildTree makes a tree without a database:
//
//	地点            人物
//	├── 云南        └── 爸
//	│   ├── 大理
//	│   └── 丽江
//	└── 四川
func buildTree(t *testing.T) (*TagTree, map[string]domain.TagID) {
	t.Helper()
	ids := map[string]domain.TagID{}
	var tags []domain.Tag

	add := func(name, parent string) {
		id := domain.NewTagID()
		ids[name] = id
		var p *domain.TagID
		if parent != "" {
			pid := ids[parent]
			p = &pid
		}
		tag, err := domain.NewTag(id, p, name)
		if err != nil {
			t.Fatal(err)
		}
		tags = append(tags, tag)
	}

	add("地点", "")
	add("人物", "")
	add("云南", "地点")
	add("四川", "地点")
	add("大理", "云南")
	add("丽江", "云南")
	add("爸", "人物")

	tree, err := NewTagTree(tags)
	if err != nil {
		t.Fatalf("NewTagTree: %v", err)
	}
	return tree, ids
}

func names(tags []domain.Tag) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.Name
	}
	return out
}

func TestTagTreeStructure(t *testing.T) {
	tree, ids := buildTree(t)

	if got := names(tree.Roots()); len(got) != 2 {
		t.Errorf("Roots = %v, want two", got)
	}
	if got := names(tree.Children(ids["地点"])); len(got) != 2 {
		t.Errorf("Children(地点) = %v, want two", got)
	}
	if got := names(tree.Children(ids["大理"])); len(got) != 0 {
		t.Errorf("Children(大理) = %v, want none", got)
	}
}

func TestTagTreeRejectsADuplicateName(t *testing.T) {
	dup, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	other, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	if _, err := NewTagTree([]domain.Tag{dup, other}); err == nil {
		t.Error("the tree accepted a duplicate name; global uniqueness is a lie")
	}
}

func TestCountsAggregateUpTheTree(t *testing.T) {
	// spec 1.6: the selector shows 人物 (1,204), which is the subtree total.
	tree, ids := buildTree(t)
	totals := tree.Totals(map[domain.TagID]int{
		ids["大理"]: 10,
		ids["丽江"]: 5,
		ids["云南"]: 1,
		ids["四川"]: 3,
	})

	if got := totals[ids["云南"]]; got != 16 {
		t.Errorf("云南 = %d, want 16", got)
	}
	if got := totals[ids["地点"]]; got != 19 {
		t.Errorf("地点 = %d, want 19", got)
	}
	if got := totals[ids["人物"]]; got != 0 {
		t.Errorf("人物 = %d, want 0", got)
	}
	// A tag nobody used is still in the map, at zero — the selector shows it.
	if _, ok := totals[ids["爸"]]; !ok {
		t.Error("an unused tag is missing from the totals")
	}
}

func TestTagTreeSortsByByteOrder(t *testing.T) {
	// spec 4.8: the in-memory tree and the database must agree, or a listing's
	// order would depend on which code path produced it.
	tree, _ := buildTree(t)
	got := names(tree.Roots())
	// 人 is U+4EBA, 地 is U+5730, so 人物 comes first in byte order.
	if got[0] != "人物" {
		t.Errorf("Roots() = %v; byte order puts 人物 (U+4EBA) before 地点 (U+5730)", got)
	}
}

func TestLoadTagTreeFromStore(t *testing.T) {
	l := openTestLibrary(t)
	tree, totals, err := l.TagTree()
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.Roots(); len(got) != 0 {
		t.Errorf("a fresh library has roots %v, want none", names(got))
	}
	if len(totals) != 0 {
		t.Errorf("a fresh library has %d totals, want none", len(totals))
	}
}

// Two processes share a library (spec 2.7), so a tag tree held in process
// memory is stale the moment anybody else writes. This is the reason the tree
// is loaded per operation rather than kept as a field.
//
// The duplicate is the sharp half. Name uniqueness is checked against the tree
// before an op is written, so that the user gets a sentence instead of a
// constraint violation (spec 1.6) — against a stale tree that check passes,
// the op is written, and the failure surfaces from the unique index inside the
// transaction with nothing useful to say.
func TestATagCreatedByAnotherProcessIsVisible(t *testing.T) {
	first := openTestLibrary(t)
	ctx := t.Context()

	second, err := Open(first.Layout().Root, WithClock(fixedClock()))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	if _, err := first.CreateTag(ctx, "护照", nil); err != nil {
		t.Fatal(err)
	}

	if _, err := second.ResolveTag("护照"); err != nil {
		t.Errorf("the other process's tag does not resolve: %v", err)
	}
	if _, err := second.CreateTag(ctx, "护照", nil); !errors.Is(err, ErrTagNameTaken) {
		t.Errorf("a duplicate tag was accepted; err = %v", err)
	}
}

// The helpers below read through the same paths production does, so that a
// test asserting "the tag is gone" is asserting it about the database rather
// than about a copy the test happens to hold.

func tagByName(t *testing.T, l *Library, name string) (domain.Tag, bool) {
	t.Helper()
	tag, ok, err := store.TagByName(l.db, name)
	if err != nil {
		t.Fatal(err)
	}
	return tag, ok
}

func tagByID(t *testing.T, l *Library, id domain.TagID) (domain.Tag, bool) {
	t.Helper()
	tag, ok, err := store.TagByID(l.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return tag, ok
}

func tagChildren(t *testing.T, l *Library, id domain.TagID) []domain.Tag {
	t.Helper()
	tree, _, err := l.TagTree()
	if err != nil {
		t.Fatal(err)
	}
	return tree.Children(id)
}

// tagCount is the subtree total the tag selector would show.
func tagCount(t *testing.T, l *Library, id domain.TagID) int {
	t.Helper()
	_, totals, err := l.TagTree()
	if err != nil {
		t.Fatal(err)
	}
	return totals[id]
}

func tagTotal(t *testing.T, l *Library) int {
	t.Helper()
	tags, err := store.ListTags(l.db)
	if err != nil {
		t.Fatal(err)
	}
	return len(tags)
}
