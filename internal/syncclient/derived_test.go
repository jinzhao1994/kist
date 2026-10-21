package syncclient_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/syncclient"
)

func TestHTTPDerivedPreviewAuthenticationRangeAndGeneration(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	root := filepath.Join(t.TempDir(), "nas")
	if _, err := library.Create(ctx, root); err != nil {
		t.Fatal(err)
	}
	nas, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nas.Close()
	credential, err := nas.AuthorizeSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 24, 12))); err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("sha256:%x", sha256.Sum256(pngBytes.Bytes()))
	// An uploaded original without a live file reference also supports previews.
	if err := nas.StoreSyncOriginal(ctx, key, int64(pngBytes.Len()), bytes.NewReader(pngBytes.Bytes())); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	handler, err := api.New(nas, api.Options{AllowedHosts: []string{server.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	server.Config.Handler = handler
	server.Start()
	defer server.Close()
	workerCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); handler.RunMedia(workerCtx) }()
	defer func() { stop(); <-done }()
	unauth, err := http.Post(server.URL+"/api/v1/sync/derived/describe", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatal(unauth.StatusCode)
	}
	peer, err := syncclient.New(server.URL, credential.Token, syncclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	caps, err := peer.Capabilities(ctx)
	if err != nil || !caps.DerivedPreviews {
		t.Fatal(caps, err)
	}
	var description library.SyncDerived
	for {
		description, err = peer.DescribeDerived(ctx, key, derive.Thumbnail, false)
		if err != nil {
			t.Fatal(err)
		}
		if description.Job.State == "ready" {
			break
		}
		if description.Job.State != "queued" && description.Job.State != "running" {
			t.Fatal(description)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	reader, err := peer.OpenDerived(ctx, description, 0)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("sha256:%x", sha256.Sum256(content)) != description.ContentKey {
		t.Fatal("descriptor hash differs")
	}
	reader, err = peer.OpenDerived(ctx, description, 10)
	if err != nil {
		t.Fatal(err)
	}
	tail, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(tail, content[10:]) {
		t.Fatal("range mismatch", err)
	}
	stale := description
	stale.Job.Generation = "stale"
	if _, err := peer.OpenDerived(ctx, stale, 10); err == nil {
		t.Fatal("stale generation accepted")
	}
	wrong := description
	wrong.ContentKey = key
	if _, err := peer.OpenDerived(ctx, wrong, 0); err == nil {
		t.Fatal("incorrect output identity accepted")
	}
	if count, _ := nas.OpCount(ctx); count != 0 {
		t.Fatal("derived preview created archive facts")
	}
}
