package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/library"
)

func TestSignInDraftSaveIsAtomicPrivateAndSingleUse(t *testing.T) {
	s, l := testServer(t)
	settings, _ := l.AIServicesSettings(t.Context())
	settings.Accounts = []library.AIAccountSettings{{Number: -7, ID: "Unsaved account", Type: ai.AccountChatGPT, LoginDraft: "synthetic-capability"}}
	credentials := ai.Credentials{Provider: "openai", HostID: "synthetic-host", ClientID: "synthetic-client", Subject: "synthetic-person", Email: "person@example.test", AccessToken: "private-access", RefreshToken: "private-refresh", IDToken: "private-id", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	cancelled := false
	draft := &signInDraft{number: -7, credentials: credentials, expires: time.Now().Add(time.Minute), attempt: &openai.Attempt{Cancel: func() { cancelled = true }}}
	s.aiDrafts.entries = map[string]*signInDraft{"synthetic-capability": draft}
	before, _ := os.ReadFile(l.Layout().ConfigPath())
	status := localDraftCall(s, "GET", Prefix+"/ai/sign-ins/synthetic-capability", "", "")
	if status.Code != 200 || !strings.Contains(status.Body.String(), `"connected":true`) {
		t.Fatal(status.Code, status.Body.String())
	}
	for _, secret := range []string{credentials.AccessToken, credentials.RefreshToken, credentials.IDToken} {
		if strings.Contains(status.Body.String(), secret) {
			t.Fatal("draft status exposed token")
		}
	}
	save := func(changes map[string]any) int {
		raw, _ := json.Marshal(map[string]any{"revision": settings.Revision, "changes": changes, "ai_services": settings})
		w := localDraftCall(s, "PUT", Prefix+"/settings", "", string(raw))
		if w.Code != 200 && !strings.Contains(w.Body.String(), "ai_services.accounts.0.login_draft") && !strings.Contains(w.Body.String(), "appearance.primary_color") {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Code
	}
	if save(map[string]any{"appearance.primary_color": "invalid"}) != 400 || cancelled || s.aiDrafts.entries["synthetic-capability"] == nil {
		t.Fatal("failed save consumed authorization")
	}
	after, _ := os.ReadFile(l.Layout().ConfigPath())
	if string(before) != string(after) {
		t.Fatal("failed save modified config")
	}
	if save(map[string]any{"appearance.primary_color": "green"}) != 200 || !cancelled {
		t.Fatal("save did not consume authorization")
	}
	cfg, _ := config.Load(l.Layout().ConfigPath())
	if len(cfg.AIServices.Accounts) != 1 || cfg.AIServices.Accounts[0].Number != 1 || cfg.AIServices.Accounts[0].Credentials != credentials || cfg.Appearance.PrimaryColor != "green" {
		t.Fatal("credentials were not saved with settings")
	}
	view, _ := l.AIServicesSettings(t.Context())
	settings.Revision = view.Revision
	settings.Accounts[0].Number = 1
	if save(map[string]any{}) != 400 {
		t.Fatal("consumed draft reused")
	}
}

func TestSignInDraftRejectsExpiredUnfinishedAndMismatchedTargets(t *testing.T) {
	for _, mode := range []string{"expired", "pending", "failed", "other account", "type", "cleared", "missing", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			s, l := testServer(t)
			settings, _ := l.AIServicesSettings(t.Context())
			settings.Accounts = []library.AIAccountSettings{{Number: -1, ID: "A", Type: ai.AccountChatGPT, LoginDraft: "capability"}}
			draft := &signInDraft{number: -1, credentials: ai.Credentials{AccessToken: "synthetic"}, expires: time.Now().Add(time.Minute)}
			s.aiDrafts.entries = map[string]*signInDraft{"capability": draft}
			switch mode {
			case "expired":
				draft.expires = time.Now().Add(-time.Second)
			case "pending":
				draft.pending = true
			case "failed":
				draft.failure = "failed"
			case "other account":
				draft.number = -2
			case "type":
				settings.Accounts[0].Type = ai.AccountOpenAI
			case "cleared":
				settings.Accounts[0].ClearSecret = true
			case "missing":
				delete(s.aiDrafts.entries, "capability")
			case "duplicate":
				settings.Accounts = append(settings.Accounts, settings.Accounts[0])
			}
			raw, _ := json.Marshal(map[string]any{"revision": settings.Revision, "changes": map[string]any{}, "ai_services": settings})
			w := localDraftCall(s, "PUT", Prefix+"/settings", "", string(raw))
			if w.Code != 400 || !strings.Contains(w.Body.String(), "login_draft") {
				t.Fatal(w.Code, w.Body.String())
			}
			view, _ := l.AIServicesSettings(t.Context())
			if len(view.Accounts) != 0 {
				t.Fatal("invalid authorization modified config")
			}
		})
	}
}

func TestDraftLoginStartsWithoutSavingAndCancels(t *testing.T) {
	s, l := testServer(t)
	settings, _ := l.AIServicesSettings(t.Context())
	settings.Proxies = []library.AIProxySettings{{Number: -5, ID: "Unsaved proxy", Type: ai.ProxyHTTP, URL: "http://127.0.0.1:9"}}
	settings.Accounts = []library.AIAccountSettings{{Number: -8, ID: "Unsaved", Type: ai.AccountChatGPT, Proxy: -5}}
	before, _ := os.ReadFile(l.Layout().ConfigPath())
	raw, _ := json.Marshal(map[string]any{"number": -8, "settings": settings})
	w := localDraftCall(s, "POST", Prefix+"/ai/sign-ins", "", string(raw))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result signInDraftView
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.ID == "" || !result.Pending || !strings.HasPrefix(result.URL, "https://auth.openai.com/") {
		t.Fatal("invalid draft result", err)
	}
	after, _ := os.ReadFile(l.Layout().ConfigPath())
	if string(before) != string(after) {
		t.Fatal("starting login persisted account or proxy")
	}
	w = localDraftCall(s, "DELETE", Prefix+"/ai/sign-ins/"+result.ID, "", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = localDraftCall(s, "GET", Prefix+"/ai/sign-ins/"+result.ID, "", ""); w.Code != 404 {
		t.Fatal("cancelled draft remained readable")
	}
	after, _ = os.ReadFile(l.Layout().ConfigPath())
	if string(before) != string(after) {
		t.Fatal("cancel wrote config")
	}
}

func localDraftCall(s *Server, method, path, key, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost:8473"+path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestAccountRenewalFailureDoesNotOutliveCredentials(t *testing.T) {
	s, l := testServer(t)
	settings, _ := l.AIServicesSettings(t.Context())
	settings.Accounts = []library.AIAccountSettings{{Number: -1, ID: "Synthetic", Type: ai.AccountChatGPT}}
	if err := l.UpdateCombinedAIServiceSettings(t.Context(), settings.Revision, map[string]any{}, nil, nil, &settings); err != nil {
		t.Fatal(err)
	}
	saved := ai.Credentials{AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh", Provider: "openai"}
	if err := l.SaveAIAccountConnection(t.Context(), 1, ai.Credentials{}, saved); err != nil {
		t.Fatal(err)
	}
	view, _ := l.AIServicesSettings(t.Context())
	s.aiRenewals = map[int64]library.AIAccountRenewal{1: {Version: view.Accounts[0].CredentialVersion, Error: "reauthenticate"}}
	w := localDraftCall(s, "GET", Prefix+"/ai/services", "", "")
	if !strings.Contains(w.Body.String(), `"authentication_error":"reauthenticate"`) {
		t.Fatal(w.Body.String())
	}
	if strings.Contains(w.Body.String(), saved.AccessToken) || strings.Contains(w.Body.String(), view.Accounts[0].CredentialVersion) {
		t.Fatal("renewal status leaked private state")
	}
	if err := l.SaveAIAccountConnection(t.Context(), 1, saved, ai.Credentials{}); err != nil {
		t.Fatal(err)
	}
	w = localDraftCall(s, "GET", Prefix+"/ai/services", "", "")
	if strings.Contains(w.Body.String(), "authentication_error") {
		t.Fatal("obsolete error survived disconnect")
	}
}
