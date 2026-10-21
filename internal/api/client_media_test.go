package api

import (
	"context"
	"image"
	"image/png"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

type clientMediaSource struct {
	previewOriginalSource
	media *library.Media
}

func (s clientMediaSource) Capabilities(ctx context.Context) (library.SyncCapabilities, error) {
	caps, err := s.previewOriginalSource.Capabilities(ctx)
	caps.DerivedPreviews = true
	return caps, err
}
func (s clientMediaSource) DescribeDerived(ctx context.Context, key, recipe string, retry bool) (library.SyncDerived, error) {
	return s.media.DescribeSyncDerived(ctx, key, recipe, retry)
}
func (s clientMediaSource) OpenDerived(ctx context.Context, d library.SyncDerived, offset int64) (io.ReadCloser, error) {
	_, f, err := s.media.OpenSyncDerived(ctx, d.Job.BlobKey, d.Job.Recipe, d.Job.Generation, d.ContentKey)
	if err != nil {
		return nil, err
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func TestClientMediaHTTPQueueRangesAndOfflineCache(t *testing.T) {
	nasServer, nas := testServer(t)
	ctx := t.Context()
	box, err := nas.CreateBox(ctx, "preview", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sample.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 30, 20)))
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = nas.Import(ctx, path, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	s := clientServerFromNAS(t, nas)
	source := clientMediaSource{previewOriginalSource: previewOriginalSource{nas: nas}, media: nasServer.media}
	s.previews = s.client.Media(func(context.Context) (library.DerivedSource, error) { return source, nil })
	files := decodeBody[wire.FileList](t, s, "/files")
	endpoint := "/files/" + files.Files[0].ID + "/derived/thumb_512_v1"
	if job := decodeBody[library.MediaJob](t, s, endpoint); job.State != "missing" {
		t.Fatal(job)
	}
	if w := call(s, "POST", Prefix+endpoint, "", `{}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	nasDone, clientDone := make(chan struct{}), make(chan struct{})
	go func() { nasServer.RunMedia(workerCtx); close(nasDone) }()
	go func() { s.RunMedia(workerCtx); close(clientDone) }()
	defer func() { cancel(); <-nasDone; <-clientDone }()
	deadline := time.Now().Add(15 * time.Second)
	for decodeBody[library.MediaJob](t, s, endpoint).State != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("preview never ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-nasDone
	<-clientDone
	// A fresh server has no connection configured. Completed bytes remain usable.
	offline, err := NewClient(s.client, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	if job := decodeBody[library.MediaJob](t, offline, endpoint); job.State != "ready" {
		t.Fatal(job)
	}
	request := httptest.NewRequest("GET", "http://localhost:8473"+Prefix+endpoint+"/content", nil)
	request.Header.Set("Range", "bytes=0-9")
	result := httptest.NewRecorder()
	offline.ServeHTTP(result, request)
	if result.Code != 206 || result.Body.Len() != 10 || result.Header().Get("ETag") == "" {
		t.Fatal(result.Code, result.Header(), result.Body)
	}
	if w := call(offline, "HEAD", Prefix+endpoint+"/content", "", ""); w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal(w.Code, w.Body)
	}
}
