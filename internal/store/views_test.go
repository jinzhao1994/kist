package store

import (
	"database/sql"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

var testTime = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

// viewTx runs fn inside a transaction against a migrated in-memory library.
func viewTx(t *testing.T, db *sql.DB, ts time.Time, fn func(*ViewTx)) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	fn(NewViewTx(tx, ts))
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func mustBox(t *testing.T, name string) domain.Box {
	t.Helper()
	b, err := domain.NewBox(domain.NewBoxID(), name, "")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestInsertAndGetBox(t *testing.T) {
	db := migrated(t)
	b := mustBox(t, "云南 2019")

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.InsertBox(b); err != nil {
			t.Fatalf("InsertBox: %v", err)
		}
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		got, ok, err := vtx.GetBox(b.ID)
		if err != nil || !ok {
			t.Fatalf("GetBox: %v ok=%v", err, ok)
		}
		if got.Name != b.Name || got.NameFold != b.NameFold {
			t.Errorf("GetBox = %+v, want name %q", got, b.Name)
		}
		// spec 2.3.1: timestamps come from the op, not the wall clock.
		if !got.CreatedAt.Equal(testTime) || !got.UpdatedAt.Equal(testTime) {
			t.Errorf("timestamps = %v / %v, want both %v", got.CreatedAt, got.UpdatedAt, testTime)
		}
	})
}

func TestGetBoxMissing(t *testing.T) {
	db := migrated(t)
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if _, ok, err := vtx.GetBox(domain.NewBoxID()); err != nil || ok {
			t.Errorf("GetBox of a missing box = ok %v, err %v", ok, err)
		}
	})
}

func TestUpdateBoxLeavesBothTimestampsAlone(t *testing.T) {
	// UpdateBox writes semantic fields only. created_at is immutable, and
	// updated_at is the write path's business — whether it moves depends on
	// the op's source (spec 1.5.1 ②), which a handler does not get to know.
	db := migrated(t)
	b := mustBox(t, "before")
	viewTx(t, db, testTime, func(vtx *ViewTx) { vtx.InsertBox(b) })

	later := testTime.Add(48 * time.Hour)
	renamed, _ := domain.NewBox(b.ID, "after", "")
	viewTx(t, db, later, func(vtx *ViewTx) {
		if err := vtx.UpdateBox(renamed); err != nil {
			t.Fatalf("UpdateBox: %v", err)
		}
	})

	viewTx(t, db, later, func(vtx *ViewTx) {
		got, _, _ := vtx.GetBox(b.ID)
		if got.Name != "after" {
			t.Errorf("Name = %q, want %q", got.Name, "after")
		}
		if !got.CreatedAt.Equal(testTime) {
			t.Errorf("CreatedAt = %v, want it unchanged at %v", got.CreatedAt, testTime)
		}
		if !got.UpdatedAt.Equal(testTime) {
			t.Errorf("UpdatedAt = %v; UpdateBox must not move it, TouchBoxes does", got.UpdatedAt)
		}
	})

	// TouchBoxes is what moves it.
	viewTx(t, db, later, func(vtx *ViewTx) {
		if err := vtx.TouchBoxes([]domain.BoxID{b.ID}); err != nil {
			t.Fatal(err)
		}
	})
	viewTx(t, db, later, func(vtx *ViewTx) {
		got, _, _ := vtx.GetBox(b.ID)
		if !got.UpdatedAt.Equal(later) {
			t.Errorf("after TouchBoxes UpdatedAt = %v, want %v", got.UpdatedAt, later)
		}
	})
}

func TestBoxDatesRoundTrip(t *testing.T) {
	db := migrated(t)
	b := mustBox(t, "trip")
	b.StartAt, _ = domain.ParseLocalDate("2019-08")
	b.EndAt, _ = domain.ParseLocalDate("2019-08-12T14:30")

	viewTx(t, db, testTime, func(vtx *ViewTx) { vtx.InsertBox(b) })
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		got, _, _ := vtx.GetBox(b.ID)
		if got.StartAt.String() != "2019-08" || got.EndAt.String() != "2019-08-12T14:30" {
			t.Errorf("dates = %q / %q", got.StartAt, got.EndAt)
		}
		if got.StartAt.Precision() != domain.PrecMonth {
			t.Errorf("StartAt precision = %v, want month", got.StartAt.Precision())
		}
	})
}

func TestDeleteBoxNeedsItsFoldersGoneFirst(t *testing.T) {
	// Foreign keys are on (M1a). Deleting a box while its root folder still
	// points at it must fail rather than silently orphan the folder — which is
	// exactly why delete_box's handler removes folders explicitly.
	db := migrated(t)
	b := mustBox(t, "doomed")
	root := domain.NewRootFolder(domain.NewFolderID(), b.ID)

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertFolder(root)
	})

	tx, _ := db.Begin()
	if err := NewViewTx(tx, testTime).DeleteBox(b.ID); err == nil {
		t.Error("deleting a box with a folder still attached succeeded")
	}
	tx.Rollback()

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.DeleteFoldersOfBox(b.ID); err != nil {
			t.Fatalf("DeleteFoldersOfBox: %v", err)
		}
		if err := vtx.DeleteBox(b.ID); err != nil {
			t.Fatalf("DeleteBox: %v", err)
		}
	})
}

func TestTagCRUD(t *testing.T) {
	db := migrated(t)
	people, _ := domain.NewTag(domain.NewTagID(), nil, "人物")
	child, _ := domain.NewTag(domain.NewTagID(), &people.ID, "爸")

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.InsertTag(people); err != nil {
			t.Fatalf("InsertTag: %v", err)
		}
		if err := vtx.InsertTag(child); err != nil {
			t.Fatalf("InsertTag child: %v", err)
		}
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		got, ok, err := vtx.GetTag(child.ID)
		if err != nil || !ok {
			t.Fatalf("GetTag: %v ok=%v", err, ok)
		}
		if got.ParentID == nil || *got.ParentID != people.ID {
			t.Errorf("ParentID = %v, want %v", got.ParentID, people.ID)
		}

		n, err := vtx.CountTagChildren(people.ID)
		if err != nil || n != 1 {
			t.Errorf("CountTagChildren = %d (%v), want 1", n, err)
		}
		if n, _ := vtx.CountTagChildren(child.ID); n != 0 {
			t.Errorf("a leaf reported %d children", n)
		}
	})
}

func TestTagNamesStayGloballyUnique(t *testing.T) {
	// The schema enforces it (M1a Task 9); this checks the write path surfaces
	// the violation instead of swallowing it.
	db := migrated(t)
	a, _ := domain.NewTag(domain.NewTagID(), nil, "护照")
	viewTx(t, db, testTime, func(vtx *ViewTx) { vtx.InsertTag(a) })

	parent := a.ID
	b, _ := domain.NewTag(domain.NewTagID(), &parent, "护照")
	tx, _ := db.Begin()
	if err := NewViewTx(tx, testTime).InsertTag(b); err == nil {
		t.Error("a duplicate tag name was accepted under a different parent")
	}
	tx.Rollback()
}

func TestBoxTagsCRUD(t *testing.T) {
	db := migrated(t)
	b := mustBox(t, "trip")
	tag, _ := domain.NewTag(domain.NewTagID(), nil, "云南")

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertTag(tag)
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if has, _ := vtx.HasBoxTag(b.ID, tag.ID); has {
			t.Error("HasBoxTag before adding = true")
		}
		if err := vtx.AddBoxTag(b.ID, tag.ID, "user"); err != nil {
			t.Fatalf("AddBoxTag: %v", err)
		}
		if has, _ := vtx.HasBoxTag(b.ID, tag.ID); !has {
			t.Error("HasBoxTag after adding = false")
		}
		if n, _ := vtx.CountTagUsage(tag.ID); n != 1 {
			t.Errorf("CountTagUsage = %d, want 1", n)
		}
		if n, _ := vtx.CountBoxTags(b.ID); n != 1 {
			t.Errorf("CountBoxTags = %d, want 1", n)
		}
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.RemoveBoxTag(b.ID, tag.ID); err != nil {
			t.Fatalf("RemoveBoxTag: %v", err)
		}
		if has, _ := vtx.HasBoxTag(b.ID, tag.ID); has {
			t.Error("HasBoxTag after removing = true")
		}
	})
}

// Adding the same tag twice is not an error: a batch op may name a box that
// already carries the tag, and one op means one outcome, not a failure.
func TestAddBoxTagIsIdempotent(t *testing.T) {
	db := migrated(t)
	b := mustBox(t, "trip")
	tag, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertTag(tag)
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.AddBoxTag(b.ID, tag.ID, "user"); err != nil {
			t.Fatal(err)
		}
		if err := vtx.AddBoxTag(b.ID, tag.ID, "user"); err != nil {
			t.Errorf("adding an existing box tag: %v", err)
		}
		if n, _ := vtx.CountBoxTags(b.ID); n != 1 {
			t.Errorf("CountBoxTags = %d after adding twice, want 1", n)
		}
	})
}

func TestTouchBoxes(t *testing.T) {
	// spec 1.5.1 ③: propagation is per op with a deduplicated target set, not
	// per file in a loop.
	db := migrated(t)
	a, b := mustBox(t, "a"), mustBox(t, "b")
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(a)
		vtx.InsertBox(b)
	})

	later := testTime.Add(time.Hour)
	viewTx(t, db, later, func(vtx *ViewTx) {
		if err := vtx.TouchBoxes([]domain.BoxID{a.ID, a.ID, b.ID}); err != nil {
			t.Fatalf("TouchBoxes: %v", err)
		}
	})

	viewTx(t, db, later, func(vtx *ViewTx) {
		for _, id := range []domain.BoxID{a.ID, b.ID} {
			got, _, _ := vtx.GetBox(id)
			if !got.UpdatedAt.Equal(later) {
				t.Errorf("box %s UpdatedAt = %v, want %v", id, got.UpdatedAt, later)
			}
			if !got.CreatedAt.Equal(testTime) {
				t.Errorf("box %s CreatedAt moved to %v", id, got.CreatedAt)
			}
		}
	})
}

func TestTouchBoxesIgnoresMissingRows(t *testing.T) {
	// Undoing a create deletes the box; the propagation pass that follows must
	// not fail because its target is gone.
	db := migrated(t)
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.TouchBoxes([]domain.BoxID{domain.NewBoxID()}); err != nil {
			t.Errorf("TouchBoxes on a missing box: %v", err)
		}
	})
}

func TestClearViewsLeavesOpsAndBlobs(t *testing.T) {
	// Invariant 1: ops and blobs are the authority. Rebuild wipes everything
	// else and replays; if it ever wiped ops, rebuild would be data loss.
	db := migrated(t)
	b := mustBox(t, "survivor")
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertFolder(domain.NewRootFolder(domain.NewFolderID(), b.ID))
	})
	appendOp(t, db, sampleOp())
	if _, err := db.Exec(`INSERT INTO blobs (key, sha256, size) VALUES ('sha256:' || hex(randomblob(32)), hex(randomblob(32)), 1)`); err != nil {
		t.Fatal(err)
	}

	tx, _ := db.Begin()
	if err := ClearViews(tx); err != nil {
		t.Fatalf("ClearViews: %v", err)
	}
	tx.Commit()

	for _, table := range []string{"boxes", "folders", "tags", "box_tags"} {
		var n int
		db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n)
		if n != 0 {
			t.Errorf("%s still has %d rows after ClearViews", table, n)
		}
	}
	for _, table := range []string{"ops", "blobs"} {
		var n int
		db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n)
		if n == 0 {
			t.Errorf("ClearViews emptied %s, which is authoritative data", table)
		}
	}
}

func TestListBoxes(t *testing.T) {
	db := migrated(t)
	early, late := mustBox(t, "早"), mustBox(t, "晚")
	viewTx(t, db, testTime, func(vtx *ViewTx) { vtx.InsertBox(early) })
	viewTx(t, db, testTime.Add(time.Hour), func(vtx *ViewTx) { vtx.InsertBox(late) })

	got, err := ListBoxes(db, ListBoxesOptions{})
	if err != nil {
		t.Fatalf("ListBoxes: %v", err)
	}
	if len(got) != 2 || got[0].ID != late.ID {
		t.Errorf("default order should be most-recently-active first, got %q then %q", got[0].Name, got[1].Name)
	}

	byName, err := ListBoxes(db, ListBoxesOptions{SortByName: true})
	if err != nil {
		t.Fatalf("ListBoxes by name: %v", err)
	}
	// spec 4.8: byte order. 早 is U+65E9 and 晚 is U+665A, so 早 comes first.
	// Under a pinyin collation it would be the other way round (wan < zao),
	// which is what makes this a real assertion rather than a tautology: it
	// fails the moment a locale-aware collation sneaks in.
	if byName[0].Name != "早" {
		t.Errorf("sorted by name gave %q first; byte order puts 早 (U+65E9) before 晚 (U+665A)", byName[0].Name)
	}
}

func TestTagDirectCounts(t *testing.T) {
	db := migrated(t)
	b := mustBox(t, "trip")
	tag, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertTag(tag)
		vtx.AddBoxTag(b.ID, tag.ID, "user")
	})

	counts, err := TagDirectCounts(db)
	if err != nil {
		t.Fatalf("TagDirectCounts: %v", err)
	}
	if counts[tag.ID] != 1 {
		t.Errorf("count for the tag = %d, want 1", counts[tag.ID])
	}
}
