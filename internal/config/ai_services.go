package config

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jinzhao1994/kist/internal/ai"
)

const maxObjectNumber = 9007199254740991

func validateAIServices(s *ai.Services) error {
	if s == nil {
		return nil
	}
	if s.NextProxy < 1 || s.NextProxy > maxObjectNumber || s.NextAccount < 1 || s.NextAccount > maxObjectNumber {
		return InvalidField("ai_services", "number", "Invalid AI object number allocation.")
	}
	proxyNumbers, proxyIDs := map[int64]bool{}, map[string]bool{}
	for i, p := range s.Proxies {
		field := fmt.Sprintf("ai_services.proxies.%d", i)
		if p.Number < 1 || p.Number >= s.NextProxy || proxyNumbers[p.Number] {
			return InvalidField(field+".number", "number", "Proxy numbers must be unique and allocated automatically.")
		}
		if strings.TrimSpace(p.ID) == "" {
			return InvalidField(field+".id", "required", "Enter a proxy name.")
		}
		if !validObjectID(p.ID) || proxyIDs[p.ID] {
			return InvalidField(field+".id", "unique_id", "Enter a unique proxy ID, at most 128 UTF-8 bytes.")
		}
		proxyNumbers[p.Number], proxyIDs[p.ID] = true, true
		switch p.Type {
		case ai.ProxyHTTP:
			if p.URL == "" || len(p.URL) > 2048 {
				return InvalidField(field+".url", "proxy", "Enter an HTTP(S) or SOCKS proxy URL.")
			}
			if _, err := ai.HTTPClient(p.URL, time.Second); err != nil {
				return InvalidField(field+".url", "proxy", "Enter an HTTP(S) or SOCKS proxy URL.")
			}
			if p.Host != "" || p.Port != 0 || p.Password != "" || p.ServerName != "" {
				return InvalidField(field+".type", "choice", "HTTP proxies cannot contain Trojan settings.")
			}
		case ai.ProxyTrojan:
			if !ai.ValidNetworkHost(p.Host) {
				return InvalidField(field+".host", "host", "Enter a proxy hostname or IP address without a scheme or port.")
			}
			if p.Port < 1 || p.Port > 65535 {
				return InvalidField(field+".port", "port", "Enter a port between 1 and 65535.")
			}
			if len(p.Password) < 1 || len(p.Password) > 4096 {
				return InvalidField(field+".password", "required", "Enter a Trojan password, at most 4096 bytes.")
			}
			if p.ServerName != "" && !ai.ValidNetworkHost(p.ServerName) {
				return InvalidField(field+".server_name", "host", "Enter a valid TLS server name.")
			}
			if p.URL != "" {
				return InvalidField(field+".type", "choice", "Trojan proxies cannot contain an HTTP proxy URL.")
			}
		default:
			return InvalidField(field+".type", "choice", "Choose HTTP or Trojan.")
		}
	}
	accountNumbers, accountIDs := map[int64]bool{}, map[string]bool{}
	for i, a := range s.Accounts {
		field := fmt.Sprintf("ai_services.accounts.%d", i)
		if a.Number < 1 || a.Number >= s.NextAccount || accountNumbers[a.Number] {
			return InvalidField(field+".number", "number", "Account numbers must be unique and allocated automatically.")
		}
		if !validObjectID(a.ID) || accountIDs[a.ID] {
			return InvalidField(field+".id", "unique_id", "Enter a unique account ID, at most 128 UTF-8 bytes.")
		}
		accountNumbers[a.Number], accountIDs[a.ID] = true, true
		if a.Proxy < 0 || a.Proxy != 0 && !proxyNumbers[a.Proxy] {
			return InvalidField(field+".proxy", "reference", "Choose an existing proxy or a direct connection.")
		}
		switch a.Type {
		case ai.AccountChatGPT:
			if a.APIKey != "" {
				return InvalidField(field+".type", "choice", "ChatGPT accounts cannot contain an API key.")
			}
		case ai.AccountOpenAI:
			// An unconfigured account may be saved before its key is entered.
			if len(a.APIKey) > 4096 || strings.ContainsAny(a.APIKey, "\r\n\t ") || a.Credentials != (ai.Credentials{}) {
				return InvalidField(field+".api_key", "api_key", "Enter an API key without whitespace; OAuth credentials are not supported here.")
			}
		default:
			return InvalidField(field+".type", "choice", "Choose ChatGPT or OpenAI API.")
		}
	}
	for name, f := range s.Features {
		field := "ai_services.features." + name
		if !slices.Contains(ai.FeatureNames(), name) {
			return InvalidField(field, "choice", "Unknown AI feature.")
		}
		if f.Account < 0 || f.Account != 0 && !accountNumbers[f.Account] {
			return InvalidField(field+".account", "reference", "Choose an existing AI account.")
		}
		if len(f.Model) > 200 || strings.TrimSpace(f.Model) != f.Model || strings.ContainsAny(f.Model, "\r\n\x00") {
			return InvalidField(field+".model", "model", "Enter a model identifier, at most 200 bytes.")
		}
		if f.Automatic && (f.Account == 0 || ai.FeatureModel(name, f) == "") {
			return InvalidField(field+".account", "model_required", "Choose an account and model before enabling automatic processing.")
		}
		if !slices.Contains(ai.ParameterValues("reasoning_effort"), f.ReasoningEffort) {
			return InvalidField(field+".reasoning_effort", "choice", "Choose a supported reasoning effort.")
		}
		if !slices.Contains(ai.ParameterValues("image_detail"), f.ImageDetail) {
			return InvalidField(field+".image_detail", "choice", "Choose a supported image detail.")
		}
		if f.AfterOp < 0 {
			return InvalidField(field+".after_op", "number", "Invalid automatic processing boundary.")
		}
		if name == "transcription" {
			if !ai.SupportedTranscriptionModel(ai.FeatureModel(name, f)) {
				return InvalidField(field+".model", "transcription_model", "Choose a model supported by the file transcription endpoint.")
			}
			if f.ReasoningEffort != "" || f.ImageDetail != "" {
				return InvalidField(field+".model", "capability", "Transcription does not accept image or reasoning parameters.")
			}
			if a, ok := s.Account(f.Account); ok && a.Type != ai.AccountOpenAI {
				return InvalidField(field+".account", "capability", "Transcription requires an OpenAI API account.")
			}
		}
	}
	return nil
}

func validObjectID(id string) bool {
	return id != "" && len(id) <= 128 && utf8.ValidString(id) && strings.TrimSpace(id) == id && !strings.ContainsFunc(id, unicode.IsControl)
}
