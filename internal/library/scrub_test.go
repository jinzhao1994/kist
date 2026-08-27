package library

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/integrity"
)

// overwriteSomeBlob replaces one stored blob's content, keeping the length —
// a flipped bit, not a truncation.
func overwriteSomeBlob(t *testing.T, l *Library, content string) {
	t.Helper()
	p := someBlobPath(t, l)
	if err := os.WriteFile(p, []byte(content), 0o444); err != nil {
		t.Fatal(err)
	}
}

// The finding scrub exists for: content that still has the right length but no
// longer hashes to its own name. verify cannot see this, by design (spec 3.8).
func TestScrubFindsBitRotThatVerifyCannotSee(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "hello"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	overwriteSomeBlob(t, l, "world")

	shallow, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if shallow.Total() != 0 {
		t.Errorf("verify read the content:\n%s", report(t, shallow))
	}

	deep, err := l.Scrub(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if deep.Count(integrity.BlobCorrupt) != 1 {
		t.Errorf("BlobCorrupt = %d, want 1:\n%s", deep.Count(integrity.BlobCorrupt), report(t, deep))
	}
	// And it names the file, not only the digest (spec 3.8).
	if got := report(t, deep); !strings.Contains(got, "passport.jpg") {
		t.Errorf("the report does not name the damaged file:\n%s", got)
	}
}

func TestScrubIsCleanAndReportsWhatItRead(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	r, err := l.Scrub(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		t.Errorf("a healthy library scrubbed dirty:\n%s", report(t, r))
	}
	if r.Checked["blobs"] != 2 {
		t.Errorf("read %d blobs, want 2", r.Checked["blobs"])
	}
}

// scrub on a real library is hours. Ctrl-C must stop it, what it found so far
// must survive, and the report must admit it did not finish.
func TestScrubStopsWhenCancelledAndSaysSo(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"a.txt": "1", "b.txt": "22", "c.txt": "333", "d.txt": "4444",
	})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	seen := 0
	r, err := l.Scrub(ctx, func(ScrubProgress) {
		seen++
		if seen == 2 {
			cancel()
		}
	})
	if err != nil {
		t.Fatalf("cancelling a scrub must not be an error: %v", err)
	}
	if r.Checked["blobs"] >= 4 {
		t.Errorf("read %d blobs after cancelling at 2", r.Checked["blobs"])
	}
	if r.Checked["blobs"] == 0 {
		t.Error("the work done before the cancel was thrown away")
	}
	if r.Count(integrity.ScrubIncomplete) != 1 {
		t.Errorf("the report does not admit it stopped early:\n%s", report(t, r))
	}
	if r.Worst() != integrity.SeverityWarn {
		t.Errorf("worst = %v; an unfinished scrub is not a clean bill of health", r.Worst())
	}
}

// scrub reads; it must not write. Finding a corrupt blob especially must not
// delete it — the damaged copy is sometimes the only copy.
func TestScrubChangesNothing(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "hello"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	overwriteSomeBlob(t, l, "world")
	before := snapshotOf(t, l)
	damaged := someBlobPath(t, l)

	if _, err := l.Scrub(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	if after := snapshotOf(t, l); after != before {
		t.Error("scrub wrote to the database")
	}
	body, err := os.ReadFile(damaged)
	if err != nil {
		t.Fatalf("scrub removed the damaged blob: %v", err)
	}
	if string(body) != "world" {
		t.Errorf("scrub altered the damaged blob: %q", body)
	}
}
