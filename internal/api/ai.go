package api

import (
	"context"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
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
		client, err := s.library.AIHTTP(30 * time.Second)
		if err != nil {
			return nil, failure(err)
		}
		attempt, err := openai.Begin(c, func(ctx context.Context, next ai.Credentials) error { return s.library.SaveAIConnection(ctx, c, next) }, client)
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

	q := s.library.OCRQueue()
	huma.Get(s.contract, Prefix+"/ai/settings", func(ctx context.Context, _ *struct{}) (*output[library.OCRSettings], error) {
		v, err := s.library.OCRSettings(ctx)
		return response(v, err)
	})
	huma.Put(s.contract, Prefix+"/ai/settings", func(ctx context.Context, in *struct{ Body library.OCRSettings }) (*output[library.OCRSettings], error) {
		v, err := s.library.SaveOCRSettings(ctx, in.Body)
		if errors.Is(err, config.ErrChanged) {
			return nil, huma.Error409Conflict("AI settings changed; reload before saving")
		}
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return response(v, nil)
	})
	huma.Get(s.contract, Prefix+"/ai/ocr", func(ctx context.Context, _ *struct{}) (*output[library.OCRQueueStatus], error) {
		v, err := q.Status(ctx)
		return response(v, err)
	})
	huma.Post(s.contract, Prefix+"/ai/ocr/backfill", func(ctx context.Context, in *struct {
		Body struct {
			Retry bool `json:"retry"`
		}
	}) (*output[library.OCRQueueStatus], error) {
		v, err := q.Backfill(ctx, in.Body.Retry)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return response(v, nil)
	})
	huma.Post(s.contract, Prefix+"/ai/ocr/pause", func(ctx context.Context, in *struct {
		Body struct {
			Paused bool `json:"paused"`
		}
	}) (*output[library.OCRQueueStatus], error) {
		v, err := q.Pause(ctx, in.Body.Paused)
		return response(v, err)
	})
	huma.Get(s.contract, Prefix+"/files/{id}/ocr", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*output[library.OCRJob], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		v, err := q.FileStatus(ctx, domain.FileID{ID: id})
		return response(v, err)
	})
	huma.Post(s.contract, Prefix+"/files/{id}/ocr", func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Body struct {
			Retry bool `json:"retry"`
		}
	}) (*output[library.OCRJob], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		v, err := q.Request(ctx, domain.FileID{ID: id}, in.Body.Retry)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return response(v, nil)
	})
}

// RunOCR joins the server lifetime; requests only update the persistent queue.
func (s *Server) RunOCR(ctx context.Context) {
	if s.library != nil {
		s.library.OCRQueue().Run(ctx)
	}
}
