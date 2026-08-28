package store

import (
	"database/sql"
	"slices"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

// tagFixture builds
//
//	地点            人物
//	├── 云南        └── 爸
//	│   ├── 大理
//	│   └── 丽江
//	└── 四川
func tagFixture(t *testing.T) (*sql.DB, map[string]domain.TagID) {
	t.Helper()
	db := migrated(t)
	ids := map[string]domain.TagID{}
	viewTx(t, db, testTime, func(vtx *ViewTx) {
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
			if err := vtx.InsertTag(tag); err != nil {
				t.Fatal(err)
			}
		}
		add("地点", "")
		add("人物", "")
		add("云南", "地点")
		add("四川", "地点")
		add("大理", "云南")
		add("丽江", "云南")
		add("爸", "人物")
	})
	return db, ids
}

// insertTag adds one tag outside the fixture, for cases that need a specific
// name rather than a specific shape.
func addTag(db *sql.DB, t *testing.T, name string, parent *domain.TagID) error {
	t.Helper()
	tag, err := domain.NewTag(domain.NewTagID(), parent, name)
	if err != nil {
		return err
	}
	var insertErr error
	viewTx(t, db, testTime, func(vtx *ViewTx) { insertErr = vtx.InsertTag(tag) })
	return insertErr
}

func TestTagByName(t *testing.T) {
	db, ids := tagFixture(t)

	got, ok, err := TagByName(db, "云南")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.ID != ids["云南"] {
		t.Errorf("TagByName(云南) = %v, %v", got, ok)
	}
	// Folded the same way the unique index is, so that "Passport" and
	// "passport" find the same tag (spec 1.6, 4.7).
	if err := addTag(db, t, "Passport", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := TagByName(db, "PASSPORT"); err != nil || !ok {
		t.Errorf("a name in another case did not resolve: %v, %v", ok, err)
	}
	if _, ok, err := TagByName(db, "西藏"); err != nil || ok {
		t.Errorf("a name nobody used resolved: %v, %v", ok, err)
	}
}

func TestTagByID(t *testing.T) {
	db, ids := tagFixture(t)

	got, ok, err := TagByID(db, ids["大理"])
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Name != "大理" {
		t.Errorf("TagByID = %v, %v", got, ok)
	}
	if got.ParentID == nil || *got.ParentID != ids["云南"] {
		t.Errorf("parent = %v, want 云南", got.ParentID)
	}
	if _, ok, err := TagByID(db, domain.NewTagID()); err != nil || ok {
		t.Errorf("an id nobody minted resolved: %v, %v", ok, err)
	}
}

// spec 4.3: selecting 人物 returns everyone's photos — which includes anything
// hanging directly off 人物 itself, so the root is part of its own subtree.
func TestTagDescendantsIncludesTheRoots(t *testing.T) {
	db, ids := tagFixture(t)

	got, err := TagDescendants(db, []domain.TagID{ids["云南"]})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"云南", "大理", "丽江"} {
		if !slices.Contains(got, ids[want]) {
			t.Errorf("the subtree of 云南 is missing %s", want)
		}
	}
	if len(got) != 3 {
		t.Errorf("the subtree of 云南 has %d entries, want 3", len(got))
	}

	if got, err := TagDescendants(db, []domain.TagID{ids["地点"]}); err != nil || len(got) != 5 {
		t.Errorf("the subtree of 地点 has %d entries, want 5: %v", len(got), err)
	}
	if got, err := TagDescendants(db, []domain.TagID{ids["爸"]}); err != nil || len(got) != 1 {
		t.Errorf("the subtree of a leaf has %d entries, want just itself: %v", len(got), err)
	}
	if got, err := TagDescendants(db, nil); err != nil || got != nil {
		t.Errorf("no roots gave %v, %v; want nothing", got, err)
	}
}

// Two roots in one subtree must not double-count the part they share. This is
// what UNION buys over UNION ALL, and it is why the caller can pass a filter's
// whole tag set in one query.
func TestTagDescendantsDeduplicatesOverlappingRoots(t *testing.T) {
	db, ids := tagFixture(t)

	got, err := TagDescendants(db, []domain.TagID{ids["地点"], ids["云南"], ids["大理"]})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Errorf("%d entries, want the 5 distinct tags under 地点: %v", len(got), got)
	}
}

func TestTagAncestorsWalksUpToTheRoot(t *testing.T) {
	db, ids := tagFixture(t)

	got, err := TagAncestors(db, ids["大理"])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"大理", "云南", "地点"} {
		if !slices.Contains(got, ids[want]) {
			t.Errorf("the ancestors of 大理 are missing %s", want)
		}
	}
	if len(got) != 3 {
		t.Errorf("%d ancestors, want 3: %v", len(got), got)
	}
	// A root's only ancestor is itself, which is what makes MoveTag's check
	// "is the new parent at or under the tag being moved" one comparison.
	if got, err := TagAncestors(db, ids["人物"]); err != nil || len(got) != 1 {
		t.Errorf("a root has %d ancestors, want just itself: %v", len(got), err)
	}
}

// MoveTag refuses to create a cycle, but a corrupt views table could still
// hold one, and a recursive query that revisits a node it has already seen
// does not terminate. UNION is what stops it — this test hangs forever if
// somebody "optimises" it to UNION ALL.
func TestTagTraversalTerminatesOnACycle(t *testing.T) {
	db, ids := tagFixture(t)

	// Close the loop behind the ORM's back: 地点's parent becomes its own
	// grandchild. No command can do this; the point is that the query
	// survives a database that has it.
	if _, err := db.Exec(`UPDATE tags SET parent_id = ? WHERE id = ?`, ids["大理"], ids["地点"]); err != nil {
		t.Fatal(err)
	}

	got, err := TagDescendants(db, []domain.TagID{ids["地点"]})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Errorf("descendants through a cycle = %d entries, want the 5 distinct tags", len(got))
	}
	up, err := TagAncestors(db, ids["大理"])
	if err != nil {
		t.Fatal(err)
	}
	if len(up) != 3 {
		t.Errorf("ancestors through a cycle = %d entries, want 3", len(up))
	}
}
