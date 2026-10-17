package library

import (
	"context"
	"io"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/extract/media"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

type legacyQuickTimeExtractor struct{ extract.Extractor }

func (legacyQuickTimeExtractor) Version() int { return 1 }
func (legacyQuickTimeExtractor) Extract(context.Context, io.ReadSeeker, string) ([]extract.KV, error) {
	return []extract.KV{{Key: "media:kind", Value: "other", Type: extract.TypeText}, {Key: "media:mime", Value: "application/octet-stream", Type: extract.TypeText}}, nil
}

func TestQuickTimeMetadataUpgradeRebuildAndUndo(t *testing.T) {
	l := openTestLibrary(t)
	box, err := l.CreateBox(t.Context(), "MOV", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.Import(t.Context(), writeTree(t, map[string]string{"clip.MOV": "\x00\x00\x00\x14ftypqt  0000qt  "}), ImportOptions{Box: box.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.extractUsing(t.Context(), []extract.Extractor{legacyQuickTimeExtractor{media.New()}}, ExtractOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := l.Files(t.Context(), query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	id := files[0].ID
	info, err := l.MediaInfo(t.Context(), id)
	if err != nil || info.Kind != "other" {
		t.Fatal(info, err)
	}
	prepared, err := l.PrepareExtraction(t.Context(), id, "media")
	if err != nil {
		t.Fatal(err)
	}
	result, err := l.ExecuteRequest(t.Context(), requestFor("upgrade-mov-001", "media"), prepared.Commit)
	if err != nil {
		t.Fatal(err)
	}
	info, err = l.MediaInfo(t.Context(), id)
	if err != nil || info.Kind != "video" || info.MIME != "video/quicktime" {
		t.Fatal(info, err)
	}
	before, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, err := store.Snapshot(l.db)
	if err != nil || before != after {
		t.Fatal("rebuild drift", err)
	}
	if _, err = l.Undo(t.Context(), result.OpID); err != nil {
		t.Fatal(err)
	}
	info, err = l.MediaInfo(t.Context(), id)
	if err != nil || info.Kind != "other" || info.MIME != "application/octet-stream" {
		t.Fatal(info, err)
	}
}
