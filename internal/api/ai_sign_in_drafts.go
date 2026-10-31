package api

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/library"
)

const signInDraftLifetime = 30 * time.Minute

type signInDraft struct {
	number      int64
	credentials ai.Credentials
	pending     bool
	failure     string
	attempt     *openai.Attempt
	expires     time.Time
	timer       *time.Timer
}
type signInDraftView struct {
	ID        string `json:"id"`
	URL       string `json:"url,omitempty"`
	Pending   bool   `json:"pending"`
	Connected bool   `json:"connected"`
	Email     string `json:"email"`
	Error     string `json:"error"`
}
type signInDraftPath struct {
	ID string `path:"id" maxLength:"128"`
}

// The vault contains transient authorization transactions only. Neither tokens
// nor the list of capabilities is readable through the API.
type signInDraftVault struct {
	sync.Mutex
	entries map[string]*signInDraft
}

func (s *Server) removeSignInDraftLocked(id string) {
	if draft := s.aiDrafts.entries[id]; draft != nil {
		delete(s.aiDrafts.entries, id)
		if draft.timer != nil {
			draft.timer.Stop()
		}
		if draft.attempt != nil {
			draft.attempt.Cancel()
		}
		draft.credentials = ai.Credentials{}
	}
}

func (s *Server) signInDraftRoutes() {
	huma.Post(s.contract, Prefix+"/ai/sign-ins", func(ctx context.Context, in *struct {
		Body struct {
			Number   int64                     `json:"number"`
			Settings library.AIServiceSettings `json:"settings"`
		}
	}) (*output[signInDraftView], error) {
		before, client, err := s.library.PrepareAIAccountDraft(ctx, in.Body.Settings, in.Body.Number)
		if errors.Is(err, config.ErrChanged) {
			return nil, huma.Error409Conflict("Settings changed; reload before signing in.")
		}
		if err != nil {
			return nil, settingsFailure(err)
		}
		s.aiDrafts.Lock()
		defer s.aiDrafts.Unlock()
		if len(s.aiDrafts.entries) >= 128 {
			return nil, huma.Error429TooManyRequests("Too many sign-in drafts; cancel unused sign-ins.")
		}
		if s.aiDrafts.entries == nil {
			s.aiDrafts.entries = map[string]*signInDraft{}
		}
		id := uuid.NewString()
		draft := &signInDraft{number: in.Body.Number, pending: true, expires: time.Now().Add(signInDraftLifetime)}
		attempt, err := openai.Begin(before, func(ctx context.Context, next ai.Credentials) error {
			s.aiDrafts.Lock()
			defer s.aiDrafts.Unlock()
			if s.aiDrafts.entries[id] != draft || !time.Now().Before(draft.expires) {
				return errors.New("sign-in draft expired")
			}
			draft.credentials = next
			draft.pending = false
			return nil
		}, client)
		if err != nil {
			return nil, failure(err)
		}
		draft.attempt = attempt
		s.aiDrafts.entries[id] = draft
		draft.timer = time.AfterFunc(signInDraftLifetime, func() {
			s.aiDrafts.Lock()
			defer s.aiDrafts.Unlock()
			if s.aiDrafts.entries[id] == draft {
				s.removeSignInDraftLocked(id)
			}
		})
		go func() {
			err := <-attempt.Done
			client.CloseIdleConnections()
			s.aiDrafts.Lock()
			defer s.aiDrafts.Unlock()
			if s.aiDrafts.entries[id] != draft {
				return
			}
			draft.pending = false
			if err != nil {
				draft.failure = "ChatGPT sign-in failed or expired; sign in again."
			}
		}()
		return response(signInDraftView{ID: id, URL: attempt.URL, Pending: true}, nil)
	})
	huma.Get(s.contract, Prefix+"/ai/sign-ins/{id}", func(ctx context.Context, in *signInDraftPath) (*output[signInDraftView], error) {
		s.aiDrafts.Lock()
		defer s.aiDrafts.Unlock()
		draft := s.aiDrafts.entries[in.ID]
		if draft == nil || !time.Now().Before(draft.expires) {
			return nil, huma.Error404NotFound("Sign-in expired; sign in again.")
		}
		return response(signInDraftView{ID: in.ID, Pending: draft.pending, Connected: draft.credentials.AccessToken != "", Email: draft.credentials.Email, Error: draft.failure}, nil)
	})
	huma.Delete(s.contract, Prefix+"/ai/sign-ins/{id}", func(ctx context.Context, in *signInDraftPath) (*output[struct{}], error) {
		s.aiDrafts.Lock()
		defer s.aiDrafts.Unlock()
		s.removeSignInDraftLocked(in.ID)
		return response(struct{}{}, nil)
	})
}

// authorizeSignInDrafts holds the vault lock through config publication so
// cancel, expiry and save have an unambiguous order. Failed saves retain drafts.
func (s *Server) authorizeSignInDrafts(settings *library.AIServiceSettings) (func(bool), error) {
	s.aiDrafts.Lock()
	ids := map[string]bool{}
	fail := func(i int) (func(bool), error) {
		s.aiDrafts.Unlock()
		return nil, config.InvalidField(fmt.Sprintf("ai_services.accounts.%d.login_draft", i), "authorization", "Sign-in expired or is unfinished; sign in again before saving.")
	}
	if settings != nil {
		for i := range settings.Accounts {
			a := &settings.Accounts[i]
			if a.LoginDraft == "" {
				continue
			}
			draft := s.aiDrafts.entries[a.LoginDraft]
			if draft != nil && draft.number == a.Number && draft.pending && time.Now().Before(draft.expires) {
				s.aiDrafts.Unlock()
				return nil, config.InvalidField(fmt.Sprintf("ai_services.accounts.%d.login_draft", i), "authorization_pending", "Complete sign-in before saving.")
			}
			if draft == nil || draft.number != a.Number || draft.pending || draft.failure != "" ||
				draft.credentials.AccessToken == "" || !time.Now().Before(draft.expires) || ids[a.LoginDraft] || a.Type != ai.AccountChatGPT || a.ClearSecret {
				return fail(i)
			}
			credentials := draft.credentials
			a.AuthorizedCredentials = &credentials
			ids[a.LoginDraft] = true
		}
	}
	return func(committed bool) {
		defer s.aiDrafts.Unlock()
		if committed {
			for id := range ids {
				s.removeSignInDraftLocked(id)
			}
		}
	}, nil
}
