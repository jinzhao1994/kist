package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIOnlyAcceptsLoopbackRequests(t *testing.T) {
	s, _ := testServer(t)
	for _, tc := range []struct {
		addr, host string
		code       int
	}{{"192.0.2.1:1234", "localhost:8473", 403}, {"127.0.0.1:1234", "localhost:8473", 200}, {"127.0.0.1:1234", "archive.example:8473", 403}} {
		req := httptest.NewRequest("GET", "http://"+tc.host+Prefix+"/ai", nil)
		req.RemoteAddr = tc.addr
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "access_token") {
			t.Fatal("secret field exposed")
		}
	}
}

func TestAISettingsAndQueueRoutes(t *testing.T) {
	s, _ := testServer(t)
	callLocal := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost:8473"+Prefix+path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	settings := callLocal("GET", "/ai/settings", "")
	if settings.Code != 200 {
		t.Fatal(settings.Code, settings.Body.String())
	}
	var value map[string]any
	if err := json.Unmarshal(settings.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	value["model"] = "gpt-6-luna"
	value["auto_ocr"] = true
	value["auto_desc"] = true
	value["proxy_url"] = "http://127.0.0.1:7890"
	body, _ := json.Marshal(value)
	if got := callLocal("PUT", "/ai/settings", string(body)); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := callLocal("PUT", "/ai/settings", string(body)); got.Code != 409 {
		t.Fatal("stale update", got.Code)
	}
	for _, test := range []struct{ method, path, body string }{{"GET", "/ai/desc", ""}, {"POST", "/ai/desc/pause", `{"paused":true}`}, {"POST", "/ai/desc/backfill", `{"retry":false}`}, {"GET", "/ai/ocr", ""}, {"POST", "/ai/ocr/pause", `{"paused":true}`}, {"POST", "/ai/ocr/backfill", `{"retry":false}`}} {
		if got := callLocal(test.method, test.path, test.body); got.Code != 200 {
			t.Fatal(test.path, got.Code, got.Body.String())
		}
	}
}

func TestDescriptionFileRouteIsLocalOnly(t *testing.T) {
	s, _ := testServer(t)
	for _, method := range []string{"GET", "POST"} {
		r := httptest.NewRequest(method, "http://localhost:8473"+Prefix+"/files/01a1737a-ceb0-7392-a1a9-a5ecb0b7429e/desc", strings.NewReader(`{"retry":false}`))
		r.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
