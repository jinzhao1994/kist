package store

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// testInstant reads a fixture's human-readable date as the unix second the
// column holds. UTC and fixed, so a test means the same thing everywhere
// (spec 1.5.2).
func testInstant(t *testing.T, s string) domain.Instant {
	t.Helper()
	d, err := domain.ParseLocalDate(s)
	if err != nil {
		t.Fatal(err)
	}
	start, err := time.Parse("2006-01-02T15:04", d.Start())
	if err != nil {
		t.Fatal(err)
	}
	return domain.NewInstant(start.UnixNano())
}

// putFile inserts one file, optionally with a captured_at, and returns its id.
func putFile(t *testing.T, db *sql.DB, folder domain.FolderID, name, captured string) domain.FileID {
	t.Helper()
	key := "sha256:" + strings.Repeat("a", 60) + fmt.Sprintf("%04x", len(name)*7+len(captured))
	f, err := domain.NewFile(domain.NewFileID(), folder, name, key)
	if err != nil {
		t.Fatal(err)
	}
	if captured != "" {
		f.CapturedAt = testInstant(t, captured)
	}
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.UpsertBlob(key, strings.Repeat("a", 64), 1); err != nil {
			t.Fatal(err)
		}
		if err := vtx.InsertFile(f); err != nil {
			t.Fatal(err)
		}
	})
	return f.ID
}

func putTag(t *testing.T, db *sql.DB, name string) domain.TagID {
	t.Helper()
	tag, err := domain.NewTag(domain.NewTagID(), nil, name)
	if err != nil {
		t.Fatal(err)
	}
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.InsertTag(tag); err != nil {
			t.Fatal(err)
		}
	})
	return tag.ID
}

func attach(t *testing.T, db *sql.DB, file domain.FileID, tag domain.TagID) {
	t.Helper()
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.AddFileTag(file, tag, "user"); err != nil {
			t.Fatal(err)
		}
	})
}

func fileNames(files []domain.File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name
	}
	return out
}

// The sort is the one spec 4.5.1 settled on. Files with no captured_at go
// last, and among them id DESC is the tie-breaker — which, because ids are
// UUIDv7, is created_at DESC for free. That is the whole reason COALESCE was
// not needed.
func TestListFilesSortsCapturedFirstThenNewest(t *testing.T) {
	db := migrated(t)
	_, root := seedBoxWithRoot(t, db)

	// Insert in a deliberately unhelpful order.
	noCapture1 := putFile(t, db, root, "b.pdf", "")
	captured := putFile(t, db, root, "photo.jpg", "2019-08-12")
	noCapture2 := putFile(t, db, root, "c.pdf", "")

	got, _, err := ListFiles(db, query.Query{}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("%d files, want 3", len(got))
	}
	for i, want := range []domain.FileID{captured, noCapture2, noCapture1} {
		if got[i].ID != want {
			t.Fatalf("position %d is %q; full order was %v", i, got[i].Name, fileNames(got))
		}
	}
}

// allSortSpecs is every ordering the contract offers.
func allSortSpecs() []query.SortSpec {
	var out []query.SortSpec
	for _, k := range []query.SortKey{query.SortCaptured, query.SortImported, query.SortName} {
		out = append(out, query.SortSpec{Key: k}, query.SortSpec{Key: k, Asc: true})
	}
	return out
}

// seedPagingFixture builds the set that makes every ordering interesting:
// repeated capture times (so the tie-breaker matters), missing ones (so the
// nulls boundary is crossed), and names that differ only by case (so the
// folded ordering has a tie of its own).
func seedPagingFixture(t *testing.T, db *sql.DB) int {
	t.Helper()
	_, root := seedBoxWithRoot(t, db)
	rows := []struct{ name, captured string }{
		{"b.pdf", ""},
		{"IMG_1.jpg", "2019-08-12"},
		{"img_2.jpg", "2019-08-12"}, // same capture time as the one above
		{"a.pdf", ""},
		{"IMG_3.jpg", "2020"},
		{"c.pdf", ""},
		{"Img_4.jpg", "2019-08-12"}, // and again, three-way tie
	}
	for _, r := range rows {
		putFile(t, db, root, r.name, r.captured)
	}
	return len(rows)
}

// Paging must visit every row exactly once, in every order the contract
// offers, and in the same order a single whole-page query produces.
//
// This is the shape of bug that does not error: a predicate that is wrong at a
// boundary silently drops or repeats a page, and the only way to see it is to
// walk the whole table one row at a time and count. M1c caught the nulls
// boundary this way with one ordering; there are six now, and the two
// captured ones both cross that boundary.
func TestCursorWalksEveryRowExactlyOnceInEveryOrder(t *testing.T) {
	db := migrated(t)
	total := seedPagingFixture(t, db)

	for _, s := range allSortSpecs() {
		t.Run(s.ID(), func(t *testing.T) {
			seen := map[domain.FileID]int{}
			var paged []domain.FileID
			cursor := ""
			for range total + 5 {
				page, next, err := ListFiles(db, query.Query{Sort: s, Cursor: cursor, Limit: 1}, time.UTC)
				if err != nil {
					t.Fatal(err)
				}
				if len(page) == 0 {
					break
				}
				seen[page[0].ID]++
				paged = append(paged, page[0].ID)
				if next == "" {
					break
				}
				cursor = next
			}

			if len(paged) != total {
				t.Errorf("walked %d rows, want %d", len(paged), total)
			}
			for id, n := range seen {
				if n != 1 {
					t.Errorf("%s was returned %d times", id, n)
				}
			}

			whole, next, err := ListFiles(db, query.Query{Sort: s, Limit: total}, time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			if next != "" {
				t.Errorf("a page holding every row still offered a next cursor")
			}
			for i := range whole {
				if i < len(paged) && whole[i].ID != paged[i] {
					t.Errorf("row %d: paging gave %s, one whole page gave %s (%s)",
						i, paged[i], whole[i].ID, whole[i].Name)
					break
				}
			}
		})
	}
}

// Files with no capture time sort last in BOTH directions. Unknown is unknown,
// and it belongs at the end whichever way you are reading (spec 4.5.1) — which
// is also why orderBy writes NULLS LAST out even under ASC, where SQLite would
// otherwise put them first (spec 4.7).
func TestNullCaptureTimesSortLastInBothDirections(t *testing.T) {
	db := migrated(t)
	seedPagingFixture(t, db)

	for _, s := range []query.SortSpec{{Key: query.SortCaptured}, {Key: query.SortCaptured, Asc: true}} {
		got, _, err := ListFiles(db, query.Query{Sort: s, Limit: 100}, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		seenNull := false
		for _, f := range got {
			if f.CapturedAt.IsZero() {
				seenNull = true
				continue
			}
			if seenNull {
				t.Errorf("%s: %s has a capture time but came after one that does not: %v",
					s.ID(), f.Name, fileNames(got))
				break
			}
		}
	}
}

// The name ordering is on the folded name, so case does not scatter a
// directory listing (spec 4.8).
func TestNameOrderIsFolded(t *testing.T) {
	db := migrated(t)
	seedPagingFixture(t, db)

	got, _, err := ListFiles(db, query.Query{Sort: query.SortSpec{Key: query.SortName, Asc: true}, Limit: 100}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.pdf", "b.pdf", "c.pdf", "IMG_1.jpg", "img_2.jpg", "IMG_3.jpg", "Img_4.jpg"}
	if got := fileNames(got); !slices.Equal(got, want) {
		t.Errorf("name order = %v, want %v", got, want)
	}
}

// A page that happens to be exactly full is still the last page. Assuming
// otherwise gives a "more" control that leads somewhere empty.
func TestAnExactlyFullLastPageOffersNoCursor(t *testing.T) {
	db := migrated(t)
	total := seedPagingFixture(t, db)

	_, next, err := ListFiles(db, query.Query{Limit: total}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if next != "" {
		t.Error("a page holding every row still offered a next cursor")
	}
}

// A cursor from another ordering must be refused rather than approximated
// (spec 4.2.1). The refusal happens in query; this checks it survives the trip
// through the store.
func TestListFilesRefusesACursorFromAnotherSort(t *testing.T) {
	db := migrated(t)
	seedPagingFixture(t, db)

	_, next, err := ListFiles(db, query.Query{Sort: query.SortSpec{Key: query.SortCaptured}, Limit: 1}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ListFiles(db, query.Query{Sort: query.SortSpec{Key: query.SortName}, Cursor: next, Limit: 1}, time.UTC)
	if !errors.Is(err, query.ErrCursorSortMismatch) {
		t.Errorf("err = %v, want ErrCursorSortMismatch", err)
	}
}

// spec 4.4: search is a folded substring match, and the folding is ours, not
// the engine's (spec 4.7).
func TestListFilesTextIsCaseInsensitive(t *testing.T) {
	db := migrated(t)
	_, root := seedBoxWithRoot(t, db)
	putFile(t, db, root, "IMG_0001.JPG", "")
	putFile(t, db, root, "notes.txt", "")

	got, _, err := ListFiles(db, query.Query{Text: "img_00"}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "IMG_0001.JPG" {
		t.Errorf("got %v, want the one JPG", fileNames(got))
	}
}

// A pattern containing LIKE's own wildcards must match them literally,
// otherwise searching for "100%" returns the whole library.
func TestListFilesEscapesLikeWildcards(t *testing.T) {
	db := migrated(t)
	_, root := seedBoxWithRoot(t, db)
	putFile(t, db, root, "100%.txt", "")
	putFile(t, db, root, "1000.txt", "")

	got, _, err := ListFiles(db, query.Query{Text: "100%"}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "100%.txt" {
		t.Errorf("got %v, want only 100%%.txt", fileNames(got))
	}

	got, _, err = ListFiles(db, query.Query{Text: "10_0"}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want nothing — the underscore must not match any character", fileNames(got))
	}
}

// spec 4.3: conditions AND together, each condition is an OR over an already
// expanded set of tag ids. The expansion itself happens in library, against
// the in-memory tree.
func TestListFilesCombinesTagConditions(t *testing.T) {
	db := migrated(t)
	_, root := seedBoxWithRoot(t, db)
	yunnan, dad, private := putTag(t, db, "云南"), putTag(t, db, "爸"), putTag(t, db, "隐私")

	a := putFile(t, db, root, "a.jpg", "")
	b := putFile(t, db, root, "b.jpg", "")
	c := putFile(t, db, root, "c.jpg", "")
	attach(t, db, a, yunnan)
	attach(t, db, a, dad)
	attach(t, db, b, yunnan)
	attach(t, db, c, yunnan)
	attach(t, db, c, dad)
	attach(t, db, c, private)

	got, _, err := ListFiles(db, query.Query{
		Tags: query.TagAll{
			query.TagAny{yunnan},
			query.TagAny{dad},
			query.TagNot{Expr: query.TagAny{private}},
		},
	}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != a {
		t.Errorf("got %v, want only a.jpg", fileNames(got))
	}
}

func TestListFilesFiltersByBoxAndFolder(t *testing.T) {
	db := migrated(t)
	box, root := seedBoxWithRoot(t, db)
	_, otherRoot := seedBoxWithRoot(t, db)

	sub := domain.NewFolderID()
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		f, _ := domain.NewFolder(sub, root, "day1")
		if err := vtx.InsertFolder(f); err != nil {
			t.Fatal(err)
		}
	})
	putFile(t, db, root, "in-root.jpg", "")
	putFile(t, db, sub, "in-sub.jpg", "")
	putFile(t, db, otherRoot, "elsewhere.jpg", "")

	got, _, err := ListFiles(db, query.Query{Boxes: []domain.BoxID{box}}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("by box: %v", fileNames(got))
	}

	got, _, err = ListFiles(db, query.Query{Folder: &sub}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "in-sub.jpg" {
		t.Errorf("by folder: %v", fileNames(got))
	}
}

func TestListFilesRejectsABadCursor(t *testing.T) {
	db := migrated(t)
	if _, _, err := ListFiles(db, query.Query{Cursor: "not base64!!"}, time.UTC); err == nil {
		t.Error("a corrupt cursor was accepted")
	}
}

// spec 4.5.1 chose this exact ORDER BY because it uses files_captured as a
// covering index. Any variant — a COALESCE, a different tie-breaker — falls
// back to "USE TEMP B-TREE FOR ORDER BY", which is a full sort of the table
// and the end of keyset paging.
func TestFileOrderUsesTheIndex(t *testing.T) {
	db := migrated(t)
	rows, err := db.Query(`EXPLAIN QUERY PLAN SELECT id FROM files` + fileOrder)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var a, b, c int
		var detail string
		if err := rows.Scan(&a, &b, &c, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plan.String(), "TEMP B-TREE") {
		t.Errorf("the file sort no longer uses files_captured:\n%s", plan.String())
	}
	if !strings.Contains(plan.String(), "files_captured") {
		t.Errorf("expected files_captured in the plan, got:\n%s", plan.String())
	}
}

func TestExistingNamesMapsFoldedNameToBlob(t *testing.T) {
	db := migrated(t)
	box, root := seedBoxWithRoot(t, db)
	putFile(t, db, root, "IMG_0001.JPG", "")

	got, err := ExistingNames(db, box)
	if err != nil {
		t.Fatal(err)
	}
	key, ok := got[root]["img_0001.jpg"]
	if !ok {
		t.Fatalf("the folded name is not a key: %v", got)
	}
	if key == "" {
		t.Error("the blob key is empty; import needs it to tell a duplicate from a clash")
	}
}

func TestFoldersOfBoxPutsTheRootFirst(t *testing.T) {
	db := migrated(t)
	box, root := seedBoxWithRoot(t, db)
	sub := domain.NewFolderID()
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		f, _ := domain.NewFolder(sub, root, "day1")
		if err := vtx.InsertFolder(f); err != nil {
			t.Fatal(err)
		}
	})

	got, err := FoldersOfBox(db, box)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d folders, want 2", len(got))
	}
	if got[0].ID != root || got[0].ParentID != nil {
		t.Errorf("the root is not first: %+v", got[0])
	}
}
