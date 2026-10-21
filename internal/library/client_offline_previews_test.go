package library

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientPrepareOfflineRequiresAndRetainsPreviews(t *testing.T) {
	c, f := localPhotoFixture(t)
	ctx := t.Context()
	extraction, err := c.PrepareExtraction(ctx, f.ID, "media")
	if err != nil {
		t.Fatal(err)
	}
	if err = extraction.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	before, _, err := c.LocalHistory(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SetOfflineSelection(ctx, "photo", ClientOfflineSelection{Files: []domain.FileID{f.ID}}); err != nil {
		t.Fatal(err)
	}
	status, err := c.OfflineStatus(ctx, "photo")
	if err != nil || status.Ready || status.Available != 1 || len(status.Previews) != 1 || status.Previews[0].Recipe != derive.Thumbnail {
		t.Fatal(status, err)
	}
	if _, err = c.PrepareOffline(ctx, "photo", nil); !errors.Is(err, ErrNotCached) {
		t.Fatal("missing preview claimed ready", err)
	}
	status, err = c.PrepareOffline(ctx, "photo", nil, c.RenderWorkspace)
	if err != nil || !status.Ready || status.PreviewsAvailable != 1 {
		t.Fatal(status, err)
	}
	usage, err := c.SetCacheLimits(ctx, time.Hour, 1)
	if err != nil || !usage.OverLimit || usage.PinnedBytes == 0 {
		t.Fatal("preview not retained", usage, err)
	}
	status, err = c.OfflineStatus(ctx, "photo")
	if err != nil || !status.Ready {
		t.Fatal(status, err)
	}
	after, _, err := c.LocalHistory(ctx, "", 100)
	if err != nil || len(after) != len(before) {
		t.Fatal("preparation changed history", after, err)
	}
	d, err := c.derivedRecord(f.BlobKey, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(c.cachePath(d.ContentKey, ".data")); err != nil {
		t.Fatal(err)
	}
	status, err = c.OfflineStatus(ctx, "photo")
	if err != nil || status.Ready || status.PreviewsAvailable != 0 || status.Available != 1 {
		t.Fatal("lost preview still ready", status, err)
	}
}

func TestClientPrepareOfflineRechecksSelectionAfterRendering(t *testing.T) {
	c, f := localPhotoFixture(t)
	ctx := t.Context()
	extraction, err := c.PrepareExtraction(ctx, f.ID, "media")
	if err != nil {
		t.Fatal(err)
	}
	if err = extraction.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.SetOfflineSelection(ctx, "photo", ClientOfflineSelection{Files: []domain.FileID{f.ID}}); err != nil {
		t.Fatal(err)
	}
	_, err = c.PrepareOffline(ctx, "photo", nil, func(ctx context.Context, request ClientRenderRequest) (derive.Result, error) {
		if err := c.SetOfflineSelection(ctx, "photo", ClientOfflineSelection{}); err != nil {
			t.Fatal(err)
		}
		return c.RenderWorkspace(ctx, request)
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("removed selection accepted", err)
	}
}

func TestClientOfflinePreviewDependenciesUnionSharedContent(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	a := importOneFile(t, nas, box, "one.gpx", "shared content")
	b := importOneFile(t, nas, box, "two.GPX", "shared content")
	d := importOneFile(t, nas, box, "report.docx", "shared content")
	c := newSyncClient(t, nas)
	if err := c.SetOfflineSelection(ctx, "mixed", ClientOfflineSelection{Files: []domain.FileID{a.ID, b.ID, d.ID}}); err != nil {
		t.Fatal(err)
	}
	status, err := c.OfflineStatus(ctx, "mixed")
	if err != nil || len(status.Originals) != 1 || len(status.Previews) != 3 {
		t.Fatal(status, err)
	}
	counts := map[string]int{}
	for _, preview := range status.Previews {
		counts[preview.Recipe] = len(preview.Files)
	}
	if counts[derive.GPXMap] != 2 || counts[derive.OfficeThumbnail] != 1 || counts[derive.OfficePDF] != 1 {
		t.Fatal(counts)
	}
}

func TestClientPrepareOfflineDownloadsRequiredRemotePreview(t *testing.T) {
	nas, id, _ := mediaFixture(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	// mediaFixture imports bytes; add the same catalog facts used by the browser.
	extraction, err := nas.PrepareExtraction(ctx, id, "media")
	if err != nil {
		t.Fatal(err)
	}
	if err = extraction.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	if err = c.SetOfflineSelection(ctx, "photo", ClientOfflineSelection{Files: []domain.FileID{id}}); err != nil {
		t.Fatal(err)
	}
	source := &offlinePreviewSource{&derivedTestSource{testSyncPeer: &testSyncPeer{nas: nas}, media: nas.Media()}}
	status, err := c.PrepareOffline(ctx, "photo", source)
	if err != nil || !status.Ready || status.PreviewsAvailable != 1 {
		t.Fatal(status, err)
	}
	calls := source.calls
	source.offline = true
	status, err = c.PrepareOffline(ctx, "photo", source)
	if err != nil || !status.Ready || source.calls != calls {
		t.Fatal("cached preparation contacted NAS", status, err)
	}
}

type offlinePreviewSource struct{ *derivedTestSource }

func (s *offlinePreviewSource) OpenOriginal(ctx context.Context, key string, _ int64, offset int64) (io.ReadCloser, error) {
	r, err := s.nas.OpenSyncOriginal(ctx, key)
	if err != nil {
		return nil, err
	}
	if _, err = r.Seek(offset, io.SeekStart); err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}
