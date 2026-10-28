package library

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

type uprightProvider struct {
	fakeOCR
	directions int
	fail       bool
	inputs     []image.Point
}

func (p *uprightProvider) ChooseUpright(_ context.Context, _ string, images []ai.Image, _ bool) (ai.UprightChoice, error) {
	p.directions++
	if p.fail {
		return ai.UprightChoice{}, errors.New("orientation unavailable")
	}
	for i, raw := range images {
		img, _, err := image.Decode(bytes.NewReader(raw.Data))
		if err != nil {
			return ai.UprightChoice{}, err
		}
		b := img.Bounds()
		r, _, _, _ := img.At(b.Max.X-1, b.Min.Y).RGBA()
		if r > 60000 && b.Dy() > b.Dx() {
			return ai.UprightChoice{Index: i, Confidence: .98, Evidence: "text", Provider: "test", Model: "test"}, nil
		}
	}
	return ai.UprightChoice{Index: -1, Evidence: "none", Provider: "test", Model: "test"}, nil
}
func (p *uprightProvider) OCR(ctx context.Context, m string, input ai.Image, options ai.TextOptions) (ai.OCRResult, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil {
		return ai.OCRResult{}, err
	}
	p.inputs = append(p.inputs, image.Pt(cfg.Width, cfg.Height))
	return p.fakeOCR.OCR(ctx, m, input, options)
}
func (p *uprightProvider) Describe(ctx context.Context, m string, input ai.Image, options ai.TextOptions) (ai.TextResult, error) {
	return p.OCR(ctx, m, input, options)
}
func rotationFile(t *testing.T, l *Library) domain.FileID {
	t.Helper()
	box, err := l.CreateBox(t.Context(), domain.NewID().String(), "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			img.Set(x, y, color.NRGBA{B: 255, A: 255})
		}
	}
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	png.Encode(&b, img)
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "direction.png"), b.Bytes(), 0600); err != nil {
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
func TestOrientationSharedInputManualOverrideReplayAndUndo(t *testing.T) {
	l := openTestLibrary(t)
	id := rotationFile(t, l)
	file, _, _ := l.File(t.Context(), id)
	path, _ := l.blobs.LocalPath(blobKey(file.BlobKey))
	original, _ := os.ReadFile(path)
	p := &uprightProvider{fakeOCR: fakeOCR{text: "synthetic document"}}
	o, err := l.PrepareOCR(t.Context(), id, p, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("rotation-ocr-001", "ocr"), o.Commit); err != nil {
		t.Fatal(err)
	}
	d, err := l.PrepareDescription(t.Context(), id, p, "test", "en")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("rotation-desc-01", "desc"), d.Commit); err != nil {
		t.Fatal(err)
	}
	if p.directions != 1 || len(p.inputs) != 2 || p.inputs[0] != image.Pt(40, 80) || p.inputs[1] != image.Pt(40, 80) {
		t.Fatal("direction not shared", p.directions, p.inputs)
	}
	values := metaOf(t, l, id)
	for kind, model := range map[string]string{"orientation": "test/test", "ocr": "test/image-model", "desc": "test/image-model"} {
		if values[kind+":model"].Value != model {
			t.Fatal("model identity lost its provider", kind, values[kind+":model"])
		}
		if _, exists := values[kind+":provider"]; exists {
			t.Fatal("provider was stored separately", kind)
		}
	}
	rows, _ := store.FileMetadata(l.db, id)
	if degrees, known, manual := imageRotation(rows, file.BlobKey); degrees != 90 || !known || manual {
		t.Fatal(degrees, known, manual)
	}
	zero := 0
	receipt, err := l.ExecuteRequest(t.Context(), requestFor("rotation-manual-1", "manual"), func(ctx context.Context) error { return l.SetImageRotation(ctx, id, file.BlobKey, &zero) })
	if err != nil {
		t.Fatal(err)
	}
	forced, err := l.prepareImageText(t.Context(), id, p, "test", file.BlobKey, "orientation", "", true)
	if err != nil || len(forced.payload) != 0 || p.directions != 1 {
		t.Fatal("manual zero overwritten", err)
	}
	rows, _ = store.FileMetadata(l.db, id)
	if degrees, known, manual := imageRotation(rows, file.BlobKey); degrees != 0 || !known || !manual {
		t.Fatal(degrees, known, manual)
	}
	before, _ := store.Snapshot(l.db)
	if err = l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Snapshot(l.db)
	if before != after {
		t.Fatal("orientation replay drift")
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) != 0 {
		t.Fatal(report, err)
	}
	if _, err = l.Undo(t.Context(), receipt.OpID); err != nil {
		t.Fatal(err)
	}
	rows, _ = store.FileMetadata(l.db, id)
	if angle, _, manual := imageRotation(rows, file.BlobKey); angle != 90 || manual {
		t.Fatal("undo did not restore AI", angle, manual)
	}
	current, _ := os.ReadFile(path)
	if !bytes.Equal(current, original) {
		t.Fatal("original blob modified")
	}
}
func TestOrientationHumanChangeRejectsInflightAndCanRelease(t *testing.T) {
	l := openTestLibrary(t)
	id := rotationFile(t, l)
	f, _, _ := l.File(t.Context(), id)
	p := &uprightProvider{fakeOCR: fakeOCR{text: "text"}}
	prepared, err := l.PrepareOCR(t.Context(), id, p, "test")
	if err != nil {
		t.Fatal(err)
	}
	angle := 180
	if err = l.SetImageRotation(t.Context(), id, f.BlobKey, &angle); err != nil {
		t.Fatal(err)
	}
	if _, err = l.ExecuteRequest(t.Context(), requestFor("rotation-stale-01", "stale"), prepared.Commit); !errors.Is(err, ErrOCRChanged) {
		t.Fatal("stale inference committed", err)
	}
	if err = l.SetImageRotation(t.Context(), id, f.BlobKey, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = l.PrepareOCR(t.Context(), id, p, "test"); err != nil || p.directions != 2 {
		t.Fatal("AI did not resume", err, p.directions)
	}
	if err = l.SetImageRotation(t.Context(), id, "sha256:wrong", &angle); !errors.Is(err, ErrOCRChanged) {
		t.Fatal("stale blob accepted", err)
	}
	angle = 45
	if err = l.SetImageRotation(t.Context(), id, f.BlobKey, &angle); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("invalid angle", err)
	}
}
func TestOrientationQueueNewOnlyBackfillAndOCRFailure(t *testing.T) {
	l := openTestLibrary(t)
	old := rotationFile(t, l)
	manual := rotationFile(t, l)
	f, _, _ := l.File(t.Context(), manual)
	zero := 0
	if err := l.SetImageRotation(t.Context(), manual, f.BlobKey, &zero); err != nil {
		t.Fatal(err)
	}
	s, _ := l.OCRSettings(t.Context())
	s.Model = "test"
	s.AutoOrientation = true
	if _, err := l.SaveOCRSettings(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	fresh := rotationFile(t, l)
	p := &uprightProvider{}
	q := l.OrientationQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "missing")
	requireOCRState(t, q, fresh, "ready")
	q.Backfill(t.Context(), false)
	for range 3 {
		if err := q.Step(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	requireOCRState(t, q, old, "ready")
	if p.directions != 2 {
		t.Fatal("manual or completed file rebilled", p.directions)
	}
	failed := rotationFile(t, l)
	p.fail = true
	if _, err := l.PrepareOCR(t.Context(), failed, p, "test"); err == nil || p.calls != 0 {
		t.Fatal("OCR proceeded without orientation", err, p.calls)
	}
	rows, _ := store.FileMetadata(l.db, failed)
	ff, _, _ := l.File(t.Context(), failed)
	if _, known, _ := imageRotation(rows, ff.BlobKey); known {
		t.Fatal("network failure saved as zero")
	}
}

func TestOrientationDerivedCacheAndOfflineSelection(t *testing.T) {
	nas, id, _ := mediaFixture(t)
	f, _, _ := nas.File(t.Context(), id)
	angle := 90
	if err := nas.SetImageRotation(t.Context(), id, f.BlobKey, &angle); err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	if err := c.SetOfflineSelection(t.Context(), "rotated", ClientOfflineSelection{Files: []domain.FileID{id}}); err != nil {
		t.Fatal(err)
	}
	status, err := c.OfflineStatus(t.Context(), "rotated")
	if err != nil || len(status.Previews) != 2 {
		t.Fatal(status, err)
	}
	recipes := map[string]bool{}
	for _, p := range status.Previews {
		recipes[p.Recipe] = true
	}
	if !recipes[derive.RotatedRecipe(derive.Thumbnail, 90)] || !recipes[derive.RotatedRecipe(derive.UprightImage, 90)] {
		t.Fatal(recipes)
	}
	source := &derivedTestSource{testSyncPeer: &testSyncPeer{nas: nas}, media: nas.Media()}
	for _, recipe := range []string{derive.Thumbnail, derive.RotatedRecipe(derive.Thumbnail, 90), derive.RotatedRecipe(derive.UprightImage, 90)} {
		d, err := c.CacheDerived(t.Context(), f.BlobKey, recipe, false, source)
		if err != nil {
			t.Fatal(err)
		}
		if d.Job.Recipe != recipe {
			t.Fatal("recipe identity lost", d)
		}
		_, reader, err := c.OpenDerived(t.Context(), f.BlobKey, recipe)
		if err != nil {
			t.Fatal(err)
		}
		cfg, _, err := image.DecodeConfig(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if recipe == derive.Thumbnail {
			if cfg.Width != 32 || cfg.Height != 16 {
				t.Fatal(cfg)
			}
		} else if cfg.Width != 16 || cfg.Height != 32 {
			t.Fatal(cfg)
		}
	}
}

func TestOrientationDependencyFailureCannotBeRetriedByAnotherTask(t *testing.T) {
	l := openTestLibrary(t)
	id := rotationFile(t, l)
	setOCR(t, l, false)
	p := &uprightProvider{fail: true, fakeOCR: fakeOCR{text: "synthetic description"}}
	if _, err := l.PrepareOCR(t.Context(), id, p, "test"); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := l.PrepareDescription(t.Context(), id, p, "test", "en"); err == nil {
		t.Fatal("description bypassed failure")
	}
	if p.directions != 1 {
		t.Fatal("dependency was rebilled", p.directions)
	}
	q := l.OrientationQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	requireOCRState(t, q, id, "failed")
	p.fail = false
	if _, err := q.Request(t.Context(), id, true); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Pause(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	if _, err := l.PrepareDescription(t.Context(), id, p, "test", "en"); err != nil {
		t.Fatal(err)
	}
	if p.directions != 2 || p.calls != 1 {
		t.Fatal(p.directions, p.calls)
	}
}
