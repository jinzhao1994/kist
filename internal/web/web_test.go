package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticRoutesAndAssetMisses(t *testing.T) {
	handler := serve(fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>kist</title>")}, "assets/app.js": {Data: []byte("console.log('offline')")}})
	for _, tc := range []struct {
		method, path string
		status       int
		contains     string
	}{
		{"GET", "/", 200, "kist"}, {"GET", "/boxes/id", 200, "kist"}, {"GET", "/assets/app.js", 200, "offline"},
		{"GET", "/assets/missing.js", 404, "404"}, {"GET", "/missing.css", 404, "404"}, {"POST", "/", 405, "Method"}, {"HEAD", "/", 200, ""},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
}
