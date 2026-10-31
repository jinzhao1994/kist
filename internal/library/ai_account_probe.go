package library

import (
	"context"
	"errors"
	"time"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
)

// ErrNoSuchAIAccount identifies a removed diagnostic target.
var ErrNoSuchAIAccount = errors.New("AI account no longer exists")

// AIAccountTestResult describes one ephemeral model-catalog request.
type AIAccountTestResult struct {
	Models    []string `json:"models"`
	ElapsedMS int64    `json:"elapsed_ms"`
	Error     string   `json:"error" enum:",not_configured,authentication,permission,rate_limited,timeout,connection,service"`
}

// TestAIAccount follows the saved account route without performing inference.
// ChatGPT credentials can refresh through the existing serialized CAS path.
func (l *Library) TestAIAccount(ctx context.Context, number int64) (AIAccountTestResult, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return AIAccountTestResult{}, err
	}
	account, ok := effectiveAIServices(cfg).Account(number)
	if !ok {
		return AIAccountTestResult{}, ErrNoSuchAIAccount
	}
	if account.Type == ai.AccountOpenAI && account.APIKey == "" || account.Type == ai.AccountChatGPT && account.Credentials.AccessToken == "" {
		return AIAccountTestResult{Error: "not_configured"}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	started := time.Now()
	provider, err := l.aiAccountProvider(ctx, cfg, number, &ai.ModelParameters{})
	if err != nil {
		return accountTestFailure(ctx, err), nil
	}
	if client, ok := provider.(openai.Client); ok && client.HTTP != nil {
		defer client.HTTP.CloseIdleConnections()
	}
	models, err := provider.Models(ctx)
	if err != nil {
		return accountTestFailure(ctx, err), nil
	}
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return AIAccountTestResult{Models: ids, ElapsedMS: time.Since(started).Milliseconds()}, nil
}

func accountTestFailure(ctx context.Context, err error) AIAccountTestResult {
	reason := "service"
	var status *openai.HTTPStatusError
	var token *openai.TokenError
	switch {
	case ctx.Err() != nil:
		reason = "timeout"
	case errors.As(err, &token) && token.Reauthenticate:
		reason = "authentication"
	case errors.As(err, &status):
		switch status.StatusCode {
		case 401:
			reason = "authentication"
		case 403:
			reason = "permission"
		case 429:
			reason = "rate_limited"
		}
	case errors.Is(err, openai.ErrConnection):
		reason = "connection"
	}
	return AIAccountTestResult{Error: reason}
}
