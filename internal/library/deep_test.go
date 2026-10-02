package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
)

// forceRename changes a materialized view behind library's back. The op log
// still says the old name, so a replay produces the old name — which is
// exactly the drift --deep exists to find.
func forceRename(t *testing.T, l *Library, from, to string) {
	t.Helper()
	res, err := l.db.Exec(`UPDATE files SET name = ?, name_fold = ? WHERE name = ?`, to, to, from)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("renamed %d rows, want 1", n)
	}
}

// The point of the whole task: a materialized view that does not match the log
// is invisible to the cheap checks and obvious to this one.
func TestDeepVerifyFindsAViewThatDriftedFromTheLog(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	forceRename(t, l, "a.txt", "tampered.txt")

	shallow, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if shallow.Count(integrity.ViewDrift) != 0 {
		t.Error("the cheap check reported drift; it cannot know")
	}

	deep, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if deep.Count(integrity.ViewDrift) == 0 {
		t.Errorf("--deep did not notice a view row the log does not produce:\n%s", report(t, deep))
	}
	if !strings.Contains(report(t, deep), "files") {
		t.Errorf("the report does not name the table:\n%s", report(t, deep))
	}
}

// A healthy library must come back clean, or the check is noise. This is
// TestRebuildReproducesTheLibraryExactly asked through a command a user can
// actually run.
func TestDeepVerifyIsCleanOnAHealthyLibrary(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y", "day1/c.txt": "z"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	tag, err := l.CreateTag(t.Context(), "人物", nil)
	if err != nil {
		t.Fatal(err)
	}
	files := listAll(t, l, box)
	if _, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{files[0].ID}); err != nil {
		t.Fatal(err)
	}

	r, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		t.Errorf("a healthy library drifted:\n%s", report(t, r))
	}
	if r.Checked["tables replayed"] == 0 {
		t.Error("the report does not say how many tables were compared")
	}
}

// --deep must leave nothing behind: not a row, not a file. A check that
// litters is a check people stop running.
func TestDeepVerifyLeavesNothingBehind(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	before := snapshotOf(t, l)

	if _, err := l.Verify(t.Context(), VerifyOptions{Deep: true}); err != nil {
		t.Fatal(err)
	}
	if after := snapshotOf(t, l); after != before {
		t.Error("--deep wrote to the library")
	}

	entries, err := os.ReadDir(l.Layout().Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".kist-verify") {
			t.Errorf("the scratch copy was left behind: %s", filepath.Join(l.Layout().Root, e.Name()))
		}
	}
}

func TestDeepVerifyKeepsReplayAndLiveAtTheSameVersion(t *testing.T) {
	l := openTestLibrary(t)
	other, err := Open(l.Layout().Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	l.hooks.afterDeepReplay = func() {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer cancel()
		if _, err := other.CreateTag(ctx, "during-verify", nil); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("writer between replay and comparison = %v, want timeout", err)
		}
	}
	r, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		t.Fatalf("verify reported a false finding: %s", report(t, r))
	}
	if _, err := other.CreateTag(t.Context(), "after-verify", nil); err != nil {
		t.Fatalf("verify did not release the lock: %v", err)
	}
}
