package library

import (
	"context"
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestPreparedExtractionRebuildAndUndo(t *testing.T) {
	l := openTestLibrary(t)
	box, err := l.CreateBox(t.Context(), "types", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Import(t.Context(), writeTree(t, map[string]string{"wrong.jpg": "%PDF-1.7\n", "empty": ""}), ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := l.Files(t.Context(), query.Query{Text: "wrong"})
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v %v", files, err)
	}
	id := files[0].ID
	prepared, err := l.PrepareExtraction(t.Context(), id, "media")
	if err != nil {
		t.Fatal(err)
	}
	info, err := l.MediaInfo(t.Context(), id)
	if err != nil || info.Kind != "" {
		t.Fatal("preparation changed state", info, err)
	}
	result, err := l.ExecuteRequest(t.Context(), requestFor("extract-media-001", "media"), prepared.Commit)
	if err != nil {
		t.Fatal(err)
	}
	info, err = l.MediaInfo(t.Context(), id)
	if err != nil || info.Kind != "document" || info.MIME != "application/pdf" {
		t.Fatal(info, err)
	}
	before, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, err := store.Snapshot(l.db)
	if err != nil || before != after {
		t.Fatal("rebuild changed media metadata", err)
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) != 0 {
		t.Fatal(report, err)
	}
	if _, err := l.Undo(t.Context(), result.OpID); err != nil {
		t.Fatal(err)
	}
	info, err = l.MediaInfo(t.Context(), id)
	if err != nil || info.Kind != "" || len(info.Pending) != 2 {
		t.Fatal(info, err)
	}
	retry, err := l.ExecuteRequest(t.Context(), requestFor("extract-media-001", "media"), prepared.Commit)
	if err != nil || retry != result {
		t.Fatal(retry, err)
	}
	info, _ = l.MediaInfo(t.Context(), id)
	if info.Kind != "" {
		t.Fatal("retry resurrected undone extraction")
	}
}
func TestExtractionPreparationRejectsRequestScopeAndCancellation(t *testing.T) {
	l := openTestLibrary(t)
	_, err := l.ExecuteRequest(t.Context(), requestFor("scope-extract-01", "media"), func(ctx context.Context) error {
		_, err := l.PrepareExtraction(ctx, domain.NewFileID(), "media")
		return err
	})
	if !errors.Is(err, ErrRequestScope) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = l.PrepareExtraction(ctx, domain.NewFileID(), "media")
	if err == nil {
		t.Fatal("cancelled preparation succeeded")
	}
}
