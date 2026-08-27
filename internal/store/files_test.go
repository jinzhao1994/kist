package store

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

// putFile inserts one file, optionally with a captured_at, and returns its id.
func putFile(t *testing.T, db *sql.DB, box domain.BoxID, folder domain.FolderID, name, captured string) domain.FileID {
	t.Helper()
	key := "sha256:" + strings.Repeat("a", 60) + fmt.Sprintf("%04x", len(name)*7+len(captured))
	f, err := domain.NewFile(domain.NewFileID(), box, folder, name, key)
	if err != nil {
		t.Fatal(err)
	}
	if captured != "" {
		if f.CapturedAt, err = domain.ParseLocalDate(captured); err != nil {
			t.Fatal(err)
		}
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
	box, root := seedBoxWithRoot(t, db)

	// Insert in a deliberately unhelpful order.
	noCapture1 := putFile(t, db, box, root, "b.pdf", "")
	captured := putFile(t, db, box, root, "photo.jpg", "2019-08-12")
	noCapture2 := putFile(t, db, box, root, "c.pdf", "")

	got, _, err := ListFiles(db, ListFilesOptions{})
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

// A cursor that loses rows at the NULL boundary is the classic keyset bug: the
// second page starts among the non-null values again, or skips the whole null
// block. Paging one row at a time through a mixed set is the cheapest way to
// catch it.
func TestListFilesCursorWalksEveryRowExactlyOnce(t *testing.T) {
	db := migrated(t)
	box, root := seedBoxWithRoot(t, db)
	for i, c := range []string{"2019-08-12", "", "2020", "", "2019-08-12"} {
		putFile(t, db, box, root, fmt.Sprintf("f%d.jpg", i), c)
	}

	seen := map[domain.FileID]bool{}
	cursor := ""
	for range 10 {
		page, next, err := ListFiles(db, ListFilesOptions{Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if seen[page[0].ID] {
			t.Fatalf("%s came back twice", page[0].Name)
		}
		seen[page[0].ID] = true
		cursor = next
		if cursor == "" {
			break
		}
	}
	if len(seen) != 5 {
		t.Errorf("walked %d of 5 files", len(seen))
	}
}

// spec 4.4: search is a folded substring match, and the folding is ours, not
// the engine's (spec 4.7).
func TestListFilesTextIsCaseInsensitive(t *testing.T) {
	db := migrated(t)
	box, root := seedBoxWithRoot(t, db)
	putFile(t, db, box, root, "IMG_0001.JPG", "")
	putFile(t, db, box, root, "notes.txt", "")

	got, _, err := ListFiles(db, ListFilesOptions{Text: "img_00"})
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
	box, root := seedBoxWithRoot(t, db)
	putFile(t, db, box, root, "100%.txt", "")
	putFile(t, db, box, root, "1000.txt", "")

	got, _, err := ListFiles(db, ListFilesOptions{Text: "100%"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "100%.txt" {
		t.Errorf("got %v, want only 100%%.txt", fileNames(got))
	}

	got, _, err = ListFiles(db, ListFilesOptions{Text: "10_0"})
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
	box, root := seedBoxWithRoot(t, db)
	yunnan, dad, private := putTag(t, db, "云南"), putTag(t, db, "爸"), putTag(t, db, "隐私")

	a := putFile(t, db, box, root, "a.jpg", "")
	b := putFile(t, db, box, root, "b.jpg", "")
	c := putFile(t, db, box, root, "c.jpg", "")
	attach(t, db, a, yunnan)
	attach(t, db, a, dad)
	attach(t, db, b, yunnan)
	attach(t, db, c, yunnan)
	attach(t, db, c, dad)
	attach(t, db, c, private)

	got, _, err := ListFiles(db, ListFilesOptions{
		AllOf:  [][]domain.TagID{{yunnan}, {dad}},
		NoneOf: []domain.TagID{private},
	})
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
	other, otherRoot := seedBoxWithRoot(t, db)

	sub := domain.NewFolderID()
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		f, _ := domain.NewFolder(sub, box, root, "day1")
		if err := vtx.InsertFolder(f); err != nil {
			t.Fatal(err)
		}
	})
	putFile(t, db, box, root, "in-root.jpg", "")
	putFile(t, db, box, sub, "in-sub.jpg", "")
	putFile(t, db, other, otherRoot, "elsewhere.jpg", "")

	got, _, err := ListFiles(db, ListFilesOptions{Box: &box})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("by box: %v", fileNames(got))
	}

	got, _, err = ListFiles(db, ListFilesOptions{Folder: &sub})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "in-sub.jpg" {
		t.Errorf("by folder: %v", fileNames(got))
	}
}

func TestListFilesRejectsABadCursor(t *testing.T) {
	db := migrated(t)
	if _, _, err := ListFiles(db, ListFilesOptions{Cursor: "not base64!!"}); err == nil {
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
	putFile(t, db, box, root, "IMG_0001.JPG", "")

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
		f, _ := domain.NewFolder(sub, box, root, "day1")
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
