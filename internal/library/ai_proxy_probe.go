package library

import (
	"context"
	"errors"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/config"
)

// ErrNoSuchAIProxy identifies a saved proxy removed before a diagnostic request.
var ErrNoSuchAIProxy = errors.New("AI proxy no longer exists")

// TestAIProxy reads saved configuration without updating it or contacting AI.
func (l *Library) TestAIProxy(ctx context.Context, number int64) (ai.ProxyTestResult, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return ai.ProxyTestResult{}, err
	}
	proxy, ok := effectiveAIServices(cfg).Proxy(number)
	if !ok {
		return ai.ProxyTestResult{}, ErrNoSuchAIProxy
	}
	return ai.TestProxy(ctx, proxy), nil
}
