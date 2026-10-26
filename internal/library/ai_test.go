package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

type fakeOCR struct {
	text  string
	calls int
}

func (f *fakeOCR) Models(context.Context) ([]ai.Model, error) { return nil, nil }
func (f *fakeOCR) OCR(context.Context, string, ai.Image) (ai.OCRResult, error) {
	f.calls++
	return ai.OCRResult{Text: f.text, Provider: "test", Model: "image-model"}, nil
}
func (f *fakeOCR) Describe(ctx context.Context, model string, img ai.Image, language string) (ai.TextResult, error) {
	return f.OCR(ctx, model, img)
}
func ocrFile(t *testing.T, l *Library) domain.FileID {
	t.Helper()
	box, err := l.CreateBox(t.Context(), domain.NewID().String(), "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "scan.png"), b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = l.Import(t.Context(), dir, ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := l.Files(t.Context(), query.Query{Boxes: []domain.BoxID{box.ID}})
	if err != nil || len(files) != 1 {
		t.Fatal(err)
	}
	return files[0].ID
}
func TestOCRLongTextRebuildUndoAndNoRepeat(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	provider := &fakeOCR{text: strings.Repeat("识别文字\n", 1000)}
	p, err := l.PrepareOCR(t.Context(), id, provider, "test")
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := l.ExecuteRequest(t.Context(), requestFor("ocr-first-0001", "ocr"), p.Commit)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.FileMetadata(l.db, id)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Key == "ocr:text" {
			found = row.Value == provider.text
		}
	}
	if !found {
		t.Fatal("missing long text")
	}
	before, _ := store.Snapshot(l.db)
	if err = l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Snapshot(l.db)
	if before != after {
		t.Fatal("OCR changed during replay")
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) != 0 {
		t.Fatal(report, err)
	}
	if _, err = l.PrepareOCR(t.Context(), id, provider, "different"); err != nil || provider.calls != 1 {
		t.Fatal("repeated inference", err)
	}
	if _, err = l.Undo(t.Context(), receipt.OpID); err != nil {
		t.Fatal(err)
	}
	rows, _ = store.FileMetadata(l.db, id)
	for _, row := range rows {
		if row.Source == "ocr" {
			t.Fatal("undo left OCR metadata")
		}
	}
}
func TestOCRRejectsConcurrentExtractionAndOversize(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	provider := &fakeOCR{text: "first"}
	a, err := l.PrepareOCR(t.Context(), id, provider, "test")
	if err != nil {
		t.Fatal(err)
	}
	provider.text = "second"
	b, err := l.PrepareOCR(t.Context(), id, provider, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("ocr-one-00001", "a"), a.Commit); err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("ocr-two-00002", "b"), b.Commit); err == nil {
		t.Fatal("stale OCR accepted")
	}
	id = ocrFile(t, l)
	provider.text = strings.Repeat("a", ai.MaxTextBytes+1)
	if _, err = l.PrepareOCR(t.Context(), id, provider, "test"); err == nil {
		t.Fatal("oversize text accepted")
	}
}
func TestAIConnectionPrivateAndCompareAndSwap(t *testing.T) {
	l := openTestLibrary(t)
	rawBefore, _ := os.ReadFile(l.layout.ConfigPath())
	if _, err := l.AIConnection(t.Context()); err != nil {
		t.Fatal(err)
	}
	rawAfter, _ := os.ReadFile(l.layout.ConfigPath())
	if !bytes.Equal(rawBefore, rawAfter) {
		t.Fatal("reading connection changed settings")
	}
	before, err := l.PrepareAIConnection(t.Context())
	if err != nil || before.HostID == "" {
		t.Fatal(err)
	}
	next := before
	next.Provider = "openai"
	next.AccessToken = "private-token"
	if err = l.SaveAIConnection(t.Context(), before, next); err != nil {
		t.Fatal(err)
	}
	if err = l.SaveAIConnection(t.Context(), before, before); err == nil {
		t.Fatal("stale login replaced credentials")
	}
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(cfg)
	if bytes.Contains(raw, []byte("private-token")) {
		t.Fatal("credentials leaked")
	}
}

func TestOCRRejectsReplacedImage(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	provider := &fakeOCR{text: "old picture"}
	p, err := l.PrepareOCR(t.Context(), id, provider, "test")
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := l.File(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	box, _, err := l.FileLocation(t.Context(), file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.Import(t.Context(), writeTree(t, map[string]string{"scan.png": "replacement"}), ImportOptions{Box: box, ConflictMode: "overwrite"}); err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("ocr-replaced-0001", "replace"), p.Commit); !errors.Is(err, ErrOCRChanged) {
		t.Fatal("stale result", err)
	}
	rows, _ := store.FileMetadata(l.db, id)
	for _, row := range rows {
		if row.Source == "ocr" {
			t.Fatal("stale OCR saved")
		}
	}
}

func TestOCRPreservesDeletedText(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	provider := &fakeOCR{text: "original"}
	p, err := l.PrepareOCR(t.Context(), id, provider, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("ocr-tombstone-0001", "tombstone"), p.Commit); err != nil {
		t.Fatal(err)
	}
	if _, err = l.RemoveMetadata(t.Context(), id, "ocr:text"); err != nil {
		t.Fatal(err)
	}
	provider.text = "must stay deleted"
	p, err = l.PrepareOCR(t.Context(), id, provider, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("ocr-tombstone-0002", "retry"), p.Commit); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatal("repeated inference after deletion")
	}
	rows, _ := store.FileMetadata(l.db, id)
	for _, row := range rows {
		if row.Key == "ocr:text" && !row.Tombstone {
			t.Fatal("deleted text restored")
		}
	}
}
