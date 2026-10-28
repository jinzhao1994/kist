package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/structured"
)

const syntheticFacts = `{"fields":[{"key":"holder","label":"Holder","type":"object","children":[{"key":"name","label":"Name","type":"text","value":"Example Person"}]},{"key":"document_number","label":"Number","type":"text","value":"001234"},{"key":"rank","label":"Rank","type":"integer","value":"28"}]}`

type factsProvider struct {
	fakeOCR
	input   ai.FactsInput
	options ai.FactsOptions
}

func (p *factsProvider) ExtractFacts(_ context.Context, _ string, in ai.FactsInput, options ai.FactsOptions) (ai.TextResult, error) {
	p.calls++
	p.input, p.options = in, options
	return ai.TextResult{Text: p.text, Provider: "test", Model: "facts-model"}, nil
}

func factsEdit(t *testing.T, l *Library, id domain.FileID, path, value string, remove bool) MetadataEdit {
	t.Helper()
	f, _, _ := l.File(t.Context(), id)
	r := metaOf(t, l, id)["facts:data"]
	return MetadataEdit{BlobKey: f.BlobKey, Key: r.Key, BeforeValue: r.Value, BeforeSource: r.Source, BeforeType: string(r.Type), Path: path, Value: value, Remove: remove}
}

func saveFacts(t *testing.T, l *Library, id domain.FileID, p *factsProvider, key string, force bool) {
	t.Helper()
	prepared, err := l.prepareImageText(t.Context(), id, p, "test", "", "facts", "en", force)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.ExecuteRequest(t.Context(), requestFor(key, "facts"), prepared.Commit); err != nil {
		t.Fatal(err)
	}
}

func TestFactsHumanFieldsMergeUndoRebuildAndSync(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	p := &factsProvider{fakeOCR: fakeOCR{text: syntheticFacts}}
	saveFacts(t, l, id, p, "facts-first", false)
	if p.input.Image == nil {
		t.Fatal("missing image")
	}
	initial := metaOf(t, l, id)["facts:data"]
	edit := factsEdit(t, l, id, "/holder/name", "Correct Person", false)
	if err := l.EditMetadata(t.Context(), id, edit); err != nil {
		t.Fatal(err)
	}
	if err := l.EditMetadata(t.Context(), id, edit); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal("stale edit", err)
	}
	if err := l.EditMetadata(t.Context(), id, factsEdit(t, l, id, "/document_number", "", true)); err != nil {
		t.Fatal(err)
	}
	p.text = strings.Replace(syntheticFacts, `"value":"28"`, `"value":"29"`, 1)
	saveFacts(t, l, id, p, "facts-second", true)
	r := metaOf(t, l, id)["facts:data"]
	if r.Source != "facts" || !strings.Contains(r.Value, `"value":"Correct Person"`) || !strings.Contains(r.Value, `"deleted":true`) || !strings.Contains(r.Value, `"value":"29"`) {
		t.Fatal("lost per-field protection", r)
	}
	if strings.Contains(p.options.Outline, "Correct Person") || strings.Contains(p.options.Outline, "001234") {
		t.Fatal("outline included previous values")
	}
	// Omitted observations are retained, not automatically deleted.
	p.text = `{"fields":[]}`
	saveFacts(t, l, id, p, "facts-third", true)
	if metaOf(t, l, id)["facts:data"].Value != r.Value {
		t.Fatal("omission deleted fields")
	}
	before := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, l) != before {
		t.Fatal("rebuild drift")
	}
	if report, err := l.Verify(t.Context(), VerifyOptions{Deep: true}); err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
	// Offline editing uses the same node semantics and survives synchronization.
	c := newSyncClient(t, l)
	edit = factsEdit(t, l, id, "/rank", "30", false)
	if err := c.EditMetadata(t.Context(), id, edit); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(t.Context(), &testSyncPeer{nas: l})
	if err != nil || result.Accepted != 1 {
		t.Fatal(result, err)
	}
	updated := metaOf(t, l, id)["facts:data"]
	if updated.Source != "facts" || !strings.Contains(updated.Value, `"value":"30","source":"user"`) {
		t.Fatal("client lost leaf ownership")
	}
	head, _, _ := l.ops.Last()
	if _, err := l.Undo(t.Context(), head.ID); err != nil {
		t.Fatal(err)
	}
	if metaOf(t, l, id)["facts:data"].Value != r.Value {
		t.Fatal("undo failed to restore field tree")
	}
	if initial.Value == r.Value {
		t.Fatal("test did not change data")
	}
}

func TestFactsRejectsInvalidOutputAndStaleInference(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	p := &factsProvider{fakeOCR: fakeOCR{text: syntheticFacts}}
	saveFacts(t, l, id, p, "facts-valid", false)
	for _, bad := range []string{`{"fields":[{"key":"number","label":"Number","type":"integer","value":"001234"}]}`, `{"fields":[],"source":"user"}`, `not json`} {
		p.text = bad
		before := snapshotOf(t, l)
		if _, err := l.prepareImageText(t.Context(), id, p, "test", "", "facts", "en", true); err == nil {
			t.Fatal("accepted invalid facts")
		}
		if snapshotOf(t, l) != before {
			t.Fatal("invalid output changed library")
		}
	}
	p.text = syntheticFacts
	prepared, err := l.prepareImageText(t.Context(), id, p, "test", "", "facts", "en", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.EditMetadata(t.Context(), id, factsEdit(t, l, id, "/rank", "31", false)); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ExecuteRequest(t.Context(), requestFor("facts-stale", "facts"), prepared.Commit); !errors.Is(err, ErrOCRChanged) {
		t.Fatal(err)
	}
	if err := l.EditMetadata(t.Context(), id, factsEdit(t, l, id, "", syntheticFacts, false)); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("raw JSON edit accepted", err)
	}
	if err := l.EditMetadata(t.Context(), id, factsEdit(t, l, id, "", "", true)); err != nil {
		t.Fatal(err)
	}
	calls := p.calls
	if _, err := l.prepareImageText(t.Context(), id, p, "test", "", "facts", "en", true); err != nil || p.calls != calls {
		t.Fatal("whole-document tombstone ignored", err)
	}
}

func TestFactsCompleteDocumentAndHint(t *testing.T) {
	l := openTestLibrary(t)
	id := documentFile(t, l)
	p := &factsProvider{fakeOCR: fakeOCR{text: syntheticFacts}}
	saveFacts(t, l, id, p, "facts-document", false)
	if p.input.Document == nil || p.input.Document.Pages != 2 || p.input.Document.TotalPages != 2 {
		t.Fatal("incomplete document")
	}
	if _, ok := metaOf(t, l, id)["orientation:clockwise_degrees"]; ok {
		t.Fatal("rotated PDF")
	}
	if err := l.EditMetadata(t.Context(), id, hintEdit(t, l, id, "facts", "Example")); err != nil {
		t.Fatal(err)
	}
	saveFacts(t, l, id, p, "facts-hint", true)
	if p.options.Supplement != "Example" || p.options.Language != "en" {
		t.Fatal(p.options)
	}
	if _, err := structured.Parse(metaOf(t, l, id)["facts:data"].Value); err != nil {
		t.Fatal(err)
	}
}

func TestFactsQueueNewOnlyAndBackfill(t *testing.T) {
	l := openTestLibrary(t)
	old := documentFile(t, l)
	settings, _ := l.OCRSettings(t.Context())
	settings.AutoFacts = true
	settings.Model = "test"
	if _, err := l.SaveOCRSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	fresh := documentFile(t, l)
	p := &factsProvider{fakeOCR: fakeOCR{text: syntheticFacts}}
	q := l.FactsQueue()
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
	if p.calls != 2 {
		t.Fatal(p.calls)
	}
}

func TestFactsDeletionSnapshotRetainsHumanTree(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	p := &factsProvider{fakeOCR: fakeOCR{text: syntheticFacts}}
	saveFacts(t, l, id, p, "facts-snapshot", false)
	if err := l.EditMetadata(t.Context(), id, factsEdit(t, l, id, "/holder/name", "Correct Person", false)); err != nil {
		t.Fatal(err)
	}
	if err := l.EditMetadata(t.Context(), id, factsEdit(t, l, id, "/rank", "", true)); err != nil {
		t.Fatal(err)
	}
	before := metaOf(t, l, id)["facts:data"]
	boxes, err := l.Boxes(t.Context(), ListBoxesOptions{})
	if err != nil || len(boxes) != 1 {
		t.Fatal(err)
	}
	preview, err := l.PreviewEntryDeletion(t.Context(), boxes[0].ID, id.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := l.DeleteEntry(t.Context(), boxes[0].ID, id.ID, false, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(t.Context(), deleted); err != nil {
		t.Fatal(err)
	}
	if metaOf(t, l, id)["facts:data"] != before {
		t.Fatal("snapshot restoration lost field ownership or tombstone")
	}
	if report, err := l.Verify(t.Context(), VerifyOptions{Deep: true}); err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
}
