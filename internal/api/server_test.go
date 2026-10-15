package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func testServer(t *testing.T) (*Server, *library.Library) {
	t.Helper()
	root := t.TempDir()
	if _, err := library.Create(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	l, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	s, err := New(l, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	return s, l
}
func call(s *Server, method, path, key, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost:8473"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestBoxRequestLifecycle(t *testing.T) {
	s, l := testServer(t)
	first := call(s, "POST", Prefix+"/boxes", "create-box-001", `{"name":"旅行","start_at":"2020"}`)
	if first.Code != 200 {
		t.Fatalf("%d %s", first.Code, first.Body)
	}
	retry := call(s, "POST", Prefix+"/boxes", "create-box-001", `{ "start_at":"2020", "name":"旅行" }`)
	if retry.Code != 200 || !bytes.Equal(first.Body.Bytes(), retry.Body.Bytes()) {
		t.Fatalf("retry=%d %s", retry.Code, retry.Body)
	}
	conflict := call(s, "POST", Prefix+"/boxes", "create-box-001", `{"name":"other"}`)
	if conflict.Code != 409 || !strings.Contains(conflict.Body.String(), "request_conflict") {
		t.Fatalf("conflict=%d %s", conflict.Code, conflict.Body)
	}
	listing := call(s, "GET", Prefix+"/boxes", "", "")
	var boxes wire.BoxList
	if err := json.Unmarshal(listing.Body.Bytes(), &boxes); err != nil || len(boxes.Boxes) != 1 {
		t.Fatalf("list=%s err=%v", listing.Body, err)
	}
	if boxes.Boxes[0].StartAt != "2020" {
		t.Fatal(boxes)
	}
	status := call(s, "GET", Prefix+"/requests/create-box-001", "", "")
	if !bytes.Equal(first.Body.Bytes(), status.Body.Bytes()) {
		t.Fatalf("status=%s", status.Body)
	}
	path := Prefix + "/boxes/" + boxes.Boxes[0].ID
	update := call(s, "PUT", path, "update-box-001", `{"name":"later"}`)
	if update.Code != 200 {
		t.Fatalf("update=%d %s", update.Code, update.Body)
	}
	cross := call(s, "PUT", path, "create-box-001", `{"name":"旅行","start_at":"2020"}`)
	if cross.Code != 409 {
		t.Fatalf("method binding=%d %s", cross.Code, cross.Body)
	}
	n, err := l.OpCount(t.Context())
	if err != nil || n != 2 {
		t.Fatalf("ops=%d %v", n, err)
	}
}
func TestRequestBoundary(t *testing.T) {
	s, l := testServer(t)
	for _, tc := range []struct {
		name, host, origin, media, body, key string
		status                               int
	}{
		{"foreign host", "evil.example", "", "application/json", `{"name":"x"}`, "boundary-001", 403},
		{"foreign origin", "localhost:8473", "https://evil.example", "application/json", `{"name":"x"}`, "boundary-001", 403},
		{"form", "localhost:8473", "", "text/plain", `{"name":"x"}`, "boundary-001", 415},
		{"missing key", "localhost:8473", "", "application/json", `{"name":"x"}`, "", 422},
		{"invalid json", "localhost:8473", "", "application/json", `{`, "boundary-001", 400},
		{"invalid date", "localhost:8473", "", "application/json", `{"name":"x","start_at":"2020-99"}`, "boundary-001", 400},
		{"reversed range", "localhost:8473", "", "application/json", `{"name":"x","start_at":"2021","end_at":"2020"}`, "boundary-001", 400},
		{"oversized", "localhost:8473", "", "application/json", `{"name":"` + strings.Repeat("a", maxJSONBody) + `"}`, "boundary-001", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://"+tc.host+Prefix+"/boxes", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			var e wire.Error
			if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || e.Code == "" || e.Class == "" {
				t.Fatalf("error contract: %s %v", w.Body, err)
			}
		})
	}
	n, err := l.OpCount(t.Context())
	if n != 0 || err != nil {
		t.Fatalf("rejected writes: %d %v", n, err)
	}
}
func TestOpenAPIAndEmptyList(t *testing.T) {
	s, _ := testServer(t)
	w := call(s, "GET", Prefix+"/boxes", "", "")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"boxes":[]}` {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w = call(s, "GET", Prefix+"/openapi.json", "", "")
	var doc map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &doc) != nil {
		t.Fatalf("schema=%d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "Idempotency-Key") || !strings.Contains(w.Body.String(), `"code"`) {
		t.Fatal("schema lacks request or error contract")
	}
}

func TestStaticFallbackDoesNotHideAPI404(t *testing.T) {
	_, l := testServer(t)
	s, err := New(l, Options{AllowedHosts: []string{"localhost:8473"}, UI: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("UI document")) })})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/unknown", "/api/v1/missing", "/api"} {
		w := call(s, "GET", path, "", "")
		if w.Code != 404 || strings.Contains(w.Body.String(), "UI document") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	w := call(s, "GET", "/boxes/id", "", "")
	if w.Code != 200 || w.Body.String() != "UI document" {
		t.Fatalf("UI fallback: %d %s", w.Code, w.Body)
	}
}

func TestCrossSiteUINavigation(t *testing.T) {
	_, l := testServer(t)
	s, err := New(l, Options{AllowedHosts: []string{"localhost:8473"}, UI: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("UI document")) })})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, path, mode, dest, host, origin string
		status                                       int
	}{
		{"root", "GET", "/", "navigate", "document", "", "", 200},
		{"deep link", "GET", "/boxes/example", "navigate", "document", "", "", 200},
		{"API", "GET", "/api/v1/boxes", "navigate", "document", "", "", 403},
		{"API root", "GET", "/api", "navigate", "document", "", "", 403},
		{"normalized API", "GET", "/ui/../api/v1/boxes", "navigate", "document", "", "", 403},
		{"encoded API", "GET", "/%61pi/v1/boxes", "navigate", "document", "", "", 403},
		{"write", "POST", "/", "navigate", "document", "", "", 403},
		{"iframe", "GET", "/", "navigate", "iframe", "", "", 403},
		{"fetch", "GET", "/", "cors", "empty", "", "", 403},
		{"missing metadata", "GET", "/", "", "", "", "", 403},
		{"foreign host", "GET", "/", "navigate", "document", "evil.example", "", 403},
		{"foreign origin", "GET", "/", "navigate", "document", "", "https://evil.example", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://localhost:8473"+tc.path, nil)
			if tc.host != "" {
				r.Host = tc.host
			}
			r.Header.Set("Sec-Fetch-Site", "cross-site")
			r.Header.Set("Sec-Fetch-Mode", tc.mode)
			r.Header.Set("Sec-Fetch-Dest", tc.dest)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if tc.status == 200 && w.Body.String() != "UI document" {
				t.Fatalf("unexpected UI: %s", w.Body)
			}
		})
	}
}

func TestUploadWASMCSP(t *testing.T) {
	s, _ := testServer(t)
	w := call(s, "GET", Prefix+"/info", "", "")
	policy := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "script-src 'self' 'wasm-unsafe-eval';") || strings.Contains(policy, "'unsafe-eval'") {
		t.Fatalf("WASM must work without enabling JavaScript eval: %s", policy)
	}
}
