package library

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
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

	tree, err := NewTagTree(tags, nil)
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

func TestTagTreeLookup(t *testing.T) {
	tree, ids := buildTree(t)

	if got, ok := tree.ByID(ids["云南"]); !ok || got.Name != "云南" {
		t.Errorf("ByID = %+v ok=%v", got, ok)
	}
	// spec 1.6: names are globally unique, so name lookup is unambiguous —
	// that is the whole point of the constraint.
	if got, ok := tree.ByName("大理"); !ok || got.ID != ids["大理"] {
		t.Errorf("ByName = %+v ok=%v", got, ok)
	}
	// Lookup goes through the same fold function the unique index is built on
	// (spec 4.7), and folding deliberately does not trim.
	if _, ok := tree.ByName("大理 "); ok {
		t.Error("ByName matched a name with trailing space; folding must not trim")
	}
	if _, ok := tree.ByName("没有这个"); ok {
		t.Error("ByName found a tag that does not exist")
	}
	if tree.Len() != 7 {
		t.Errorf("Len = %d, want 7", tree.Len())
	}
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

func TestDescendantsIncludesSelf(t *testing.T) {
	// spec 4.3: selecting 人物 should return everyone's photos — which
	// includes anything hanging directly off 人物 itself.
	tree, ids := buildTree(t)

	got := tree.Descendants(ids["云南"])
	if len(got) != 3 {
		t.Fatalf("Descendants(云南) has %d entries, want 3 (self + 大理 + 丽江)", len(got))
	}
	found := map[domain.TagID]bool{}
	for _, id := range got {
		found[id] = true
	}
	for _, want := range []string{"云南", "大理", "丽江"} {
		if !found[ids[want]] {
			t.Errorf("Descendants(云南) is missing %s", want)
		}
	}

	if got := tree.Descendants(ids["地点"]); len(got) != 5 {
		t.Errorf("Descendants(地点) has %d entries, want 5", len(got))
	}
	if got := tree.Descendants(ids["爸"]); len(got) != 1 {
		t.Errorf("Descendants of a leaf has %d entries, want just itself", len(got))
	}
}

func TestPath(t *testing.T) {
	tree, ids := buildTree(t)
	got := names(tree.Path(ids["大理"]))
	want := []string{"地点", "云南", "大理"}
	if len(got) != len(want) {
		t.Fatalf("Path = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Path = %v, want %v", got, want)
		}
	}
}

func TestWouldCycle(t *testing.T) {
	// Moving a node under its own descendant creates a loop, and a loop in the
	// tag tree makes Descendants recurse forever. The check belongs here, at
	// command time, because Apply is not allowed to make decisions.
	tree, ids := buildTree(t)

	if !tree.WouldCycle(ids["地点"], ids["大理"]) {
		t.Error("moving 地点 under its own grandchild was not detected as a cycle")
	}
	if !tree.WouldCycle(ids["云南"], ids["云南"]) {
		t.Error("moving a node under itself was not detected as a cycle")
	}
	if tree.WouldCycle(ids["云南"], ids["人物"]) {
		t.Error("a legitimate move was reported as a cycle")
	}
}

func TestTagTreeIncrementalMaintenance(t *testing.T) {
	tree, ids := buildTree(t)

	// insert
	newID := domain.NewTagID()
	parent := ids["人物"]
	mom, _ := domain.NewTag(newID, &parent, "妈")
	if err := tree.Insert(mom); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if len(tree.Children(parent)) != 2 {
		t.Error("Insert did not attach the node to its parent")
	}

	// rename
	renamed, _ := domain.NewTag(newID, &parent, "母亲")
	if err := tree.Update(renamed); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, ok := tree.ByName("妈"); ok {
		t.Error("the old name still resolves after a rename")
	}
	if _, ok := tree.ByName("母亲"); !ok {
		t.Error("the new name does not resolve after a rename")
	}

	// move
	places := ids["地点"]
	moved, _ := domain.NewTag(newID, &places, "母亲")
	if err := tree.Update(moved); err != nil {
		t.Fatalf("Update (move): %v", err)
	}
	if len(tree.Children(parent)) != 1 {
		t.Error("the node was not detached from its old parent")
	}
	if len(tree.Children(places)) != 3 {
		t.Error("the node was not attached to its new parent")
	}

	// remove
	if err := tree.Remove(newID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := tree.ByID(newID); ok {
		t.Error("the node survived Remove")
	}
	if len(tree.Children(places)) != 2 {
		t.Error("Remove did not detach the node from its parent")
	}
}

func TestTagTreeRejectsADuplicateName(t *testing.T) {
	tree, _ := buildTree(t)
	dup, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	if err := tree.Insert(dup); err == nil {
		t.Error("the tree accepted a duplicate name; global uniqueness is a lie")
	}
}

func TestCountsAggregateUpTheTree(t *testing.T) {
	// spec 1.6: the selector shows 人物 (1,204), which is the subtree total.
	tree, ids := buildTree(t)
	tree.SetDirectCount(ids["大理"], 10)
	tree.SetDirectCount(ids["丽江"], 5)
	tree.SetDirectCount(ids["云南"], 1)
	tree.SetDirectCount(ids["四川"], 3)

	if got := tree.Count(ids["云南"]); got != 16 {
		t.Errorf("Count(云南) = %d, want 16", got)
	}
	if got := tree.Count(ids["地点"]); got != 19 {
		t.Errorf("Count(地点) = %d, want 19", got)
	}
	if got := tree.Count(ids["人物"]); got != 0 {
		t.Errorf("Count(人物) = %d, want 0", got)
	}

	tree.AddDirectCount(ids["大理"], -4)
	if got := tree.Count(ids["地点"]); got != 15 {
		t.Errorf("after AddDirectCount, Count(地点) = %d, want 15", got)
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
	if l.Tags().Len() != 0 {
		t.Errorf("a fresh library has %d tags, want 0", l.Tags().Len())
	}
}
