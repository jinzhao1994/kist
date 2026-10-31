package api

import (
	"context"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/library"
)

type aiAccountPath struct {
	Number int64 `path:"number" minimum:"1"`
}
type aiAccountAction struct {
	Number int64 `path:"number" minimum:"1"`
	Body   struct{}
}

type aiServicesView struct {
	library.AIServiceSettings
	Definitions    []ai.FeatureDefinition `json:"definitions"`
	PendingAccount int64                  `json:"pending_account"`
	ErrorAccount   int64                  `json:"error_account"`
	Error          string                 `json:"error"`
}

// prepareAISignInChange returns cleanup to run only after configuration has
// committed. Keep a newer sign-in safe if the pending attempt changes meanwhile.
func (s *Server) prepareAISignInChange(next *library.AIServiceSettings) func() {
	noop := func() {}
	if next == nil {
		return noop
	}
	s.aiMu.Lock()
	attempt, number := s.aiAttempt, s.aiAttemptAccount
	s.aiMu.Unlock()
	if attempt == nil {
		return noop
	}
	for _, account := range next.Accounts {
		if number > 0 && account.Number == number && account.Type == ai.AccountChatGPT && !account.ClearSecret {
			return noop
		}
	}
	return func() {
		s.aiMu.Lock()
		defer s.aiMu.Unlock()
		if s.aiAttempt != attempt {
			return
		}
		attempt.Cancel()
		s.aiAttempt, s.aiAttemptAccount = nil, 0
		s.aiError, s.aiErrorAccount = "", 0
	}
}

func (s *Server) aiServiceRoutes() {
	s.signInDraftRoutes()
	huma.Post(s.contract, Prefix+"/ai/accounts/{number}/test", func(ctx context.Context, in *aiAccountAction) (*output[library.AIAccountTestResult], error) {
		result, err := s.library.TestAIAccount(ctx, in.Number)
		if errors.Is(err, library.ErrNoSuchAIAccount) {
			return nil, huma.Error404NotFound("AI account no longer exists")
		}
		return response(result, err)
	})
	huma.Post(s.contract, Prefix+"/ai/proxies/{number}/test", func(ctx context.Context, in *aiAccountAction) (*output[ai.ProxyTestResult], error) {
		result, err := s.library.TestAIProxy(ctx, in.Number)
		if errors.Is(err, library.ErrNoSuchAIProxy) {
			return nil, huma.Error404NotFound("AI proxy no longer exists")
		}
		return response(result, err)
	})
	huma.Get(s.contract, Prefix+"/ai/services", func(ctx context.Context, _ *struct{}) (*output[aiServicesView], error) {
		settings, err := s.library.AIServicesSettings(ctx)
		if err != nil {
			return nil, failure(err)
		}
		out := aiServicesView{AIServiceSettings: settings, Definitions: ai.FeatureDefinitions()}
		s.aiMu.Lock()
		for i := range out.Accounts {
			a := &out.Accounts[i]
			if status := s.aiRenewals[a.Number]; status.Version == a.CredentialVersion {
				a.AuthenticationError = status.Error
			}
		}
		if s.aiAttempt != nil {
			out.PendingAccount = s.aiAttemptAccount
		}
		out.ErrorAccount, out.Error = s.aiErrorAccount, s.aiError
		s.aiMu.Unlock()
		return response(out, nil)
	})
	huma.Get(s.contract, Prefix+"/ai/accounts/{number}/models", func(ctx context.Context, in *aiAccountPath) (*output[[]ai.Model], error) {
		provider, err := s.library.AIAccountProvider(ctx, in.Number)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		models, err := provider.Models(ctx)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		return response(models, nil)
	})
	huma.Post(s.contract, Prefix+"/ai/accounts/{number}/connect", func(ctx context.Context, in *aiAccountAction) (*output[struct {
		URL string `json:"url"`
	}], error) {
		s.aiMu.Lock()
		defer s.aiMu.Unlock()
		if s.aiAttempt != nil {
			return nil, huma.Error409Conflict("ChatGPT sign-in is already pending")
		}
		before, err := s.library.PrepareAIAccountConnection(ctx, in.Number)
		if err != nil {
			return nil, failure(err)
		}
		client, err := s.library.AIAccountHTTP(ctx, in.Number, 30*time.Second)
		if err != nil {
			return nil, failure(err)
		}
		attempt, err := openai.Begin(before, func(ctx context.Context, next ai.Credentials) error {
			return s.library.SaveAIAccountConnection(ctx, in.Number, before, next)
		}, client)
		if err != nil {
			return nil, failure(err)
		}
		s.aiAttempt, s.aiAttemptAccount, s.aiError, s.aiErrorAccount = attempt, in.Number, "", 0
		go func() {
			err := <-attempt.Done
			s.aiMu.Lock()
			defer s.aiMu.Unlock()
			if s.aiAttempt != attempt {
				return
			}
			s.aiAttempt = nil
			s.aiAttemptAccount = 0
			if err != nil {
				s.aiError, s.aiErrorAccount = err.Error(), in.Number
			}
		}()
		return response(struct {
			URL string `json:"url"`
		}{attempt.URL}, nil)
	})
	huma.Post(s.contract, Prefix+"/ai/accounts/{number}/disconnect", func(ctx context.Context, in *aiAccountAction) (*output[struct {
		Revoked bool `json:"revoked"`
	}], error) {
		s.aiMu.Lock()
		if s.aiAttempt != nil && s.aiAttemptAccount == in.Number {
			s.aiAttempt.Cancel()
			s.aiAttempt = nil
			s.aiAttemptAccount = 0
		}
		if s.aiErrorAccount == in.Number {
			s.aiError = ""
			s.aiErrorAccount = 0
		}
		s.aiMu.Unlock()
		revoked, err := s.library.DisconnectAIAccount(ctx, in.Number)
		return response(struct {
			Revoked bool `json:"revoked"`
		}{revoked}, err)
	})
}
