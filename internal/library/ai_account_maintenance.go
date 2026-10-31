package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
)

// AIAccountRenewal is disposable retry state, not credentials or archive data.
// Version makes failures obsolete after a different process updates credentials.
type AIAccountRenewal struct {
	Version string
	Error   string
	RetryAt time.Time
	Delay   time.Duration
}

func credentialVersion(c ai.Credentials) string {
	raw, _ := json.Marshal(c)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// RenewAIAccounts advances only due credentials. It never probes models or runs
// inference, and shares the rotating-token lock with foreground calls.
func (l *Library) RenewAIAccounts(ctx context.Context, previous map[int64]AIAccountRenewal) map[int64]AIAccountRenewal {
	return l.renewAIAccounts(ctx, previous, configuredAccountHTTP)
}

func (l *Library) renewAIAccounts(ctx context.Context, previous map[int64]AIAccountRenewal, clientFor func(*config.Config, int64, time.Duration) (*http.Client, error)) map[int64]AIAccountRenewal {
	next := map[int64]AIAccountRenewal{}
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return previous
	}
	for _, account := range effectiveAIServices(cfg).Accounts {
		if ctx.Err() != nil {
			return next
		}
		c := account.Credentials
		if account.Type != ai.AccountChatGPT || c.AccessToken == "" {
			continue
		}
		if c.ExpiresAt > time.Now().Add(time.Minute).Unix() {
			continue
		}
		version := credentialVersion(c)
		prior := previous[account.Number]
		if prior.Version == version && (prior.Error == "reauthenticate" || time.Now().Before(prior.RetryAt)) {
			next[account.Number] = prior
			continue
		}
		client, err := clientFor(cfg, account.Number, 30*time.Second)
		if err == nil {
			_, err = l.refreshAIAccount(ctx, account.Number, c, client)
			client.CloseIdleConnections()
		}
		if err == nil || errors.Is(err, config.ErrChanged) || ctx.Err() != nil {
			continue
		}
		delay := time.Minute
		if prior.Version == version && prior.Delay > 0 {
			delay = min(prior.Delay*2, time.Hour)
		}
		state := AIAccountRenewal{Version: version, Error: "temporary", RetryAt: time.Now().Add(delay), Delay: delay}
		var tokenError *openai.TokenError
		if errors.As(err, &tokenError) && tokenError.Reauthenticate {
			state.Error = "reauthenticate"
		}
		next[account.Number] = state
	}
	return next
}
