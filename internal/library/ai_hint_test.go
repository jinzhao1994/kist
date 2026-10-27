package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/oplog"
)

type hintProvider struct {
	fakeOCR
	ocrHint, descHint string
}

func (p *hintProvider) OCR(ctx context.Context, model string, image ai.Image, options ai.TextOptions) (ai.OCRResult, error) {
	p.ocrHint = options.Supplement
	return p.fakeOCR.OCR(ctx, model, image, options)
}
func (p *hintProvider) Describe(ctx context.Context, model string, image ai.Image, options ai.TextOptions) (ai.TextResult, error) {
	p.descHint = options.Supplement
	return p.fakeOCR.OCR(ctx, model, image, options)
}

func TestAIHintsIsolationSnapshotAndHumanProtection(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	f, _, _ := l.File(t.Context(), id)
	ocr, desc := "Example Person", "A synthetic training document"
	for kind, value := range map[string]string{"ocr": ocr, "desc": desc} {
		if err := l.SetAIHint(t.Context(), id, kind, AIHintEdit{BlobKey: f.BlobKey, Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	p := &hintProvider{fakeOCR: fakeOCR{text: "Synthetic result"}}
	for _, kind := range []string{"ocr", "desc"} {
		prepared, err := l.prepareImageText(t.Context(), id, p, "test", f.BlobKey, kind, "en", true)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = l.ExecuteRequest(t.Context(), requestFor("hint-initial-"+kind, kind), prepared.Commit); err != nil {
			t.Fatal(err)
		}
	}
	if p.ocrHint != ocr || p.descHint != desc {
		t.Fatal("task hints mixed", p)
	}
	rows := metaOf(t, l, id)
	if rows["ocr:input_hint"].Value != ocr || rows["desc:input_hint"].Value != desc || rows["ocr:hint"].Source != "user" {
		t.Fatal("missing provenance")
	}
	prepared, err := l.prepareImageText(t.Context(), id, p, "test", f.BlobKey, "ocr", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.SetAIHint(t.Context(), id, "ocr", AIHintEdit{BlobKey: f.BlobKey, Before: &ocr, Value: "Updated reference"}); err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("hint-stale-request", "ocr"), prepared.Commit); !errors.Is(err, ErrOCRChanged) {
		t.Fatal("stale inference committed", err)
	}
	row := rows["ocr:text"]
	if err = l.EditMetadata(t.Context(), id, MetadataEdit{BlobKey: f.BlobKey, Key: row.Key, BeforeValue: row.Value, BeforeSource: row.Source, BeforeType: string(row.Type), Value: "Human correction"}); err != nil {
		t.Fatal(err)
	}
	calls := p.calls
	prepared, err = l.prepareImageText(t.Context(), id, p, "test", f.BlobKey, "ocr", "", true)
	if err != nil || len(prepared.payload) != 0 || p.calls != calls {
		t.Fatal("human text triggered inference", err)
	}
	// Clearing an active hint removes its old generated snapshot on explicit refresh.
	if err = l.SetAIHint(t.Context(), id, "desc", AIHintEdit{BlobKey: f.BlobKey, Before: &desc, Value: ""}); err != nil {
		t.Fatal(err)
	}
	prepared, err = l.prepareImageText(t.Context(), id, p, "test", f.BlobKey, "desc", "en", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("hint-clear-desc", "desc"), prepared.Commit); err != nil {
		t.Fatal(err)
	}
	if p.descHint != "" {
		t.Fatal("old hint reused")
	}
	if _, ok := metaOf(t, l, id)["desc:input_hint"]; ok {
		t.Fatal("old generated hint retained")
	}
	before := snapshotOf(t, l)
	if err = l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if before != snapshotOf(t, l) {
		t.Fatal("rebuild drift")
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
}

func TestAIHintValidationUndoAndOfflineSync(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	f, _, _ := l.File(t.Context(), id)
	for _, edit := range []AIHintEdit{{BlobKey: f.BlobKey, Value: strings.Repeat("a", 1025)}, {BlobKey: f.BlobKey, Value: string([]byte{0xff})}} {
		if err := l.SetAIHint(t.Context(), id, "ocr", edit); !errors.Is(err, ErrInvalidRequest) {
			t.Fatal(err)
		}
	}
	if err := l.SetAIHint(t.Context(), id, "orientation", AIHintEdit{BlobKey: f.BlobKey, Value: "hint"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	if err := l.SetAIHint(t.Context(), id, "ocr", AIHintEdit{BlobKey: "changed", Value: "hint"}); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal(err)
	}
	if err := l.SetAIHint(t.Context(), id, "ocr", AIHintEdit{BlobKey: f.BlobKey, Value: "hint"}); err != nil {
		t.Fatal(err)
	}
	if err := l.SetAIHint(t.Context(), id, "ocr", AIHintEdit{BlobKey: f.BlobKey, Value: "stale"}); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal(err)
	}
	head, _, _ := l.ops.Last()
	if _, err := l.Undo(t.Context(), head.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := metaOf(t, l, id)["ocr:hint"]; ok {
		t.Fatal("undo left hint")
	}
	c := newSyncClient(t, l)
	if err := c.SetAIHint(t.Context(), id, "desc", AIHintEdit{BlobKey: f.BlobKey, Value: "offline context"}); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(t.Context(), &testSyncPeer{nas: l})
	if err != nil || result.Accepted != 1 {
		t.Fatal(result, err)
	}
	if metaOf(t, l, id)["desc:hint"].Value != "offline context" {
		t.Fatal("lost offline hint")
	}
}
