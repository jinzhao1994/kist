package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticRoutesAndAssetMisses(t *testing.T) {
	handler := serve(fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>kist</title>")}, "assets/app.js": {Data: []byte("console.log('offline')")}}, func() (string, error) { return "pink", nil })
	for _, tc := range []struct {
		method, path string
		status       int
		contains     string
	}{
		{"GET", "/", 200, "kist"}, {"GET", "/boxes/id", 200, "kist"}, {"GET", "/assets/app.js", 200, "offline"},
		{"GET", "/boxes/2026-10-25_OCR%20%E6%B5%8B%E8%AF%95/kist-ocr-sample.png", 200, "kist"},
		{"GET", "/boxes/trip/folder.v2", 200, "kist"},
		{"HEAD", "/boxes/trip/photo.png", 200, ""},
		{"GET", "/assets/missing.js", 404, "404"}, {"GET", "/missing.css", 404, "404"}, {"POST", "/", 405, "Method"}, {"HEAD", "/", 200, ""},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
}

func TestHTMLIncludesCurrentAppearance(t *testing.T) {
	color := "pink"
	handler := serve(fstest.MapFS{
		"index.html": {Data: []byte(`<html><head><meta name="kist-primary-color" content=""></head></html>`)},
	}, func() (string, error) { return color, nil })
	for _, route := range []string{"/", "/files", "/settings", "/index.html"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", route, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `content="pink"`) || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d %s", route, w.Code, w.Body)
		}
	}
	color = `"><script>alert(1)</script>`
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(w.Body.String(), "<script>") || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
		t.Fatalf("appearance was stale or unescaped: %s", w.Body)
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("HEAD", "/files", nil))
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") == "" {
		t.Fatalf("HEAD: %d %s", w.Code, w.Body)
	}
}
