package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/library"
)

func TestAIServiceLocalBoundaryIncludesEveryInvocation(t *testing.T) {
	s, _ := testServer(t)
	// Keep the exercised public routes independent of the feature registry used
	// by the guard, including the newer structured and audio extraction paths.
	for _, path := range []string{
		"/ai/services", "/ai/accounts/1/models", "/ai/accounts/1/connect",
		"/files/00000000-0000-7000-8000-000000000001/orientation",
		"/files/00000000-0000-7000-8000-000000000001/ocr",
		"/files/00000000-0000-7000-8000-000000000001/desc",
		"/files/00000000-0000-7000-8000-000000000001/facts",
		"/files/00000000-0000-7000-8000-000000000001/transcription",
	} {
		for _, method := range []string{"GET", "POST"} {
			r := httptest.NewRequest(method, "http://localhost:8473"+Prefix+path, strings.NewReader("{}"))
			r.RemoteAddr = "192.0.2.1:1234"
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Errorf("%s %s bypassed loopback boundary: %d", method, path, w.Code)
			}
		}
	}
	for _, path := range []string{
		"/files/id/metadata", "/files/id/metadata/ocr:hint",
		"/files/id/metadata/transcription:hint", "/files/id/ocr/nested",
		"/files/id/image-rotation", "/files/id/content", "/files/id/geo",
	} {
		if requiresLocalAI(Prefix + path) {
			t.Errorf("ordinary metadata/content route treated as inference: %s", path)
		}
	}
}

func TestAIServiceSettingsAtomicSaveAndRedaction(t *testing.T) {
	s, _ := testServer(t)
	call := func(method, path string, value any, remote string) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(value)
		r := httptest.NewRequest(method, "http://localhost:8473"+Prefix+path, bytes.NewReader(raw))
		r.RemoteAddr = remote
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	get := call("GET", "/ai/services", nil, "127.0.0.1:1234")
	if get.Code != 200 {
		t.Fatal(get.Code, get.Body.String())
	}
	var draft library.AIServiceSettings
	if err := json.Unmarshal(get.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	draft.Proxies = []library.AIProxySettings{{Number: -1, ID: "代理", Type: ai.ProxyHTTP, URL: "http://user:synthetic-password@127.0.0.1:7890"}}
	draft.Accounts = []library.AIAccountSettings{{Number: -1, ID: "API", Type: ai.AccountOpenAI, Proxy: -1, APIKey: "synthetic-api-key"}}
	draft.Features["ocr"] = ai.Feature{Account: -1, Model: "synthetic-vision", ReasoningEffort: "low", ImageDetail: "original"}
	body := map[string]any{"revision": draft.Revision, "changes": map[string]any{"appearance.primary_color": "green"}, "ai_services": draft}
	if result := call("PUT", "/settings", body, "192.0.2.1:1234"); result.Code != 403 {
		t.Fatal("remote request bypassed local AI restriction", result.Code)
	}
	result := call("PUT", "/settings", body, "127.0.0.1:1234")
	if result.Code != 200 {
		t.Fatal(result.Code, result.Body.String())
	}
	if result := call("PUT", "/settings", body, "127.0.0.1:1234"); result.Code != 409 {
		t.Fatal("accepted stale draft", result.Code)
	}
	get = call("GET", "/ai/services", nil, "127.0.0.1:1234")
	if get.Code != 200 {
		t.Fatal(get.Code, get.Body.String())
	}
	for _, secret := range []string{"synthetic-api-key", "synthetic-password"} {
		if strings.Contains(get.Body.String(), secret) {
			t.Fatal("AI service response disclosed secret")
		}
	}
	if err := json.Unmarshal(get.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if !draft.Accounts[0].Connected || !draft.Proxies[0].SecretConfigured || draft.Features["ocr"].Account != draft.Accounts[0].Number {
		t.Fatal("incorrect connection view")
	}
	draft.Proxies = nil
	body = map[string]any{"revision": draft.Revision, "changes": map[string]any{"appearance.primary_color": "pink"}, "ai_services": draft}
	result = call("PUT", "/settings", body, "127.0.0.1:1234")
	if result.Code != 400 || !strings.Contains(result.Body.String(), "ai_services.accounts.0.proxy") {
		t.Fatal("missing field error", result.Code, result.Body.String())
	}
	settings := call("GET", "/settings", nil, "127.0.0.1:1234")
	var view settingsResponse
	if err := json.Unmarshal(settings.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Effective["appearance.primary_color"] != "green" {
		t.Fatal("invalid AI draft partially saved general settings")
	}
	if result := call("GET", "/ai/accounts/999/models", nil, "127.0.0.1:1234"); result.Code != 400 {
		t.Fatal("unknown account accepted", result.Code)
	}
	if result := call("POST", "/ai/accounts/1/connect", map[string]any{}, "127.0.0.1:1234"); result.Code == 200 {
		t.Fatal("API key account attempted OAuth")
	}
}
