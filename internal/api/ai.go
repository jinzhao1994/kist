package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type AIStatus struct {
	Supported bool   `json:"supported"`
	Connected bool   `json:"connected"`
	Email     string `json:"email"`
	Pending   bool   `json:"pending"`
	Error     string `json:"error"`
}

func (s *Server) aiRoutes() {
	huma.Get(s.contract, Prefix+"/ai", func(ctx context.Context, _ *struct{}) (*output[AIStatus], error) {
		out := AIStatus{Supported: s.library != nil}
		if s.library == nil {
			return response(out, nil)
		}
		c, err := s.library.AIConnection(ctx)
		if err != nil {
			return nil, failure(err)
		}
		out.Connected = c.AccessToken != ""
		out.Email = c.Email
		s.aiMu.Lock()
		out.Pending = s.aiAttempt != nil
		out.Error = s.aiError
		s.aiMu.Unlock()
		return response(out, nil)
	})
	if s.library == nil {
		return
	}
	huma.Post(s.contract, Prefix+"/ai/connect", func(ctx context.Context, _ *struct{ Body struct{} }) (*output[struct {
		URL string `json:"url"`
	}], error) {
		s.aiMu.Lock()
		defer s.aiMu.Unlock()
		if s.aiAttempt != nil {
			return nil, huma.Error409Conflict("ChatGPT sign-in is already pending")
		}
		c, err := s.library.PrepareAIConnection(ctx)
		if err != nil {
			return nil, failure(err)
		}
		attempt, err := openai.Begin(c, func(ctx context.Context, next ai.Credentials) error { return s.library.SaveAIConnection(ctx, c, next) })
		if err != nil {
			return nil, failure(err)
		}
		s.aiAttempt = attempt
		s.aiError = ""
		go func() {
			err := <-attempt.Done
			s.aiMu.Lock()
			defer s.aiMu.Unlock()
			if s.aiAttempt == attempt {
				s.aiAttempt = nil
				if err != nil {
					s.aiError = err.Error()
				}
			}
		}()
		return response(struct {
			URL string `json:"url"`
		}{attempt.URL}, nil)
	})
	huma.Post(s.contract, Prefix+"/ai/disconnect", func(ctx context.Context, _ *struct{ Body struct{} }) (*output[struct {
		Revoked bool `json:"revoked"`
	}], error) {
		s.aiMu.Lock()
		if s.aiAttempt != nil {
			s.aiAttempt.Cancel()
			s.aiAttempt = nil
		}
		s.aiError = ""
		s.aiMu.Unlock()
		confirmed, err := s.library.DisconnectAI(ctx)
		return response(struct {
			Revoked bool `json:"revoked"`
		}{confirmed}, err)
	})
	huma.Get(s.contract, Prefix+"/ai/models", func(ctx context.Context, _ *struct{}) (*output[[]ai.Model], error) {
		p, err := s.library.AIProvider(ctx)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		models, err := p.Models(ctx)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		return response(models, nil)
	})
	type ocrInput struct {
		ID   string `path:"id"`
		Key  string `header:"Idempotency-Key" required:"true"`
		Body struct {
			Model string `json:"model" minLength:"1" maxLength:"200"`
		}
	}
	huma.Post(s.contract, Prefix+"/files/{id}/ocr", func(ctx context.Context, in *ocrInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		select {
		case s.aiSlots <- struct{}{}:
			defer func() { <-s.aiSlots }()
		default:
			return nil, huma.Error429TooManyRequests("Another OCR request is running")
		}
		target := Prefix + "/files/" + id.String() + "/ocr"
		raw, _ := json.Marshal(struct {
			Method, Target string
			Body           any
		}{http.MethodPost, target, in.Body})
		sum := sha256.Sum256(raw)
		receipt, found, err := s.library.CheckRequest(ctx, library.Request{ID: in.Key, Digest: hex.EncodeToString(sum[:])})
		if err != nil {
			return nil, failure(err)
		}
		if found {
			return response(receipt, nil)
		}
		p, err := s.library.AIProvider(ctx)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		models, err := p.Models(ctx)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		available := false
		for _, m := range models {
			if m.ID == in.Body.Model {
				available = true
			}
		}
		if !available {
			return nil, invalid("Model is not available to this account")
		}
		prepared, err := s.library.PrepareOCR(ctx, domain.FileID{ID: id}, p, in.Body.Model)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return s.mutate(ctx, in.Key, "POST", target, in.Body, prepared.Commit)
	})
}
