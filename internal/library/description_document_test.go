package library

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

type documentProvider struct {
	fakeOCR
	document     ai.Document
	options      ai.TextOptions
	orientations int
}

func (p *documentProvider) DescribeDocument(_ context.Context, _ string, document ai.Document, options ai.TextOptions) (ai.TextResult, error) {
	p.document, p.options = document, options
	p.calls++
	return ai.TextResult{Text: p.text, Provider: "test", Model: "document-model"}, nil
}
func (p *documentProvider) ChooseUpright(context.Context, string, []ai.Image, bool) (ai.UprightChoice, error) {
	p.orientations++
	return ai.UprightChoice{}, nil
}
func documentFile(t *testing.T, l *Library) domain.FileID {
	t.Helper()
	for _, name := range []string{"pdfinfo", "pdftocairo"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip("poppler not installed")
		}
	}
	box, err := l.CreateBox(t.Context(), domain.NewID().String(), "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../derive/testdata/two-pages.pdf")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "synthetic.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = l.Import(t.Context(), dir, ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := l.Files(t.Context(), query.Query{Boxes: []domain.BoxID{box.ID}})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	return files[0].ID
}
func TestDocumentDescriptionMetadataAndProtection(t *testing.T) {
	l := openTestLibrary(t)
	id := documentFile(t, l)
	p := &documentProvider{fakeOCR: fakeOCR{text: "A synthetic two-page document."}}
	prepared, err := l.PrepareDescription(t.Context(), id, p, "test", "en")
	if err != nil {
		t.Fatal(err)
	}
	if p.document.Pages != 2 || p.document.TotalPages != 2 || p.options.Language != "en" || p.orientations != 0 {
		t.Fatal(p.document.Pages, p.options, p.orientations)
	}
	receipt, err := l.ExecuteRequest(t.Context(), requestFor("document-first", "desc"), prepared.Commit)
	if err != nil {
		t.Fatal(err)
	}
	rows := metaOf(t, l, id)
	if rows["desc:text"].Value != p.text || rows["desc:model"].Value != "test/document-model" {
		t.Fatal(rows)
	}
	if _, found := rows["desc:coverage"]; found {
		t.Fatal("complete document description recorded obsolete page coverage")
	}
	if _, found := rows["orientation:clockwise_degrees"]; found {
		t.Fatal("document was rotated")
	}
	before, _ := store.Snapshot(l.db)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Snapshot(l.db)
	if before != after {
		t.Fatal("document metadata changed during replay")
	}
	if _, err := l.Undo(t.Context(), receipt.OpID); err != nil {
		t.Fatal(err)
	}
	if _, found := metaOf(t, l, id)["desc:text"]; found {
		t.Fatal("undo retained description")
	}
	prepared, err = l.PrepareDescription(t.Context(), id, p, "test", "en")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("document-restore", "desc"), prepared.Commit); err != nil {
		t.Fatal(err)
	}
	if err := l.EditMetadata(t.Context(), id, hintEdit(t, l, id, "desc", "Example Person")); err != nil {
		t.Fatal(err)
	}
	prepared, err = l.prepareImageText(t.Context(), id, p, "test", "", "desc", "en", true)
	if err != nil || p.options.Supplement != "Example Person" {
		t.Fatal(p.options, err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("document-second", "desc"), prepared.Commit); err != nil {
		t.Fatal(err)
	}
	file, _, _ := l.File(t.Context(), id)
	row := metaOf(t, l, id)["desc:text"]
	if err := l.EditMetadata(t.Context(), id, MetadataEdit{BlobKey: file.BlobKey, Key: row.Key, BeforeValue: row.Value, BeforeSource: row.Source, BeforeType: string(row.Type), Value: "Human description"}); err != nil {
		t.Fatal(err)
	}
	calls := p.calls
	if _, err := l.prepareImageText(t.Context(), id, p, "test", file.BlobKey, "desc", "en", true); err != nil || p.calls != calls {
		t.Fatal("overwrote human description", err)
	}
}
func TestDocumentDescriptionQueueNewOnlyAndBackfill(t *testing.T) {
	l := openTestLibrary(t)
	old := documentFile(t, l)
	settings, _ := l.OCRSettings(t.Context())
	settings.AutoDesc = true
	settings.Model = "test"
	if _, err := l.SaveOCRSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	fresh := documentFile(t, l)
	p := &documentProvider{fakeOCR: fakeOCR{text: "Synthetic document"}}
	q := l.DescriptionQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "missing")
	requireOCRState(t, q, fresh, "ready")
	requireOCRState(t, l.OCRQueue(), fresh, "missing")
	if _, err := l.OCRQueue().Request(t.Context(), fresh, false); err == nil {
		t.Fatal("OCR accepted PDF")
	}
	if _, err := q.Backfill(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := q.Step(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	requireOCRState(t, q, old, "ready")
	if p.calls != 2 || p.orientations != 0 {
		t.Fatal(p.calls, p.orientations)
	}
}
func TestDescriptionDocumentFormatsAndTextBudget(t *testing.T) {
	for _, name := range []string{"report.doc", "report.docx", "slides.ppt", "slides.pptx"} {
		if _, ok := descriptionDocumentRecipe(name, "application/octet-stream"); !ok {
			t.Fatal(name)
		}
	}
	for _, name := range []string{"data.xls", "data.xlsx", "data.csv", "clip.mp4"} {
		if _, ok := descriptionDocumentRecipe(name, "application/octet-stream"); ok {
			t.Fatal(name)
		}
	}
	for _, text := range []string{strings.Repeat("文", 500), strings.Repeat("a", 1100), strings.Repeat("🎉", 500)} {
		bounded, err := boundedDescription(text)
		if err != nil || len(bounded) > ai.MaxDescriptionBytes || !utf8.ValidString(bounded) || !strings.HasSuffix(bounded, "…") {
			t.Fatal(len(bounded), err)
		}
	}
}
