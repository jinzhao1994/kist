package library

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

type aiConfigurationContextKey struct{}

func (q *OCRQueue) requireConfiguredFeature(ctx context.Context) error {
	if q.kind == "metadata" || q.kind == "geo" {
		return nil
	}
	cfg, err := q.library.aiConfiguration(ctx)
	if err != nil {
		return err
	}
	if cfg.AIServices == nil {
		if q.kind == "transcription" {
			return errors.New("configure an OpenAI API account for transcription in Settings")
		}
		if cfg.AIOptions.Model == "" {
			return errors.New("choose an AI model in Settings first")
		}
		return nil
	}
	f := cfg.AIServices.Features[q.kind]
	if f.Account == 0 || f.Model == "" {
		return errors.New("choose an AI account and model for this feature in Settings")
	}
	return nil
}

func (l *Library) aiConfiguration(ctx context.Context) (*config.Config, error) {
	if cfg, ok := ctx.Value(aiConfigurationContextKey{}).(*config.Config); ok {
		return cfg, nil
	}
	return config.Load(l.layout.ConfigPath())
}

// AIFeatureProvider binds authentication, transport and parameters to one task's
// configuration snapshot. It never infers a route from list position.
func (l *Library) AIFeatureProvider(ctx context.Context, kind string) (ai.Provider, error) {
	cfg, err := l.aiConfiguration(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.AIServices == nil {
		return l.AIProvider(ctx)
	}
	feature, ok := cfg.AIServices.Features[kind]
	if !ok || feature.Account == 0 || feature.Model == "" {
		return nil, errors.New("choose an AI account and model for this feature in Settings")
	}
	return l.aiAccountProvider(ctx, cfg, feature.Account, &ai.ModelParameters{ReasoningEffort: feature.ReasoningEffort, ImageDetail: feature.ImageDetail})
}

func accountHTTP(services ai.Services, number int64, timeout time.Duration) (*http.Client, error) {
	a, ok := services.Account(number)
	if !ok {
		return nil, errors.New("AI account no longer exists")
	}
	if a.Proxy == 0 {
		return ai.ProxyHTTPClient(nil, timeout)
	}
	p, ok := services.Proxy(a.Proxy)
	if !ok {
		return nil, errors.New("AI account proxy no longer exists")
	}
	return ai.ProxyHTTPClient(&p, timeout)
}

func configuredAccountHTTP(cfg *config.Config, number int64, timeout time.Duration) (*http.Client, error) {
	services := effectiveAIServices(cfg)
	if _, ok := services.Account(number); !ok {
		return nil, errors.New("AI account no longer exists")
	}
	if cfg.AIServices == nil {
		// An explicit migration changes empty proxy selection to direct. Merely
		// reading the new settings must preserve the legacy environment policy.
		return ai.HTTPClient(cfg.AIOptions.ProxyURL, timeout)
	}
	return accountHTTP(services, number, timeout)
}

func (l *Library) AIAccountHTTP(ctx context.Context, number int64, timeout time.Duration) (*http.Client, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return configuredAccountHTTP(cfg, number, timeout)
}

func (l *Library) AIAccountProvider(ctx context.Context, number int64) (ai.Provider, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return nil, err
	}
	return l.aiAccountProvider(ctx, cfg, number, &ai.ModelParameters{})
}

func (l *Library) aiAccountProvider(ctx context.Context, cfg *config.Config, number int64, parameters *ai.ModelParameters) (ai.Provider, error) {
	services := effectiveAIServices(cfg)
	a, ok := services.Account(number)
	if !ok {
		return nil, errors.New("AI account no longer exists")
	}
	client, err := configuredAccountHTTP(cfg, number, 3*time.Minute)
	if err != nil {
		return nil, err
	}
	if a.Type == ai.AccountOpenAI {
		if a.APIKey == "" {
			return nil, errors.New("configure this account's OpenAI API key first")
		}
		return openai.Client{Token: a.APIKey, APIKey: true, Parameters: parameters, HTTP: client}, nil
	}
	if a.Type != ai.AccountChatGPT || a.Credentials.Provider != "openai" || a.Credentials.AccessToken == "" {
		return nil, errors.New("connect this ChatGPT account first")
	}
	auth, err := configuredAccountHTTP(cfg, number, 30*time.Second)
	if err != nil {
		return nil, err
	}
	next, err := l.refreshAIAccount(ctx, number, a.Credentials, auth)
	if err != nil {
		return nil, err
	}
	return openai.Client{Token: next.AccessToken, HTTP: client, Parameters: parameters}, nil
}

// refreshAIAccount serializes rotating-token redemption across processes. The
// config lock only protects the final CAS; remote IO never holds that lock.
func (l *Library) refreshAIAccount(ctx context.Context, number int64, selected ai.Credentials, client *http.Client) (ai.Credentials, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	lock := lockfile.New(filepath.Join(l.layout.Root, fmt.Sprintf("ai-account-%d-refresh.lock", number)))
	for {
		if err := ctx.Err(); err != nil {
			return ai.Credentials{}, err
		}
		release, acquired, err := lock.TryAcquire()
		if err != nil {
			return ai.Credentials{}, err
		}
		if acquired {
			defer release()
			break
		}
		select {
		case <-ctx.Done():
			return ai.Credentials{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	current, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return ai.Credentials{}, err
	}
	a, ok := effectiveAIServices(current).Account(number)
	before := a.Credentials
	if !ok || a.Type != ai.AccountChatGPT || before.AccessToken == "" ||
		before.HostID != selected.HostID || before.Provider != selected.Provider ||
		before.ClientID != selected.ClientID || before.Subject != selected.Subject {
		return ai.Credentials{}, config.ErrChanged
	}
	next, err := openai.Refresh(ctx, before, client)
	if err != nil {
		return ai.Credentials{}, err
	}
	if next != before {
		if err := l.SaveAIAccountConnection(ctx, number, before, next); err != nil {
			return ai.Credentials{}, err
		}
	}
	return next, nil
}

// updateAIAccount rereads the complete config under its OS lock, changing just
// one account. Concurrent token refreshes never overwrite another account.
func (l *Library) updateAIAccount(ctx context.Context, number int64, change func(*ai.Account) error) error {
	release, err := lockfile.New(filepath.Join(l.layout.Root, "config.lock")).Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return err
	}
	services := effectiveAIServices(cfg)
	for i := range services.Accounts {
		if services.Accounts[i].Number != number {
			continue
		}
		before := services.Accounts[i]
		if err := change(&services.Accounts[i]); err != nil {
			return err
		}
		if before == services.Accounts[i] {
			return ctx.Err()
		}
		if cfg.AIServices == nil {
			// Keep legacy UI usable until the explicit settings migration.
			c := services.Accounts[i].Credentials
			return config.Update(l.layout.ConfigPath(), "", map[string]any{"ai": c})
		}
		return config.Update(l.layout.ConfigPath(), "", map[string]any{"ai_services": services})
	}
	return errors.New("AI account no longer exists")
}

func (l *Library) PrepareAIAccountConnection(ctx context.Context, number int64) (ai.Credentials, error) {
	var out ai.Credentials
	err := l.updateAIAccount(ctx, number, func(a *ai.Account) error {
		if a.Type != ai.AccountChatGPT {
			return errors.New("this account does not use ChatGPT sign-in")
		}
		if a.Credentials.HostID == "" {
			a.Credentials.HostID = "urn:uuid:" + uuid.NewString()
		}
		out = a.Credentials
		return nil
	})
	return out, err
}

func (l *Library) SaveAIAccountConnection(ctx context.Context, number int64, before, next ai.Credentials) error {
	return l.updateAIAccount(ctx, number, func(a *ai.Account) error {
		if a.Type != ai.AccountChatGPT || a.Credentials != before {
			return config.ErrChanged
		}
		a.Credentials = next
		return nil
	})
}

func (l *Library) DisconnectAIAccount(ctx context.Context, number int64) (bool, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return false, err
	}
	services := effectiveAIServices(cfg)
	a, ok := services.Account(number)
	if !ok {
		return false, errors.New("AI account no longer exists")
	}
	confirmed := true
	if a.Type == ai.AccountChatGPT {
		client, err := configuredAccountHTTP(cfg, number, 30*time.Second)
		if err != nil {
			return false, err
		}
		confirmed = openai.Revoke(ctx, a.Credentials, client) == nil
	}
	err = l.updateAIAccount(ctx, number, func(current *ai.Account) error {
		if *current != a {
			return config.ErrChanged
		}
		current.APIKey = ""
		current.Credentials.AccessToken = ""
		current.Credentials.RefreshToken = ""
		current.Credentials.IDToken = ""
		current.Credentials.Scope = ""
		current.Credentials.ExpiresAt = 0
		return nil
	})
	return confirmed, err
}
