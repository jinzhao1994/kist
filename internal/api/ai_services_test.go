package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/library"
)

func TestAIServiceSaveCancelsOnlyObsoleteSignIn(t *testing.T) {
	for _, mode := range []string{"rename", "delete", "change type", "clear credentials", "invalid save", "legacy attempt"} {
		t.Run(mode, func(t *testing.T) {
			s, l := testServer(t)
			draft, _ := l.AIServicesSettings(t.Context())
			draft.Accounts = []library.AIAccountSettings{{Number: -1, ID: "ChatGPT", Type: ai.AccountChatGPT}}
			if err := l.UpdateCombinedAIServiceSettings(t.Context(), draft.Revision, map[string]any{}, nil, nil, &draft); err != nil {
				t.Fatal(err)
			}
			draft, _ = l.AIServicesSettings(t.Context())
			cancelled := false
			pending := &openai.Attempt{Cancel: func() { cancelled = true }}
			s.aiAttempt, s.aiAttemptAccount = pending, draft.Accounts[0].Number
			switch mode {
			case "rename":
				draft.Accounts[0].ID = "Renamed"
			case "delete":
				draft.Accounts = nil
			case "change type":
				draft.Accounts[0].Type, draft.Accounts[0].APIKey = ai.AccountOpenAI, "synthetic-new-key"
			case "clear credentials":
				draft.Accounts[0].ClearSecret = true
			case "invalid save":
				draft.Accounts[0].ID = ""
			case "legacy attempt":
				s.aiAttemptAccount = 0
			}
			body, _ := json.Marshal(map[string]any{"revision": draft.Revision, "changes": map[string]any{}, "ai_services": draft})
			r := httptest.NewRequest("PUT", "http://localhost:8473"+Prefix+"/settings", bytes.NewReader(body))
			r.RemoteAddr = "127.0.0.1:1234"
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			wantStatus, wantCancel := 200, mode != "rename" && mode != "invalid save"
			if mode == "invalid save" {
				wantStatus = 400
			}
			if w.Code != wantStatus || cancelled != wantCancel || (s.aiAttempt == nil) != wantCancel {
				t.Fatalf("status=%d cancelled=%v pending=%v", w.Code, cancelled, s.aiAttempt != nil)
			}
			if mode == "change type" {
				view, _ := l.AIServicesSettings(t.Context())
				if len(view.Accounts) != 1 || view.Accounts[0].Type != ai.AccountOpenAI || !view.Accounts[0].Connected {
					t.Fatal("canceling old login cleared the new API account")
				}
			}
		})
	}
}

func TestAIServiceSaveDoesNotCancelANewerSignIn(t *testing.T) {
	s, _ := testServer(t)
	cancelled := false
	s.aiAttempt, s.aiAttemptAccount = &openai.Attempt{Cancel: func() { cancelled = true }}, 1
	apply := s.prepareAISignInChange(&library.AIServiceSettings{})
	newer := &openai.Attempt{Cancel: func() { cancelled = true }}
	s.aiAttempt = newer
	apply()
	if cancelled || s.aiAttempt != newer {
		t.Fatal("settings cleanup canceled a newer login")
	}
}

func TestAIServiceLocalBoundaryIncludesEveryInvocation(t *testing.T) {
	s, _ := testServer(t)
	// Keep the exercised public routes independent of the feature registry used
	// by the guard, including the newer structured and audio extraction paths.
	for _, path := range []string{
		"/ai/services", "/ai/accounts/1/models", "/ai/accounts/1/connect", "/ai/proxies/1/test", "/ai/accounts/1/test",
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

func TestAIProxyDiagnosticReadsOnlySavedConfiguration(t *testing.T) {
	s, l := testServer(t)
	var calls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "CONNECT" || r.Host != "ipwho.is:443" {
			t.Error("unexpected diagnostic destination")
		}
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
	defer proxy.Close()
	draft, _ := l.AIServicesSettings(t.Context())
	draft.Proxies = []library.AIProxySettings{{Number: -1, ID: "Synthetic", Type: ai.ProxyHTTP, URL: proxy.URL}}
	if err := l.UpdateCombinedAIServiceSettings(t.Context(), draft.Revision, map[string]any{}, nil, nil, &draft); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(l.Layout().ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path   string
		status int
	}{{"/ai/proxies/1/test", 200}, {"/ai/proxies/999/test", 404}} {
		r := httptest.NewRequest("POST", "http://localhost:8473"+Prefix+test.path, strings.NewReader("{}"))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		if test.status == 200 && !strings.Contains(w.Body.String(), `"error":"connection"`) {
			t.Fatal(w.Body.String())
		}
	}
	after, _ := os.ReadFile(l.Layout().ConfigPath())
	if calls.Load() != 1 || !bytes.Equal(before, after) {
		t.Fatal("diagnostic mutated settings or made an extra request")
	}
}

func TestAIAccountDiagnosticEndpoint(t *testing.T) {
	s, l := testServer(t)
	draft, _ := l.AIServicesSettings(t.Context())
	draft.Accounts = []library.AIAccountSettings{{Number: -1, ID: "Synthetic", Type: ai.AccountChatGPT}}
	if err := l.UpdateCombinedAIServiceSettings(t.Context(), draft.Revision, map[string]any{}, nil, nil, &draft); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		number string
		status int
	}{{"1", 200}, {"999", 404}, {"0", 422}} {
		r := httptest.NewRequest("POST", "http://localhost:8473"+Prefix+"/ai/accounts/"+test.number+"/test", strings.NewReader("{}"))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%s: %d %s", test.number, w.Code, w.Body.String())
		}
		if test.number == "1" && !strings.Contains(w.Body.String(), `"error":"not_configured"`) {
			t.Fatal(w.Body.String())
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
	var view SettingsResponse
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
