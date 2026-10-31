package library

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
)

func saveServiceDraft(t *testing.T, l *Library, draft AIServiceSettings) AIServiceSettings {
	t.Helper()
	if err := l.UpdateCombinedAIServiceSettings(t.Context(), draft.Revision, map[string]any{}, nil, nil, &draft); err != nil {
		t.Fatal(err)
	}
	view, err := l.AIServicesSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestAIServiceStableNumbersSecretsAndReferences(t *testing.T) {
	l := openTestLibrary(t)
	draft, err := l.AIServicesSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Accounts) != 0 || len(draft.Proxies) != 0 {
		t.Fatal("fresh library should have no connections")
	}
	draft.Proxies = []AIProxySettings{{Number: -1, ID: "代理", Type: ai.ProxyTrojan, Host: "proxy.example", Port: 443, Password: "synthetic-proxy-password"}}
	draft.Accounts = []AIAccountSettings{
		{Number: -1, ID: "API A", Type: ai.AccountOpenAI, Proxy: -1, APIKey: "synthetic-key-a"},
		{Number: -2, ID: "API B", Type: ai.AccountOpenAI, APIKey: "synthetic-key-b"},
	}
	draft.Features["ocr"] = ai.Feature{Account: -1, Model: "synthetic-a", ReasoningEffort: "high", ImageDetail: "original"}
	draft.Features["orientation"] = ai.Feature{Account: -2, Model: "synthetic-b", ReasoningEffort: "low", ImageDetail: "low"}
	draft = saveServiceDraft(t, l, draft)
	if draft.Proxies[0].Number != 1 || draft.Accounts[0].Number != 1 || draft.Accounts[1].Number != 2 || draft.Accounts[0].Proxy != 1 || draft.Features["ocr"].Account != 1 {
		t.Fatal("temporary references were not resolved")
	}
	public, _ := json.Marshal(draft)
	if strings.Contains(string(public), "synthetic-key-") || strings.Contains(string(public), "synthetic-proxy-password") {
		t.Fatal("settings leaked credentials")
	}
	stale := draft
	draft.Accounts[0].ID = "重命名"
	draft.Accounts[0], draft.Accounts[1] = draft.Accounts[1], draft.Accounts[0]
	draft = saveServiceDraft(t, l, draft)
	if draft.Features["ocr"].Account != 1 {
		t.Fatal("rename or reordering changed route")
	}
	if err := l.UpdateCombinedAIServiceSettings(t.Context(), stale.Revision, map[string]any{}, nil, nil, &stale); !errors.Is(err, config.ErrChanged) {
		t.Fatal("accepted stale settings", err)
	}
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := cfg.AIServices.Account(1)
	p, _ := cfg.AIServices.Proxy(1)
	if a.APIKey != "synthetic-key-a" || p.Password != "synthetic-proxy-password" {
		t.Fatal("redacted save lost secrets")
	}
	// Referenced objects cannot disappear, and a failed save changes nothing.
	before, _ := os.ReadFile(l.layout.ConfigPath())
	draft.Proxies = nil
	if err := l.UpdateCombinedAIServiceSettings(t.Context(), draft.Revision, map[string]any{}, nil, nil, &draft); err == nil {
		t.Fatal("deleted referenced proxy")
	}
	after, _ := os.ReadFile(l.layout.ConfigPath())
	if string(before) != string(after) {
		t.Fatal("invalid draft partially persisted")
	}
	draft, _ = l.AIServicesSettings(t.Context())
	delete(draft.Features, "ocr")
	draft.Accounts = draft.Accounts[:1] // Account B remains; A was moved to second.
	draft.Proxies = nil
	draft = saveServiceDraft(t, l, draft)
	draft.Accounts = append(draft.Accounts, AIAccountSettings{Number: -1, ID: "API C", Type: ai.AccountOpenAI})
	draft = saveServiceDraft(t, l, draft)
	if draft.Accounts[1].Number != 3 {
		t.Fatal("deleted account number was reused")
	}
}

func TestAIServicePerFeatureProviderAndSnapshot(t *testing.T) {
	l := openTestLibrary(t)
	draft, _ := l.AIServicesSettings(t.Context())
	draft.Accounts = []AIAccountSettings{{Number: -1, ID: "A", Type: ai.AccountOpenAI, APIKey: "synthetic-a"}, {Number: -2, ID: "B", Type: ai.AccountOpenAI, APIKey: "synthetic-b"}}
	draft.Features["ocr"] = ai.Feature{Account: -1, Model: "model-a", ReasoningEffort: "high", ImageDetail: "original"}
	draft.Features["orientation"] = ai.Feature{Account: -2, Model: "model-b", ReasoningEffort: "low", ImageDetail: "low"}
	draft = saveServiceDraft(t, l, draft)
	cfg, _ := config.Load(l.layout.ConfigPath())
	ctx := context.WithValue(t.Context(), aiConfigurationContextKey{}, cfg)
	for kind, want := range map[string]string{"ocr": "synthetic-a", "orientation": "synthetic-b"} {
		provider, err := l.AIFeatureProvider(ctx, kind)
		if err != nil {
			t.Fatal(err)
		}
		client := provider.(openai.Client)
		if client.Token != want || !client.APIKey {
			t.Fatal("wrong account selected")
		}
		feature := draft.Features[kind]
		if client.Parameters.ReasoningEffort != feature.ReasoningEffort || client.Parameters.ImageDetail != feature.ImageDetail {
			t.Fatal("wrong inference parameters")
		}
	}
	feature := draft.Features["ocr"]
	feature.Account = 2
	draft.Features["ocr"] = feature
	draft = saveServiceDraft(t, l, draft)
	provider, err := l.AIFeatureProvider(ctx, "ocr")
	if err != nil || provider.(openai.Client).Token != "synthetic-a" {
		t.Fatal("running snapshot changed", err)
	}
	provider, err = l.AIFeatureProvider(t.Context(), "ocr")
	if err != nil || provider.(openai.Client).Token != "synthetic-b" {
		t.Fatal("new task did not see changes", err)
	}
	if _, err = l.AIFeatureProvider(t.Context(), "desc"); err == nil {
		t.Fatal("unconfigured feature silently chose an account")
	}
}

func TestAIServiceLegacyConversionAndAccountTokenIsolation(t *testing.T) {
	l := openTestLibrary(t)
	settings, _ := l.OCRSettings(t.Context())
	settings.Model = "legacy-model"
	settings.ProxyURL = "http://user:synthetic-secret@127.0.0.1:7890"
	if _, err := l.SaveOCRSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	credentials := ai.Credentials{HostID: "legacy-host", Provider: "openai", AccessToken: "synthetic-legacy-token", RefreshToken: "synthetic-refresh"}
	if err := l.SaveAIConnection(t.Context(), ai.Credentials{}, credentials); err != nil {
		t.Fatal(err)
	}
	draft, _ := l.AIServicesSettings(t.Context())
	if len(draft.Accounts) != 1 || len(draft.Proxies) != 1 || strings.Contains(draft.Proxies[0].URL, "synthetic-secret") {
		t.Fatal("bad legacy conversion")
	}
	draft.Accounts = append(draft.Accounts, AIAccountSettings{Number: -1, ID: "另一个登录", Type: ai.AccountChatGPT})
	draft = saveServiceDraft(t, l, draft)
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AI != (ai.Credentials{}) || cfg.AIOptions.Model != "" {
		t.Fatal("legacy fields remained active")
	}
	account, _ := cfg.AIServices.Account(1)
	if account.Credentials != credentials || cfg.AIServices.Features["ocr"].Model != "legacy-model" {
		t.Fatal("lost legacy connection or model")
	}
	second, err := l.PrepareAIAccountConnection(t.Context(), 2)
	if err != nil || second.HostID == "" {
		t.Fatal(err)
	}
	connected := second
	connected.Provider = "openai"
	connected.AccessToken = "synthetic-second-token"
	if err := l.SaveAIAccountConnection(t.Context(), 2, second, connected); err != nil {
		t.Fatal(err)
	}
	if err := l.SaveAIAccountConnection(t.Context(), 2, second, connected); !errors.Is(err, config.ErrChanged) {
		t.Fatal("accepted stale callback", err)
	}
	cfg, _ = config.Load(l.layout.ConfigPath())
	account, _ = cfg.AIServices.Account(1)
	if account.Credentials != credentials {
		t.Fatal("another login overwrote first account")
	}
}
