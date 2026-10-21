package api

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestHistoricalAPIUsesCutoffForNamesFilesAndContent(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	box, err := l.CreateBox(ctx, "original box", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("historical bytes"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.Import(ctx, source, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	imported, _ := l.OpCount(ctx)
	tag, err := l.CreateTag(ctx, "original tag", nil)
	if err != nil {
		t.Fatal(err)
	}
	cutoff, _ := l.OpCount(ctx)
	if _, err := l.UpdateBox(ctx, box.ID, library.BoxFields{Name: "current box"}); err != nil {
		t.Fatal(err)
	}
	if err := l.RenameTag(ctx, tag.ID, "current tag"); err != nil {
		t.Fatal(err)
	}
	deleted, err := l.Undo(ctx, imported)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := l.OpCount(ctx)
	base := fmt.Sprintf("/history/%d/boxes/%s", cutoff, box.ID)
	page := decodeBody[historicalBox](t, s, base+"?limit=1")
	if page.Box.Name != "original box" || len(page.Tags) != 1 || page.Tags[0].Name != "original tag" || len(page.Files) != 1 || page.Cursor == "" {
		t.Fatalf("historical box=%+v", page)
	}
	second := decodeBody[historicalBox](t, s, base+"?limit=1&cursor="+url.QueryEscape(page.Cursor))
	if len(second.Files) != 1 || second.Files[0].ID == page.Files[0].ID || second.Cursor != "" {
		t.Fatalf("second=%+v", second)
	}
	file := page.Files[0]
	detail := decodeBody[historicalFile](t, s, fmt.Sprintf("/history/%d/files/%s", cutoff, file.ID))
	if detail.File.File.ID != file.ID || detail.Tags[0].Name != "original tag" || len(detail.File.File.ExtractionPending) != 0 {
		t.Fatalf("detail=%+v", detail)
	}
	current := decodeBody[wire.FileList](t, s, "/files")
	if len(current.Files) != 0 {
		t.Fatal("current files were restored by a history read")
	}
	contentPath := Prefix + fmt.Sprintf("/files/%s/content?at=%d", file.ID, cutoff)
	content := call(s, "GET", contentPath, "", "")
	if content.Code != 200 || content.Body.String() != "historical bytes" || !strings.HasPrefix(content.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("%d %s", content.Code, content.Body)
	}
	r := httptest.NewRequest("GET", "http://localhost:8473"+contentPath, nil)
	r.Header.Set("Range", "bytes=0-3")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.String() != "hist" {
		t.Fatalf("range: %d %s", w.Code, w.Body)
	}
	for _, path := range []string{
		fmt.Sprintf("/history/%d/files/%s", deleted, file.ID),
		fmt.Sprintf("/files/%s/content?at=%d", file.ID, deleted),
		fmt.Sprintf("/history/999/boxes/%s", box.ID),
	} {
		if w := call(s, "GET", Prefix+path, "", ""); w.Code != 404 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	after, _ := l.OpCount(ctx)
	if before != after {
		t.Fatal("historical HTTP reads appended operations")
	}
}

func TestHistoricalAPIBoundsAndCancellation(t *testing.T) {
	s, _ := testServer(t)
	for _, path := range []string{"/history/0/boxes/invalid", "/history/1/files/invalid", "/history/1/boxes/" + domain.NewBoxID().String() + "?limit=1001", "/history/1/boxes/" + domain.NewBoxID().String() + "?cursor=invalid"} {
		w := call(s, "GET", Prefix+path, "", "")
		if w.Code < 400 || w.Code >= 500 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	// Fill the replay slots so cancellation must unblock a queued request.
	s.historySlots <- struct{}{}
	s.historySlots <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := s.withHistory(ctx, 1, func(context.Context, historyReader) error { t.Fatal("cancelled callback"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	err := s.withHistory(ctx, 1, func(context.Context, historyReader) error { t.Fatal("expired callback"); return nil })
	var status huma.StatusError
	if !errors.As(err, &status) || status.GetStatus() != 503 {
		t.Fatalf("deadline: %v", err)
	}
	<-s.historySlots
	<-s.historySlots
	if err := s.withHistory(t.Context(), 1, func(context.Context, historyReader) error { return nil }); !errors.Is(err, library.ErrNoSuchOp) {
		t.Fatal(err)
	}
	if len(s.historySlots) != 0 {
		t.Fatal("failed query leaked a slot")
	}
}
