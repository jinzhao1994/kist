package library

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/domain"
)

func TestExtractionQueueJobsPaginationAndCurrentLocations(t *testing.T) {
	l, box := openWithBox(t)
	date, _ := domain.ParseLocalDate("2026-03")
	if _, err := l.UpdateBox(t.Context(), box, BoxFields{Name: "Synthetic archive", StartAt: date}); err != nil {
		t.Fatal(err)
	}
	root := writeTree(t, map[string]string{
		"nested/a.txt": "a", "b.txt": "b", "c.txt": "c", "d.txt": "d", "e.txt": "e",
	})
	if _, err := l.Import(t.Context(), root, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	files := listAll(t, l, box)
	q := l.OCRQueue()
	state := ocrQueueState{}
	for i, file := range files {
		job := OCRJob{FileID: file.ID, BlobKey: file.BlobKey, Name: "old name", RequestID: uuid.NewString(), State: "queued"}
		job.State = []string{"queued", "failed", "running", "ready", "queued"}[i]
		if job.State == "failed" {
			job.Error = "synthetic failure"
		}
		state.Jobs = append(state.Jobs, job)
	}
	if err := q.save(state); err != nil {
		t.Fatal(err)
	}
	if _, err := l.RenameEntry(t.Context(), box, files[0].ID.ID, false, files[0].Name, "renamed.txt"); err != nil {
		t.Fatal(err)
	}
	first, err := q.Jobs(t.Context(), "", 1)
	if err != nil || len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	item := first.Items[0]
	if item.Name != "renamed.txt" || !item.FileAvailable || item.Box == nil || item.Box.Name != "Synthetic archive" || item.Box.StartAt != "2026-03" {
		t.Fatalf("stale file/box context: %+v", item)
	}
	// Earlier work completing must not shift the next page past another job.
	state.Jobs = state.Jobs[1:]
	if err := q.save(state); err != nil {
		t.Fatal(err)
	}
	next, err := q.Jobs(t.Context(), first.NextCursor, 20)
	if err != nil || len(next.Items) != 2 || next.NextCursor != "" || next.Items[0].State != "failed" || next.Items[0].Error != "synthetic failure" || next.Items[1].FileID != files[4].ID {
		t.Fatalf("next page: %+v, %v", next, err)
	}
	for _, item := range next.Items {
		if !item.FileAvailable || item.Box == nil || item.Box.StartAt != "2026-03" {
			t.Fatalf("missing nested file location: %+v", item)
		}
	}
	for _, other := range []*OCRQueue{l.OrientationQueue(), l.DescriptionQueue(), l.FactsQueue(), l.LocationQueue()} {
		if _, err := other.Jobs(t.Context(), first.NextCursor, 20); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("cross-kind cursor accepted: %v", err)
		}
	}
	for _, bad := range []struct {
		cursor string
		limit  int
	}{{"bad cursor", 20}, {strings.Repeat("a", 1025), 20}, {"", 0}, {"", 101}} {
		if _, err := q.Jobs(t.Context(), bad.cursor, bad.limit); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("bad query accepted: %+v, %v", bad, err)
		}
	}
	preview, err := l.PreviewEntryDeletion(t.Context(), box, files[4].ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.DeleteEntry(t.Context(), box, files[4].ID.ID, false, preview.Revision); err != nil {
		t.Fatal(err)
	}
	page, err := q.Jobs(t.Context(), "", 20)
	if err != nil || len(page.Items) != 2 || page.Items[1].FileAvailable || page.Items[1].Box != nil || page.Items[1].Name != "old name" {
		t.Fatalf("unavailable file: %+v, %v", page, err)
	}
	if err := q.save(ocrQueueState{}); err != nil {
		t.Fatal(err)
	}
	empty, err := q.Jobs(t.Context(), "", 20)
	raw, _ := json.Marshal(empty)
	if err != nil || string(raw) != `{"items":[],"next_cursor":""}` {
		t.Fatalf("empty response: %s, %v", raw, err)
	}
}
