package projection

import (
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// Every rule here comes from spec 3.4's table. They are applied on all
// platforms, not only Windows, so that relink is idempotent across a move
// between operating systems and so that CI actually executes them.
func TestSanitize(t *testing.T) {
	cases := []struct{ in, want string }{
		// Characters Windows refuses. Replaced with their full-width
		// counterparts, which read almost the same and are legal everywhere.
		{`a<b`, "a＜b"},
		{`a>b`, "a＞b"},
		{`a:b`, "a：b"},
		{`a"b`, "a＂b"},
		{`a/b`, "a／b"},
		{`a\b`, "a＼b"},
		{`a|b`, "a｜b"},
		{`a?b`, "a？b"},
		{`a*b`, "a＊b"},

		// Control characters have no readable counterpart.
		{"a\x00b", "a_b"},
		{"a\tb", "a_b"},

		// Windows silently drops trailing dots and spaces, so a name ending in
		// one would round-trip differently there than here.
		{"report.", "report"},
		{"report ", "report"},
		{"report. . ", "report"},

		// Reserved device names, with or without an extension. Only the stem
		// counts, and the match is case-insensitive.
		{"CON", "CON_"},
		{"con.txt", "con_.txt"},
		{"COM1", "COM1_"},
		{"lpt9.log", "lpt9_.log"},
		// Not reserved: the rule is the whole stem, not a prefix.
		{"CONSOLE", "CONSOLE"},
		{"COM10", "COM10"},

		// Nothing left is still a name.
		{"...", "_"},
		{"", "_"},

		// The common case must survive untouched.
		{"2019云南.jpg", "2019云南.jpg"},
		{"发票 (2).jpg", "发票 (2).jpg"},
	}
	for _, c := range cases {
		if got := Sanitize(c.in); got != c.want {
			t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Sanitize has to be idempotent, or relink would rename the same file on every
// run — and each rename is another chance at a Windows sharing violation
// (spec 3.4.1).
func TestSanitizeIsIdempotent(t *testing.T) {
	for _, in := range []string{`a<b`, "CON", "con.txt", "report. . ", "a\x00b", "", "...", "2019云南.jpg"} {
		once := Sanitize(in)
		if twice := Sanitize(once); twice != once {
			t.Errorf("Sanitize(%q) = %q, but Sanitize(%q) = %q", in, once, once, twice)
		}
	}
}

func mustBox(t *testing.T, name, start, created string) domain.Box {
	t.Helper()
	b, err := domain.NewBox(domain.NewBoxID(), domain.NewFolderID(), name, "")
	if err != nil {
		t.Fatal(err)
	}
	if start != "" {
		if b.StartAt, err = domain.ParseLocalDate(start); err != nil {
			t.Fatal(err)
		}
	}
	if created != "" {
		if b.CreatedAt, err = time.Parse(time.RFC3339, created); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

// spec 3.4: the box directory is "<date>_<slug>", the date comes from start_at
// falling back to created_at, and its length carries the precision.
func TestBoxDirName(t *testing.T) {
	cases := []struct{ name, start, created, want string }{
		{"童年老照片", "2019", "", "2019_童年老照片"},
		{"办护照", "2019-08", "", "2019-08_办护照"},
		{"婚礼", "2019-08-12", "", "2019-08-12_婚礼"},
		// Minute precision drops the colon: it cannot appear in a Windows
		// filename, so the ISO form would be uncreatable there.
		{"午餐", "2019-08-12T14:30", "", "2019-08-12_1430_午餐"},
		// No start_at falls back to created_at, so every box has a date.
		{"扫描件", "", "2026-08-27T02:13:17Z", "2026-08-27_扫描件"},
		// A name needing sanitization is sanitized after the date is joined on.
		{"2019/2020学年", "2019", "", "2019_2019／2020学年"},
	}
	for _, c := range cases {
		if got := BoxDirName(mustBox(t, c.name, c.start, c.created)); got != c.want {
			t.Errorf("BoxDirName(%q, start %q) = %q, want %q", c.name, c.start, got, c.want)
		}
	}
}

// ordered returns n ids in ascending order, which for UUIDv7 is creation
// order.
func ordered(t *testing.T, n int) []domain.ID {
	t.Helper()
	out := make([]domain.ID, n)
	for i := range out {
		out[i] = domain.NewID()
		// UUIDv7's timestamp has millisecond resolution; without a gap two ids
		// minted in the same millisecond order by their random bits, which is
		// not what this test is about.
		time.Sleep(2 * time.Millisecond)
	}
	return out
}

// spec 3.4.1: the projected name must be a pure function of library state.
// Collisions are broken by id order — UUIDv7, so creation order — with the
// first taking the bare name. That way one box changing its date can only
// change names deterministically, never depend on what happened before.
func TestResolveBreaksCollisionsByCreationOrder(t *testing.T) {
	ids := ordered(t, 3)
	first, second, third := ids[0], ids[1], ids[2]

	got := Resolve([]Named{
		{third, "2019_婚礼"}, {first, "2019_婚礼"}, {second, "2019_别的"},
	})

	if got[first] != "2019_婚礼" {
		t.Errorf("the oldest box got %q, want the bare name", got[first])
	}
	if got[third] == "2019_婚礼" {
		t.Error("the younger box kept the bare name too")
	}
	if got[second] != "2019_别的" {
		t.Errorf("an uncontested name was changed: %q", got[second])
	}

	// Rerunning on the same set in a different order gives the same answer —
	// that is what "pure function of state" has to mean in practice.
	again := Resolve([]Named{
		{second, "2019_别的"}, {third, "2019_婚礼"}, {first, "2019_婚礼"},
	})
	if again[third] != got[third] {
		t.Errorf("the same state produced two different names: %q then %q", got[third], again[third])
	}
}

// Disambiguation must survive sanitization, not create work for it: two names
// that only differed in a character Sanitize replaces collide afterwards, and
// the suffix is what pulls them apart again.
func TestResolveHandlesCollisionsCreatedBySanitize(t *testing.T) {
	ids := ordered(t, 2)
	got := Resolve([]Named{{ids[0], "2019_a:b"}, {ids[1], "2019_a：b"}})
	if got[ids[0]] == got[ids[1]] {
		t.Fatalf("both became %q", got[ids[0]])
	}
}

// Two names differing only in case are one name on macOS and on Windows, so
// they have to be treated as a collision here.
func TestResolveTreatsCaseInsensitiveClashesAsCollisions(t *testing.T) {
	ids := ordered(t, 2)
	got := Resolve([]Named{{ids[0], "Photo.JPG"}, {ids[1], "photo.jpg"}})
	if domain.Fold(got[ids[0]]) == domain.Fold(got[ids[1]]) {
		t.Errorf("%q and %q are the same file on a case-insensitive filesystem", got[ids[0]], got[ids[1]])
	}
}

// The disambiguated name keeps the extension, so the file still opens with the
// right application.
func TestDisambiguateKeepsTheExtension(t *testing.T) {
	id := domain.NewID()
	got := Disambiguate("发票.jpg", id)
	if got == "发票.jpg" {
		t.Fatal("nothing was appended")
	}
	if _, ext := domain.SplitName(got); ext != ".jpg" {
		t.Errorf("Disambiguate(%q) = %q, which is no longer a jpeg", "发票.jpg", got)
	}
}
