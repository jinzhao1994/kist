package library

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestDescriptionReextractPreservesOldResultAndDeduplicates(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	p := &descriptionProvider{fakeOCR: fakeOCR{text: "original"}}
	q := l.DescriptionQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	if _, err := q.Request(t.Context(), id, false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	value := func() string {
		t.Helper()
		rows, err := store.FileMetadata(l.db, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Key == "desc:text" {
				return row.Value
			}
		}
		return ""
	}
	p.text = ""
	j, err := q.Reextract(t.Context(), id, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "queued")
	duplicate, err := q.Reextract(t.Context(), id, uuid.NewString())
	if err != nil || duplicate.RequestID != j.RequestID {
		t.Fatal("queued duplicate", duplicate, err)
	}
	if value() != "original" {
		t.Fatal("cleared old result before inference")
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "failed")
	if value() != "original" || p.calls != 2 {
		t.Fatal("failed refresh changed result", value(), p.calls)
	}
	// A recorded failure is not silently retried with the same request key.
	if _, err := q.Reextract(t.Context(), id, j.RequestID); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "failed")
	if _, err := q.Pause(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	p.text = "replacement"
	j, err = q.Reextract(t.Context(), id, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	if value() != "replacement" || p.calls != 3 {
		t.Fatal(value(), p.calls)
	}
	if _, err := q.Reextract(t.Context(), id, j.RequestID); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.calls != 3 {
		t.Fatal("rebilled successful HTTP retry")
	}
	// Simulate a crash after archive commit but before queue cleanup.
	s, err := q.load()
	if err != nil {
		t.Fatal(err)
	}
	j.State = "running"
	s.Jobs = append(s.Jobs, j)
	if err := q.save(s); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	if p.calls != 3 {
		t.Fatal("rebilled committed refresh after restart")
	}
	receipt, found, err := l.RequestStatus(t.Context(), j.RequestID)
	if err != nil || !found {
		t.Fatal(receipt, found, err)
	}
	before, _ := store.Snapshot(l.db)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Snapshot(l.db)
	if before != after {
		t.Fatal("refresh changed after rebuild")
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) != 0 {
		t.Fatal(report, err)
	}
	if _, err := l.Undo(t.Context(), receipt.OpID); err != nil {
		t.Fatal(err)
	}
	if value() != "original" {
		t.Fatal("undo did not restore original", value())
	}
}

func TestReextractInterruptedInferenceDoesNotTreatOldResultAsSuccess(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	p := &fakeOCR{text: "original"}
	q := queueFor(t, l, p)
	if _, err := q.Request(t.Context(), id, false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Reextract(t.Context(), id, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	s, err := q.load()
	if err != nil {
		t.Fatal(err)
	}
	s.Jobs[0].State = "running"
	if err := q.save(s); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "failed")
	if p.calls != 1 {
		t.Fatal("interrupted inference was automatically retried")
	}
	if _, err := q.Backfill(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "queued")
	if _, err := q.Pause(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	if p.calls != 2 {
		t.Fatal("explicit failed-job retry skipped existing-result refresh")
	}

}

func TestBasicReextractRunsAtCurrentVersion(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	p, err := l.PrepareExtraction(t.Context(), id, "media")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.ExecuteRequest(t.Context(), requestFor("basic-extract-first", "first"), p.Commit); err != nil {
		t.Fatal(err)
	}
	skipped, err := l.PrepareExtraction(t.Context(), id, "media")
	if err != nil || len(skipped.payload) != 0 {
		t.Fatal("ordinary extraction must skip", err)
	}
	refreshed, err := l.PrepareReExtraction(t.Context(), id, "media")
	if err != nil || len(refreshed.payload) == 0 {
		t.Fatal("explicit refresh was skipped", err)
	}
	before, _ := store.FileMetadata(l.db, id)
	if _, err := l.ExecuteRequest(t.Context(), requestFor("basic-extract-again", "again"), refreshed.Commit); err != nil {
		t.Fatal(err)
	}
	after, _ := store.FileMetadata(l.db, id)
	if len(before) != len(after) {
		t.Fatal("same observations lost fields")
	}
}
