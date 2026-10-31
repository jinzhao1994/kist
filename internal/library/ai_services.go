package library

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"os"
	"slices"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/store"
)

// AIServiceSettings is a redacted, revisioned settings draft. Secret inputs are
// write-only in practice: views always clear them rather than masking values.
type AIServiceSettings struct {
	Revision string                `json:"revision"`
	Proxies  []AIProxySettings     `json:"proxies" maxItems:"128"`
	Accounts []AIAccountSettings   `json:"accounts" maxItems:"128"`
	Features map[string]ai.Feature `json:"features"`
}

type AIProxySettings struct {
	Number           int64  `json:"number"`
	ID               string `json:"id" maxLength:"128"`
	Type             string `json:"type" enum:"http,trojan"`
	URL              string `json:"url" maxLength:"2048"`
	Host             string `json:"host" maxLength:"253"`
	Port             int    `json:"port"`
	ServerName       string `json:"server_name" maxLength:"253"`
	Password         string `json:"password,omitempty" writeOnly:"true" maxLength:"4096"`
	ClearSecret      bool   `json:"clear_secret,omitempty"`
	SecretConfigured bool   `json:"secret_configured" readOnly:"true"`
}

type AIAccountSettings struct {
	CredentialVersion   string `json:"-"`
	AuthenticationError string `json:"authentication_error,omitempty" readOnly:"true" enum:",temporary,reauthenticate"`
	LoginDraft          string `json:"login_draft,omitempty" maxLength:"128" writeOnly:"true"`
	// AuthorizedCredentials is populated only by the server's authorization vault.
	AuthorizedCredentials *ai.Credentials `json:"-"`
	Number                int64           `json:"number"`
	ID                    string          `json:"id" maxLength:"128"`
	Type                  string          `json:"type" enum:"chatgpt,openai"`
	Proxy                 int64           `json:"proxy"`
	APIKey                string          `json:"api_key,omitempty" writeOnly:"true" maxLength:"4096"`
	ClearSecret           bool            `json:"clear_secret,omitempty"`
	Connected             bool            `json:"connected" readOnly:"true"`
	Email                 string          `json:"email" readOnly:"true"`
}

// effectiveAIServices interprets legacy settings without writing or signing in.
// The first explicit save atomically switches to the new representation.
func effectiveAIServices(cfg *config.Config) ai.Services {
	if cfg.AIServices != nil {
		s := *cfg.AIServices
		s.Proxies = slices.Clone(s.Proxies)
		s.Accounts = slices.Clone(s.Accounts)
		s.Features = maps.Clone(s.Features)
		if s.Features == nil {
			s.Features = map[string]ai.Feature{}
		}
		f := s.Features["transcription"]
		f.Model = ai.FeatureModel("transcription", f)
		s.Features["transcription"] = f
		return s
	}
	s := ai.Services{NextProxy: 1, NextAccount: 1, Features: map[string]ai.Feature{}}
	proxy := int64(0)
	if cfg.AIOptions.ProxyURL != "" {
		proxy = 1
		s.NextProxy = 2
		s.Proxies = append(s.Proxies, ai.Proxy{Number: 1, ID: "默认代理", Type: ai.ProxyHTTP, URL: cfg.AIOptions.ProxyURL})
	}
	account := int64(0)
	if cfg.AI != (ai.Credentials{}) || cfg.AIOptions.Model != "" {
		account = 1
		s.NextAccount = 2
		s.Accounts = append(s.Accounts, ai.Account{Number: 1, ID: "ChatGPT", Type: ai.AccountChatGPT, Proxy: proxy, Credentials: cfg.AI})
	}
	legacy := map[string]ai.Feature{
		"ocr":         {Automatic: cfg.AIOptions.AutoOCR, AfterOp: cfg.AIOptions.AfterOp},
		"desc":        {Automatic: cfg.AIOptions.AutoDesc, AfterOp: cfg.AIOptions.DescAfterOp},
		"facts":       {Automatic: cfg.AIOptions.AutoFacts, AfterOp: cfg.AIOptions.FactsAfterOp},
		"orientation": {Automatic: cfg.AIOptions.AutoOrientation, AfterOp: cfg.AIOptions.OrientationAfterOp},
	}
	for _, name := range ai.FeatureNames() {
		f := legacy[name]
		if name != "transcription" {
			f.Account, f.Model, f.ReasoningEffort, f.ImageDetail = account, cfg.AIOptions.Model, "low", "original"
		}
		f.Model = ai.FeatureModel(name, f)
		s.Features[name] = f
	}
	return s
}

func (l *Library) AIServicesSettings(ctx context.Context) (AIServiceSettings, error) {
	if err := ctx.Err(); err != nil {
		return AIServiceSettings{}, err
	}
	raw, err := os.ReadFile(l.layout.ConfigPath())
	if err != nil {
		return AIServiceSettings{}, err
	}
	cfg, err := config.Decode(raw)
	if err != nil {
		return AIServiceSettings{}, err
	}
	s := effectiveAIServices(cfg)
	out := AIServiceSettings{Revision: config.Revision(raw), Proxies: []AIProxySettings{}, Accounts: []AIAccountSettings{}, Features: s.Features}
	for _, p := range s.Proxies {
		v := AIProxySettings{Number: p.Number, ID: p.ID, Type: p.Type, URL: p.URL, Host: p.Host, Port: p.Port, ServerName: p.ServerName, SecretConfigured: p.Password != ""}
		if u, err := url.Parse(p.URL); err == nil && u.User != nil {
			v.SecretConfigured = true
			u.User = nil
			v.URL = u.String()
		}
		out.Proxies = append(out.Proxies, v)
	}
	for _, a := range s.Accounts {
		out.Accounts = append(out.Accounts, AIAccountSettings{Number: a.Number, ID: a.ID, Type: a.Type, Proxy: a.Proxy, Connected: a.APIKey != "" || a.Credentials.AccessToken != "", Email: a.Credentials.Email, CredentialVersion: credentialVersion(a.Credentials)})
	}
	return out, nil
}

func (l *Library) aiServicesChanges(cfg *config.Config, in AIServiceSettings) (map[string]any, error) {
	old := effectiveAIServices(cfg)
	next := ai.Services{NextProxy: old.NextProxy, NextAccount: old.NextAccount, Proxies: []ai.Proxy{}, Accounts: []ai.Account{}, Features: map[string]ai.Feature{}}
	proxyRefs, accountRefs := map[int64]int64{}, map[int64]int64{}
	for i, p := range in.Proxies {
		field := fmt.Sprintf("ai_services.proxies.%d", i)
		before, exists := old.Proxy(p.Number)
		number := p.Number
		if number <= 0 {
			if number != 0 {
				if _, duplicate := proxyRefs[number]; duplicate {
					return nil, config.InvalidField(field+".number", "number", "Duplicate temporary proxy number.")
				}
			}
			number = next.NextProxy
			next.NextProxy++
			if p.Number != 0 {
				proxyRefs[p.Number] = number
			}
		} else if !exists {
			return nil, config.InvalidField(field+".number", "number", "Unknown proxy number; create a new proxy instead.")
		}
		v := ai.Proxy{Number: number, ID: p.ID, Type: p.Type, URL: p.URL, Host: p.Host, Port: p.Port, Password: p.Password, ServerName: p.ServerName}
		if exists && before.Type == v.Type && !p.ClearSecret {
			if v.Type == ai.ProxyTrojan && v.Password == "" {
				v.Password = before.Password
			}
			if v.Type == ai.ProxyHTTP {
				u, err := url.Parse(v.URL)
				previous, prevErr := url.Parse(before.URL)
				if err == nil && prevErr == nil && u.User == nil {
					u.User = previous.User
					v.URL = u.String()
				}
			}
		}
		if p.ClearSecret {
			v.Password = ""
			if u, err := url.Parse(v.URL); err == nil {
				u.User = nil
				v.URL = u.String()
			}
		}
		next.Proxies = append(next.Proxies, v)
	}
	for i, a := range in.Accounts {
		field := fmt.Sprintf("ai_services.accounts.%d", i)
		before, exists := old.Account(a.Number)
		number := a.Number
		if number <= 0 {
			if number != 0 {
				if _, duplicate := accountRefs[number]; duplicate {
					return nil, config.InvalidField(field+".number", "number", "Duplicate temporary account number.")
				}
			}
			number = next.NextAccount
			next.NextAccount++
			if a.Number != 0 {
				accountRefs[a.Number] = number
			}
		} else if !exists {
			return nil, config.InvalidField(field+".number", "number", "Unknown account number; create a new account instead.")
		}
		proxy := a.Proxy
		if proxy < 0 {
			proxy = proxyRefs[proxy]
			if proxy == 0 {
				return nil, config.InvalidField(field+".proxy", "reference", "Unknown temporary proxy.")
			}
		}
		v := ai.Account{Number: number, ID: a.ID, Type: a.Type, Proxy: proxy, APIKey: a.APIKey}
		if exists && before.Type == v.Type && !a.ClearSecret {
			if v.Type == ai.AccountChatGPT {
				v.Credentials = before.Credentials
			}
			if v.Type == ai.AccountOpenAI && v.APIKey == "" {
				v.APIKey = before.APIKey
			}
		}
		if a.ClearSecret {
			v.APIKey = ""
			v.Credentials = ai.Credentials{}
		}
		if a.LoginDraft != "" {
			if a.AuthorizedCredentials == nil || a.Type != ai.AccountChatGPT || a.ClearSecret {
				return nil, config.InvalidField(field+".login_draft", "authorization", "Sign-in expired; sign in again before saving.")
			}
			v.Credentials = *a.AuthorizedCredentials
		}
		next.Accounts = append(next.Accounts, v)
	}
	boundary := int64(-1)
	for name, f := range in.Features {
		if f.Account < 0 {
			f.Account = accountRefs[f.Account]
			if f.Account == 0 {
				return nil, config.InvalidField("ai_services.features."+name+".account", "reference", "Unknown temporary account.")
			}
		}
		f.Model = ai.FeatureModel(name, f)
		prior := old.Features[name]
		f.AfterOp = prior.AfterOp
		if f.Automatic && !prior.Automatic {
			if boundary < 0 {
				var err error
				boundary, err = store.MaxOpID(l.db)
				if err != nil {
					return nil, err
				}
			}
			f.AfterOp = boundary
		}
		next.Features[name] = f
	}
	// This is one configuration publication, including removal of superseded
	// private fields. It never deletes archive metadata or execution history.
	return map[string]any{"version": config.CurrentVersion, "ai_services": next, "ai": nil, "ai_options": nil}, nil
}
