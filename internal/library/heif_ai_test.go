package library

import (
	"bytes"
	"context"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

type heifProvider struct {
	fakeOCR
	t *testing.T
}

func (p *heifProvider) OCR(ctx context.Context, model string, input ai.Image, options ai.TextOptions) (ai.TextResult, error) {
	p.t.Helper()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil || input.MIME != "image/jpeg" || format != "jpeg" || cfg.Width != 128 || cfg.Height != 96 {
		p.t.Fatal(input.MIME, cfg, format, err)
	}
	return p.fakeOCR.OCR(ctx, model, input, options)
}
func (p *heifProvider) Describe(ctx context.Context, model string, input ai.Image, options ai.TextOptions) (ai.TextResult, error) {
	return p.OCR(ctx, model, input, options)
}

func TestHEIFAIQueueAndSourceIdentity(t *testing.T) {
	if _, err := exec.LookPath("heif-info"); err != nil {
		t.Skip("libheif tools not installed")
	}
	for _, kind := range []string{"ocr", "desc"} {
		t.Run(kind, func(t *testing.T) {
			l := openTestLibrary(t)
			add := func() domain.FileID {
				box, err := l.CreateBox(t.Context(), domain.NewID().String(), "", domain.LocalDate{}, domain.LocalDate{})
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile("../derive/testdata/pattern.heic")
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				if err = os.WriteFile(filepath.Join(dir, "scan.HEIC"), data, 0600); err != nil {
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
			old := add()
			cfg, err := l.OCRSettings(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			cfg.Model = "test"
			cfg.AutoOCR = kind == "ocr"
			cfg.AutoDesc = kind == "desc"
			if _, err = l.SaveOCRSettings(t.Context(), cfg); err != nil {
				t.Fatal(err)
			}
			fresh := add()
			q := l.OCRQueue()
			if kind == "desc" {
				q = l.DescriptionQueue()
			}
			p := &heifProvider{fakeOCR: fakeOCR{text: "synthetic result"}, t: t}
			q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
			if err = q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			requireOCRState(t, q, old, "missing")
			requireOCRState(t, q, fresh, "ready")
			if _, err = q.Backfill(t.Context(), false); err != nil {
				t.Fatal(err)
			}
			if err = q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			requireOCRState(t, q, old, "ready")
			if _, err = q.Reextract(t.Context(), old, uuid.NewString()); err != nil {
				t.Fatal(err)
			}
			if err = q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			requireOCRState(t, q, old, "ready")
			if p.calls != 3 {
				t.Fatal(p.calls)
			}
			f, _, err := l.File(t.Context(), old)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := store.FileMetadata(l.db, old)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.Key == kind+":input_sha256" {
					t.Fatal("input checksum was stored in result metadata")
				}
			}
			path, _ := l.blobs.LocalPath(blobKey(f.BlobKey))
			original, err := os.ReadFile(path)
			fixture, fixtureErr := os.ReadFile("../derive/testdata/pattern.heic")
			if err != nil || fixtureErr != nil || !bytes.Equal(original, fixture) {
				t.Fatal("AI conversion changed the original HEIF", err, fixtureErr)
			}
			before, _ := store.Snapshot(l.db)
			if err = l.Rebuild(t.Context()); err != nil {
				t.Fatal(err)
			}
			after, _ := store.Snapshot(l.db)
			if before != after {
				t.Fatal("rebuild changed HEIF results")
			}
		})
	}
}
