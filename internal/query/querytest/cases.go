package querytest

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// testCase is one question and its answer.
type testCase struct {
	name  string
	build func() *Builder
	check func(t *testing.T, b Backend, f *Builder)
}

// library is the fixture most cases share.
//
//	云南   root:  passport.jpg (2019-08-12, 证件)   notes.txt (—)
//	       day1:  IMG_1.jpg (2019-08-12, 人物/爸)   IMG_2.jpg (2020-03, 人物/爸, 隐私)
//	档案   root:  100%.pdf (—)                      contract.PDF (2015)
//
// It is deliberately small: a case that needs twenty rows to state its point
// is usually a case about two things.
func library() *Builder {
	return NewBuilder().
		Tag("人物", "").Tag("爸", "人物").Tag("证件", "").Tag("隐私", "").
		Box("云南", "2019-08-12", "2019-08-20").
		Box("档案", "", "").
		Folder("云南", "day1").
		File("云南", "", "passport.jpg", "2019-08-12", "证件").
		File("云南", "", "notes.txt", "").
		File("云南", "day1", "IMG_1.jpg", "2019-08-12", "爸").
		File("云南", "day1", "IMG_2.jpg", "2020-03", "爸", "隐私").
		File("档案", "", "100%.pdf", "").
		File("档案", "", "contract.PDF", "2015")
}

func run(t *testing.T, b Backend, q query.Query) []domain.File {
	t.Helper()
	got, _, err := b.Files(q)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	return got
}

func cases() []testCase {
	var out []testCase
	add := func(name string, build func() *Builder, check func(*testing.T, Backend, *Builder)) {
		out = append(out, testCase{name: name, build: build, check: check})
	}
	simple := func() *Builder { return library() }

	// ── the zero query ──────────────────────────────────────────────────

	add("the zero query returns everything", simple, func(t *testing.T, b Backend, f *Builder) {
		if got := run(t, b, query.Query{}); len(got) != 6 {
			t.Errorf("got %v, want all six files", names(got))
		}
	})

	add("an empty library returns nothing and no cursor", NewBuilder, func(t *testing.T, b Backend, f *Builder) {
		got, next, err := b.Files(query.Query{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 || next != "" {
			t.Errorf("got %v with cursor %q", names(got), next)
		}
	})

	// ── Boxes ───────────────────────────────────────────────────────────

	add("one box", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Boxes: []domain.BoxID{f.BoxID("档案")}})
		requireSet(t, got, "100%.pdf", "contract.PDF")
	})

	add("several boxes union", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Boxes: []domain.BoxID{f.BoxID("档案"), f.BoxID("云南")}})
		if len(got) != 6 {
			t.Errorf("got %v, want everything", names(got))
		}
	})

	add("a box that does not exist matches nothing", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Boxes: []domain.BoxID{{ID: domain.NewID()}}})
		requireSet(t, got)
	})

	// ── Folder ──────────────────────────────────────────────────────────

	add("a folder does not recurse", simple, func(t *testing.T, b Backend, f *Builder) {
		root := f.FolderID("云南", "")
		got := run(t, b, query.Query{Folder: &root})
		// The two files under day1 must NOT appear: Folder is one directory,
		// not a subtree (spec 1.4). This is the case most likely to be got
		// wrong by a second implementation, and getting it wrong shows up as
		// a directory listing that quietly includes everything below it.
		requireSet(t, got, "passport.jpg", "notes.txt")
	})

	add("a subfolder holds only its own files", simple, func(t *testing.T, b Backend, f *Builder) {
		day1 := f.FolderID("云南", "day1")
		got := run(t, b, query.Query{Folder: &day1})
		requireSet(t, got, "IMG_1.jpg", "IMG_2.jpg")
	})

	// ── Text ────────────────────────────────────────────────────────────

	add("text is a substring match", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Text: "img_"}), "IMG_1.jpg", "IMG_2.jpg")
	})

	add("text ignores case in both directions", simple, func(t *testing.T, b Backend, f *Builder) {
		// The pattern's case and the stored name's case both vary, and the
		// folding is ours rather than the engine's (spec 4.7).
		requireSet(t, run(t, b, query.Query{Text: "PDF"}), "100%.pdf", "contract.PDF")
		requireSet(t, run(t, b, query.Query{Text: "pdf"}), "100%.pdf", "contract.PDF")
	})

	add("a percent sign in the pattern is a literal", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Text: "100%"}), "100%.pdf")
	})

	add("an underscore in the pattern is a literal", simple, func(t *testing.T, b Backend, f *Builder) {
		// IMG_1.jpg contains a literal underscore, so it matches; nothing
		// else does, which is what proves the underscore is not a wildcard.
		requireSet(t, run(t, b, query.Query{Text: "g_1"}), "IMG_1.jpg")
		requireSet(t, run(t, b, query.Query{Text: "1_0"}))
	})

	add("text matches names, not boxes or folders", simple, func(t *testing.T, b Backend, f *Builder) {
		// "day1" is a folder name and "云南" is a box name. Neither is a file
		// name, so neither matches (spec 4.4 covers box names in the box
		// query, not here).
		requireSet(t, run(t, b, query.Query{Text: "day1"}))
		requireSet(t, run(t, b, query.Query{Text: "云南"}))
	})

	// ── Tags ────────────────────────────────────────────────────────────

	add("a leaf matches the files carrying it", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Tags: query.TagAny{f.TagID("爸")}})
		requireSet(t, got, "IMG_1.jpg", "IMG_2.jpg")
	})

	add("a leaf holding several ids is an OR", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Tags: query.TagAny{f.TagID("爸"), f.TagID("证件")}})
		requireSet(t, got, "IMG_1.jpg", "IMG_2.jpg", "passport.jpg")
	})

	add("AND intersects", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Tags: query.TagAll{
			query.TagAny{f.TagID("爸")},
			query.TagAny{f.TagID("隐私")},
		}})
		requireSet(t, got, "IMG_2.jpg")
	})

	add("OR unions", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Tags: query.TagAnyOf{
			query.TagAny{f.TagID("证件")},
			query.TagAny{f.TagID("隐私")},
		}})
		requireSet(t, got, "passport.jpg", "IMG_2.jpg")
	})

	add("NOT excludes", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Tags: query.TagNot{Expr: query.TagAny{f.TagID("隐私")}}})
		requireSet(t, got, "passport.jpg", "notes.txt", "IMG_1.jpg", "100%.pdf", "contract.PDF")
	})

	add("three levels of nesting compose", simple, func(t *testing.T, b Backend, f *Builder) {
		// (爸 OR 证件) AND NOT 隐私 — the shape v0.1's UI cannot produce, and
		// the reason the tree is in the contract at all (spec 4.3).
		got := run(t, b, query.Query{Tags: query.TagAll{
			query.TagAnyOf{
				query.TagAny{f.TagID("爸")},
				query.TagAny{f.TagID("证件")},
			},
			query.TagNot{Expr: query.TagAny{f.TagID("隐私")}},
		}})
		requireSet(t, got, "IMG_1.jpg", "passport.jpg")
	})

	add("an empty AND matches everything", simple, func(t *testing.T, b Backend, f *Builder) {
		if got := run(t, b, query.Query{Tags: query.TagAll{}}); len(got) != 6 {
			t.Errorf("got %v, want everything — an empty AND is the identity", names(got))
		}
	})

	add("an empty OR matches nothing", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Tags: query.TagAnyOf{}}))
	})

	add("an empty leaf matches nothing", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Tags: query.TagAny{}}))
	})

	add("NOT of an empty leaf matches everything", simple, func(t *testing.T, b Backend, f *Builder) {
		if got := run(t, b, query.Query{Tags: query.TagNot{Expr: query.TagAny{}}}); len(got) != 6 {
			t.Errorf("got %v, want everything", names(got))
		}
	})

	add("a nested empty AND does not narrow", simple, func(t *testing.T, b Backend, f *Builder) {
		// The outer expression is not an identity, so an implementation
		// cannot short-circuit past the empty child — this is the only way
		// the empty-AND rule is reachable in a real translation, and without
		// it the rule is stated but never exercised.
		got := run(t, b, query.Query{Tags: query.TagAll{
			query.TagAny{f.TagID("爸")},
			query.TagAll{},
		}})
		requireSet(t, got, "IMG_1.jpg", "IMG_2.jpg")
	})

	add("a nested empty OR narrows to nothing", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Tags: query.TagAll{
			query.TagAny{f.TagID("爸")},
			query.TagAnyOf{},
		}})
		requireSet(t, got)
	})

	add("a nested empty leaf narrows to nothing", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Tags: query.TagAll{
			query.TagAny{f.TagID("爸")},
			query.TagAny{},
		}})
		requireSet(t, got)
	})

	add("a tag nobody carries matches nothing", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Tags: query.TagAny{{ID: domain.NewID()}}}))
	})

	// ── Time ────────────────────────────────────────────────────────────

	add("a range containing the value matches", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Time: &query.DateRange{From: mustDate("2019-08"), To: mustDate("2019-09")}})
		requireSet(t, got, "passport.jpg", "IMG_1.jpg")
	})

	add("a coarse value overlapping the range matches", simple, func(t *testing.T, b Backend, f *Builder) {
		// contract.PDF is known only as "2015". A search for March 2015
		// overlaps it, and overlap is the rule (spec 4.2.1): for an archive,
		// a false positive beats a false negative.
		got := run(t, b, query.Query{Time: &query.DateRange{From: mustDate("2015-03"), To: mustDate("2015-04")}})
		requireSet(t, got, "contract.PDF")
	})

	add("a range entirely before the values matches nothing", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Time: &query.DateRange{From: mustDate("2010"), To: mustDate("2011")}}))
	})

	add("a range entirely after the values matches nothing", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Time: &query.DateRange{From: mustDate("2030"), To: mustDate("2031")}}))
	})

	add("an open-ended range", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Time: &query.DateRange{From: mustDate("2020")}}), "IMG_2.jpg")
	})

	add("an open-started range", simple, func(t *testing.T, b Backend, f *Builder) {
		requireSet(t, run(t, b, query.Query{Time: &query.DateRange{To: mustDate("2015")}}), "contract.PDF")
	})

	add("files with no capture time are in no range", simple, func(t *testing.T, b Backend, f *Builder) {
		// notes.txt and 100%.pdf have none. Nothing is known about when they
		// were made, and a time filter is an explicit narrowing (spec 4.2.1).
		got := run(t, b, query.Query{Time: &query.DateRange{From: mustDate("1900"), To: mustDate("2100")}})
		requireSet(t, got, "passport.jpg", "IMG_1.jpg", "IMG_2.jpg", "contract.PDF")
	})

	// ── Sort ────────────────────────────────────────────────────────────

	add("captured descending, unknown last", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Sort: query.SortSpec{Key: query.SortCaptured}})
		assertKnownTimesFirst(t, got)
		if got[0].Name != "IMG_2.jpg" {
			t.Errorf("newest is %s, want IMG_2.jpg; full order %v", got[0].Name, names(got))
		}
	})

	add("captured ascending, unknown still last", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Sort: query.SortSpec{Key: query.SortCaptured, Asc: true}})
		assertKnownTimesFirst(t, got)
		if got[0].Name != "contract.PDF" {
			t.Errorf("oldest is %s, want contract.PDF; full order %v", got[0].Name, names(got))
		}
	})

	add("name ascending is folded", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Sort: query.SortSpec{Key: query.SortName, Asc: true}})
		requireNames(t, got, "100%.pdf", "contract.PDF", "IMG_1.jpg", "IMG_2.jpg", "notes.txt", "passport.jpg")
	})

	add("name descending reverses it", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Sort: query.SortSpec{Key: query.SortName}})
		requireNames(t, got, "passport.jpg", "notes.txt", "IMG_2.jpg", "IMG_1.jpg", "contract.PDF", "100%.pdf")
	})

	add("imported ascending is declaration order", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Sort: query.SortSpec{Key: query.SortImported, Asc: true}})
		requireNames(t, got, "passport.jpg", "notes.txt", "IMG_1.jpg", "IMG_2.jpg", "100%.pdf", "contract.PDF")
	})

	add("imported descending is newest first", simple, func(t *testing.T, b Backend, f *Builder) {
		got := run(t, b, query.Query{Sort: query.SortSpec{Key: query.SortImported}})
		requireNames(t, got, "contract.PDF", "100%.pdf", "IMG_2.jpg", "IMG_1.jpg", "notes.txt", "passport.jpg")
	})

	// ── Cursor ──────────────────────────────────────────────────────────

	add("paging one row at a time equals one whole page", simple, func(t *testing.T, b Backend, f *Builder) {
		for _, s := range allSorts() {
			whole := run(t, b, query.Query{Sort: s, Limit: 100})
			var paged []domain.File
			cursor := ""
			for range len(whole) + 3 {
				page, next, err := b.Files(query.Query{Sort: s, Limit: 1, Cursor: cursor})
				if err != nil {
					t.Fatalf("%s: %v", s.ID(), err)
				}
				if len(page) == 0 {
					break
				}
				paged = append(paged, page...)
				if next == "" {
					break
				}
				cursor = next
			}
			if a, w := names(paged), names(whole); len(a) != len(w) {
				t.Errorf("%s: paged %v, whole %v", s.ID(), a, w)
				continue
			}
			for i := range whole {
				if paged[i].ID != whole[i].ID {
					t.Errorf("%s: row %d paged %s, whole %s", s.ID(), i, paged[i].Name, whole[i].Name)
					break
				}
			}
		}
	})

	add("the last page offers no cursor", simple, func(t *testing.T, b Backend, f *Builder) {
		_, next, err := b.Files(query.Query{Limit: 6})
		if err != nil {
			t.Fatal(err)
		}
		if next != "" {
			t.Error("a page holding every row still offered a next cursor")
		}
	})

	add("a cursor from another sort is refused", simple, func(t *testing.T, b Backend, f *Builder) {
		_, next, err := b.Files(query.Query{Sort: query.SortSpec{Key: query.SortCaptured}, Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = b.Files(query.Query{Sort: query.SortSpec{Key: query.SortName}, Cursor: next, Limit: 1})
		if !errors.Is(err, query.ErrCursorSortMismatch) {
			t.Errorf("err = %v, want ErrCursorSortMismatch", err)
		}
	})

	add("a corrupt cursor is an error", simple, func(t *testing.T, b Backend, f *Builder) {
		if _, _, err := b.Files(query.Query{Cursor: "not base64!!"}); err == nil {
			t.Error("a corrupt cursor was accepted")
		}
	})

	// ── Limit ───────────────────────────────────────────────────────────

	add("a limit beyond the maximum is an error", simple, func(t *testing.T, b Backend, f *Builder) {
		if _, _, err := b.Files(query.Query{Limit: query.MaxLimit + 1}); err == nil {
			t.Error("an oversized limit was accepted")
		}
	})

	add("a negative limit is an error", simple, func(t *testing.T, b Backend, f *Builder) {
		if _, _, err := b.Files(query.Query{Limit: -1}); err == nil {
			t.Error("a negative limit was accepted")
		}
	})

	// ── combinations ────────────────────────────────────────────────────

	add("conditions intersect rather than union", simple, func(t *testing.T, b Backend, f *Builder) {
		day1 := f.FolderID("云南", "day1")
		got := run(t, b, query.Query{
			Boxes:  []domain.BoxID{f.BoxID("云南")},
			Folder: &day1,
			Text:   "img",
			Tags:   query.TagNot{Expr: query.TagAny{f.TagID("隐私")}},
			Sort:   query.SortSpec{Key: query.SortName, Asc: true},
		})
		requireSet(t, got, "IMG_1.jpg")
	})

	return out
}

// allSorts is every ordering the contract offers.
func allSorts() []query.SortSpec {
	var out []query.SortSpec
	for _, k := range []query.SortKey{query.SortCaptured, query.SortImported, query.SortName} {
		out = append(out, query.SortSpec{Key: k}, query.SortSpec{Key: k, Asc: true})
	}
	return out
}

// assertKnownTimesFirst checks that files with no capture time come last, in
// whichever direction — unknown belongs at the end whichever way you are
// reading (spec 4.5.1).
func assertKnownTimesFirst(t *testing.T, got []domain.File) {
	t.Helper()
	seenUnknown := false
	for _, f := range got {
		if f.CapturedAt.IsZero() {
			seenUnknown = true
			continue
		}
		if seenUnknown {
			t.Errorf("%s has a capture time but came after one that does not: %v", f.Name, names(got))
			return
		}
	}
}
