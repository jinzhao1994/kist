package library

import (
	"context"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
)

type descriptionProvider struct {
	fakeOCR
	language string
}

func (p *descriptionProvider) Describe(ctx context.Context, model string, image ai.Image, language string) (ai.TextResult, error) {
	p.language = language
	return p.OCR(ctx, model, image)
}
func TestDescriptionMetadataReplayUndoAndIsolation(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	p := &descriptionProvider{fakeOCR: fakeOCR{text: "original OCR"}}
	o, err := l.PrepareOCR(t.Context(), id, p, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("desc-ocr-0001", "ocr"), o.Commit); err != nil {
		t.Fatal(err)
	}
	beforeFile, _, _ := l.File(t.Context(), id)
	p.text = strings.Repeat("一张白底测试文档。", 100)
	d, err := l.PrepareDescription(t.Context(), id, p, "test", "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := l.ExecuteRequest(t.Context(), requestFor("desc-first-01", "desc"), d.Commit)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.FileMetadata(l.db, id)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, row := range rows {
		values[row.Key] = row.Value
		if strings.HasPrefix(row.Key, "desc:") && row.Source != "desc" {
			t.Fatal(row)
		}
	}
	if values["ocr:text"] != "original OCR" || values["desc:text"] != p.text || values["desc:language"] != "zh-CN" || p.language != "zh-CN" {
		t.Fatal(values)
	}
	if values["desc:recipe"] != "3" || values["ocr:recipe"] != "1" {
		t.Fatal("incorrect recipe versions", values)
	}
	afterFile, _, _ := l.File(t.Context(), id)
	if beforeFile.Name != afterFile.Name || beforeFile.BlobKey != afterFile.BlobKey || beforeFile.CapturedAt != afterFile.CapturedAt {
		t.Fatal("description modified file attributes")
	}
	before, _ := store.Snapshot(l.db)
	if err = l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Snapshot(l.db)
	if before != after {
		t.Fatal("description changed during replay")
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) != 0 {
		t.Fatal(report, err)
	}
	if _, err = l.PrepareDescription(t.Context(), id, p, "different", "en"); err != nil || p.calls != 2 {
		t.Fatal("description regenerated", err, p.calls)
	}
	if _, err = l.Undo(t.Context(), receipt.OpID); err != nil {
		t.Fatal(err)
	}
	rows, _ = store.FileMetadata(l.db, id)
	for _, r := range rows {
		if r.Source == "desc" {
			t.Fatal("undo left description")
		}
	}
}
func TestDescriptionRejectsEmptyAndOversize(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	for _, value := range []string{" \n\t", strings.Repeat("x", (16<<10)+1)} {
		p := &descriptionProvider{fakeOCR: fakeOCR{text: value}}
		if _, err := l.PrepareDescription(t.Context(), id, p, "test", "en"); err == nil {
			t.Fatal("invalid description accepted")
		}
	}
}
func TestDescriptionQueueIndependentBoundaryAndLanguage(t *testing.T) {
	l := openTestLibrary(t)
	old := ocrFile(t, l)
	setOCR(t, l, true)
	middle := ocrFile(t, l)
	settings, _ := l.OCRSettings(t.Context())
	settings.AutoDesc = true
	if _, err := l.SaveOCRSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(l.layout.ConfigPath(), "", map[string]any{"library.language": "fr"}); err != nil {
		t.Fatal(err)
	}
	fresh := ocrFile(t, l)
	p := &descriptionProvider{fakeOCR: fakeOCR{text: "Un document."}}
	d := l.DescriptionQueue()
	d.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	o := queueFor(t, l, p)
	// Both queues must share the interprocess inference lock.
	release, ok, err := o.worker.TryAcquire()
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err = d.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	release()
	if p.calls != 0 {
		t.Fatal("parallel inference")
	}
	if err = d.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, d, old, "missing")
	requireOCRState(t, d, middle, "missing")
	requireOCRState(t, d, fresh, "ready")
	requireOCRState(t, o, fresh, "missing")
	if p.language != "fr" {
		t.Fatal(p.language)
	}
	if _, err = d.Backfill(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err = d.Step(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	requireOCRState(t, d, old, "ready")
	requireOCRState(t, d, middle, "ready")
	if p.calls != 3 {
		t.Fatal(p.calls)
	}
	if err = o.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, o, old, "missing")
	requireOCRState(t, o, middle, "ready")
}
