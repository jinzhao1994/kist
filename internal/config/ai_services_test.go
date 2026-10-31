package config

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
)

func serviceConfig() *Config {
	c := Defaults("library")
	c.AIServices = &ai.Services{
		NextProxy: 3, NextAccount: 4,
		Proxies: []ai.Proxy{
			{Number: 1, ID: "外部代理", Type: ai.ProxyHTTP, URL: "http://user:synthetic-url-secret@localhost:7890"},
			{Number: 2, ID: "TLS 代理", Type: ai.ProxyTrojan, Host: "proxy.example", Port: 443, Password: "synthetic-trojan-secret"},
		},
		Accounts: []ai.Account{
			{Number: 1, ID: "日常登录", Type: ai.AccountChatGPT, Proxy: 1, Credentials: ai.Credentials{AccessToken: "synthetic-oauth-secret"}},
			{Number: 2, ID: "API 主账户", Type: ai.AccountOpenAI, Proxy: 2, APIKey: "synthetic-key-secret"},
			{Number: 3, ID: "API 备用", Type: ai.AccountOpenAI},
		},
		Features: map[string]ai.Feature{
			"ocr":           {Account: 1, Model: "synthetic-vision", ReasoningEffort: "low", ImageDetail: "original"},
			"transcription": {Account: 2, Model: "gpt-transcribe"},
		},
	}
	return c
}

func TestAIServicesRoundTripAndSecrets(t *testing.T) {
	c := serviceConfig()
	raw, err := Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	next, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := next.AIServices.Account(2); !ok || a.APIKey != "synthetic-key-secret" || a.Proxy != 2 {
		t.Fatal("lost private account data")
	}
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
	public, err := json.Marshal(next.AIServices)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"synthetic-key-secret", "synthetic-oauth-secret", "synthetic-trojan-secret", "synthetic-url-secret"} {
		if strings.Contains(string(public), secret) {
			t.Fatal("private connection data escaped through JSON")
		}
	}
	// Names and list ordering are not references.
	next.AIServices.Proxies[0].ID = "改名"
	next.AIServices.Accounts[1].ID = "新名称"
	next.AIServices.Accounts[0], next.AIServices.Accounts[2] = next.AIServices.Accounts[2], next.AIServices.Accounts[0]
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAIServicesValidationRejectsDanglingAndAmbiguousRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		change      func(*ai.Services)
	}{
		{"empty proxy name", "proxies.0.id", func(s *ai.Services) { s.Proxies[0].ID = "" }},
		{"blank proxy name", "proxies.0.id", func(s *ai.Services) { s.Proxies[0].ID = " \t\u3000" }},
		{"invalid HTTP port", "proxies.0.url", func(s *ai.Services) { s.Proxies[0].URL = "http://proxy.example:65536" }},
		{"invalid HTTP host", "proxies.0.url", func(s *ai.Services) { s.Proxies[0].URL = "http://invalid..example" }},
		{"duplicate proxy ID", "proxies.1.id", func(s *ai.Services) { s.Proxies[1].ID = s.Proxies[0].ID }},
		{"duplicate account ID", "accounts.2.id", func(s *ai.Services) { s.Accounts[2].ID = s.Accounts[1].ID }},
		{"duplicate number", "accounts.2.number", func(s *ai.Services) { s.Accounts[2].Number = 1 }},
		{"unknown proxy", "accounts.1.proxy", func(s *ai.Services) { s.Accounts[1].Proxy = 7 }},
		{"unknown account", "features.ocr.account", func(s *ai.Services) { s.Features["ocr"] = ai.Feature{Account: 7} }},
		{"transcription login", "features.transcription.account", func(s *ai.Services) { s.Features["transcription"] = ai.Feature{Account: 1} }},
		{"transcription model", "features.transcription.model", func(s *ai.Services) { s.Features["transcription"] = ai.Feature{Account: 2, Model: "gpt-6-luna"} }},
		{"transcription controls", "features.transcription.model", func(s *ai.Services) { s.Features["transcription"] = ai.Feature{Account: 2, ImageDetail: "high"} }},
		{"invalid host", "proxies.1.host", func(s *ai.Services) { s.Proxies[1].Host = "https://invalid.example" }},
		{"invalid port", "proxies.1.port", func(s *ai.Services) { s.Proxies[1].Port = 65536 }},
		{"missing password", "proxies.1.password", func(s *ai.Services) { s.Proxies[1].Password = "" }},
		{"automatic unconfigured", "features.desc.account", func(s *ai.Services) { s.Features["desc"] = ai.Feature{Automatic: true} }},
		{"invalid effort", "features.ocr.reasoning_effort", func(s *ai.Services) { s.Features["ocr"] = ai.Feature{ReasoningEffort: "invalid"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := serviceConfig()
			tc.change(c.AIServices)
			err := c.Validate()
			var field *FieldError
			if !errors.As(err, &field) || field.Field != "ai_services."+tc.field {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(err.Error(), "synthetic-") {
				t.Fatal("validation disclosed a secret")
			}
		})
	}
	c := Defaults("library")
	c.AIServices = &ai.Services{NextProxy: 1, NextAccount: 1}
	if err := c.Validate(); err != nil {
		t.Fatal("empty lists rejected", err)
	}
	c.AIServices = nil
	if err := c.Validate(); err != nil {
		t.Fatal("legacy configuration rejected", err)
	}
}
