package library

import (
	"context"
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/domain"
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
	p := &hintProvider{fakeOCR: fakeOCR{text: "Synthetic result"}}
	for kind, value := range map[string]string{"ocr": ocr, "desc": desc} {
		if _, exists := metaOf(t, l, id)[kind+":hint"]; exists {
			t.Fatal("hint exists before first generation")
		}
		generateHintResult(t, l, id, p, kind)
		row := metaOf(t, l, id)[kind+":hint"]
		if row.Value != "" || row.Source != kind {
			t.Fatal("generation did not create an empty hint", row)
		}
		if err := l.EditMetadata(t.Context(), id, hintEdit(t, l, id, kind, value)); err != nil {
			t.Fatal(err)
		}
		generateHintResult(t, l, id, p, kind)
	}
	if p.ocrHint != ocr || p.descHint != desc {
		t.Fatal("task hints mixed", p)
	}
	rows := metaOf(t, l, id)
	for kind, value := range map[string]string{"ocr": ocr, "desc": desc} {
		if rows[kind+":hint"].Value != value || rows[kind+":hint"].Source != "user" || rows[kind+":text"].Source != kind {
			t.Fatal("hint or result provenance changed")
		}
		if _, ok := rows[kind+":input_hint"]; ok {
			t.Fatal("redundant hint snapshot stored")
		}
	}
	prepared, err := l.prepareImageText(t.Context(), id, p, "test", f.BlobKey, "ocr", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.EditMetadata(t.Context(), id, hintEdit(t, l, id, "ocr", "Updated reference")); err != nil {
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
	// Clearing a hint stops using it on the next explicit refresh.
	if err = l.EditMetadata(t.Context(), id, hintEdit(t, l, id, "desc", "")); err != nil {
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

func hintEdit(t *testing.T, l *Library, id domain.FileID, kind, value string) MetadataEdit {
	t.Helper()
	f, _, _ := l.File(t.Context(), id)
	row := metaOf(t, l, id)[kind+":hint"]
	return MetadataEdit{BlobKey: f.BlobKey, Key: row.Key, BeforeValue: row.Value, BeforeSource: row.Source, BeforeType: string(row.Type), Value: value}
}

func generateHintResult(t *testing.T, l *Library, id domain.FileID, p ai.Provider, kind string) {
	t.Helper()
	prepared, err := l.prepareImageText(t.Context(), id, p, "test", "", kind, "en", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor(domain.NewID().String(), kind), prepared.Commit); err != nil {
		t.Fatal(err)
	}
}

func TestAIHintUndoAndOfflineSync(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	p := &hintProvider{fakeOCR: fakeOCR{text: "Synthetic result"}}
	generateHintResult(t, l, id, p, "desc")
	edit := hintEdit(t, l, id, "desc", "Example context")
	if err := l.EditMetadata(t.Context(), id, edit); err != nil {
		t.Fatal(err)
	}
	if err := l.EditMetadata(t.Context(), id, edit); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal(err)
	}
	head, _, _ := l.ops.Last()
	if _, err := l.Undo(t.Context(), head.ID); err != nil {
		t.Fatal(err)
	}
	row := metaOf(t, l, id)["desc:hint"]
	if row.Value != "" || row.Source != "desc" {
		t.Fatal("undo did not restore the generated empty hint", row)
	}
	c := newSyncClient(t, l)
	if err := c.EditMetadata(t.Context(), id, hintEdit(t, l, id, "desc", "offline context")); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(t.Context(), &testSyncPeer{nas: l})
	if err != nil || result.Accepted != 1 {
		t.Fatal(result, err)
	}
	generateHintResult(t, l, id, p, "desc")
	row = metaOf(t, l, id)["desc:hint"]
	if row.Value != "offline context" || row.Source != "user" || p.descHint != row.Value {
		t.Fatal("lost offline hint", row)
	}
}
