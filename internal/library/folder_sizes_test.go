package library

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestLogicalSizesSurviveDeletionRestoreAndReplay(t *testing.T) {
	l, id := openWithBox(t)
	ctx := t.Context()
	source := writeTree(t, map[string]string{"root.txt": "abc", "sub/a.txt": "hello", "sub/deep/b.txt": "hello"})
	if _, err := l.Import(ctx, source, ImportOptions{Box: id}); err != nil {
		t.Fatal(err)
	}
	check := func(want int64) {
		t.Helper()
		box, found, err := l.Box(ctx, id)
		if err != nil || !found || box.TotalSize != want {
			t.Fatalf("box size: %+v %v", box, err)
		}
		page, _, err := l.SearchBoxes(ctx, query.BoxQuery{})
		if err != nil || len(page) != 1 || page[0].TotalSize != want {
			t.Fatalf("search size: %+v %v", page, err)
		}
		fs, err := l.Folders(ctx, id)
		if err != nil || len(fs) == 0 || fs[0].TotalSize != want {
			t.Fatalf("root size: %+v %v", fs, err)
		}
	}
	check(13)
	fs, _ := l.Folders(ctx, id)
	var sub domain.FolderID
	for _, f := range fs {
		if f.Name == "sub" {
			sub = f.ID
			if f.TotalSize != 10 {
				t.Fatal(f.TotalSize)
			}
		}
	}
	preview, err := l.PreviewEntryDeletion(ctx, id, sub.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	op, err := l.DeleteEntry(ctx, id, sub.ID, true, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	check(3)
	if _, err = l.Undo(ctx, op); err != nil {
		t.Fatal(err)
	}
	check(13)
	if err = l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	check(13)
	report, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) != 0 {
		t.Fatalf("deep verify: %+v %v", report, err)
	}
	// Semantic snapshots must not include read-model totals in their payloads.
	contents := contentsOf(t, l, id)
	if contents.Box.TotalSize != 0 {
		t.Fatal("derived box size leaked into semantic snapshot")
	}
	for _, f := range contents.Folders {
		if f.TotalSize != 0 {
			t.Fatal("derived folder size leaked into semantic snapshot")
		}
	}
}
