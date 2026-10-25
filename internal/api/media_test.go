package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestMediaLifecycleAndRanges(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	box, err := l.CreateBox(ctx, "media", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "alpha.png")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 30, 20))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err = l.Import(ctx, src, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	path := Prefix + "/files/" + files.Files[0].ID + "/derived/thumb_512_v1"
	state := func(method, body string) library.MediaJob {
		t.Helper()
		w := call(s, method, path, "", body)
		if w.Code != 200 {
			t.Fatalf("%s %d %s", method, w.Code, w.Body)
		}
		var j library.MediaJob
		if err := json.Unmarshal(w.Body.Bytes(), &j); err != nil {
			t.Fatal(err)
		}
		return j
	}
	if got := state("GET", ""); got.State != "missing" {
		t.Fatal(got)
	}
	if w := call(s, "GET", path+"/content", "", ""); w.Code != 409 {
		t.Fatalf("not ready: %d %s", w.Code, w.Body)
	}
	first := state("POST", `{}`)
	second := state("POST", `{}`)
	if first.State != "queued" || second.Generation != first.Generation {
		t.Fatalf("%+v %+v", first, second)
	}
	if got := state("DELETE", ""); got.State != "cancelled" {
		t.Fatal(got)
	}
	if got := state("POST", `{}`); got.State != "cancelled" {
		t.Fatal("implicit retry", got)
	}
	if got := state("POST", `{"retry":true}`); got.State != "queued" {
		t.Fatal(got)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { s.RunMedia(workerCtx); close(done) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for state("GET", "").State != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("never ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	full := call(s, "GET", path+"/content", "", "")
	if full.Code != 200 || full.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("%d %s", full.Code, full.Body)
	}
	head := call(s, "HEAD", path+"/content", "", "")
	if head.Code != 200 || head.Body.Len() != 0 {
		t.Fatalf("head: %d", head.Code)
	}
	r := httptest.NewRequest("GET", "http://localhost:8473"+path+"/content", nil)
	r.Header.Set("Range", "bytes=2-7")
	part := httptest.NewRecorder()
	s.ServeHTTP(part, r)
	if part.Code != 206 || !bytes.Equal(part.Body.Bytes(), full.Body.Bytes()[2:8]) {
		t.Fatalf("range: %d", part.Code)
	}
	r = httptest.NewRequest("GET", "http://localhost:8473"+path+"/content", nil)
	r.Header.Set("If-None-Match", full.Header().Get("ETag"))
	cached := httptest.NewRecorder()
	s.ServeHTTP(cached, r)
	if cached.Code != 304 {
		t.Fatal(cached.Code)
	}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		r = httptest.NewRequest(method, "http://localhost:8473"+path, bytes.NewBufferString(`{}`))
		r.Header.Set("Origin", "https://evil.example")
		r.Header.Set("Content-Type", "application/json")
		denied := httptest.NewRecorder()
		s.ServeHTTP(denied, r)
		if denied.Code != 403 {
			t.Fatalf("cross-origin %s: %d", method, denied.Code)
		}
	}
	if got := call(s, "POST", Prefix+"/files/"+files.Files[0].ID+"/derived/unknown", "", `{}`); got.Code != 400 {
		t.Fatalf("recipe: %d", got.Code)
	}
	assertMediaBrowserCaching(t, s, path+"/content")
	count, err := l.OpCount(ctx)
	if err != nil || count != 2 {
		t.Fatalf("media added ops: %d %v", count, err)
	}
}

func assertMediaBrowserCaching(t *testing.T, s *Server, path string) {
	t.Helper()
	full := call(s, "GET", path, "", "")
	if full.Code != 200 || full.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("unversioned: %d %v", full.Code, full.Header())
	}
	etag := full.Header().Get("ETag")
	version := strings.Trim(etag, `"`)
	if version == "" {
		t.Fatal("missing generation")
	}
	versioned := path + "?v=" + url.QueryEscape(version)
	for _, method := range []string{"GET", "HEAD"} {
		got := call(s, method, versioned, "", "")
		if got.Code != 200 || got.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" || got.Header().Get("ETag") != etag {
			t.Fatalf("versioned %s: %d %v", method, got.Code, got.Header())
		}
		if method == "GET" && !bytes.Equal(got.Body.Bytes(), full.Body.Bytes()) {
			t.Fatal("versioned bytes changed")
		}
		if method == "HEAD" && got.Body.Len() != 0 {
			t.Fatal("HEAD returned bytes")
		}
		req := httptest.NewRequest(method, "http://localhost:8473"+path+"?v=old-generation", nil)
		// A matching ETag must not turn a mismatched URL version into a 304.
		req.Header.Set("If-None-Match", etag)
		mismatch := httptest.NewRecorder()
		s.ServeHTTP(mismatch, req)
		if mismatch.Code != 409 || mismatch.Header().Get("Cache-Control") != "no-store" || mismatch.Header().Get("ETag") != "" {
			t.Fatalf("mismatch: %d %v", mismatch.Code, mismatch.Header())
		}
	}
	for _, conditional := range []bool{false, true} {
		req := httptest.NewRequest("GET", "http://localhost:8473"+versioned, nil)
		expected := 206
		if conditional {
			req.Header.Set("If-None-Match", etag)
			expected = 304
		} else {
			req.Header.Set("Range", "bytes=0-9")
		}
		got := httptest.NewRecorder()
		s.ServeHTTP(got, req)
		if got.Code != expected || got.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" {
			t.Fatalf("conditional response: %d %v", got.Code, got.Header())
		}
	}
}
