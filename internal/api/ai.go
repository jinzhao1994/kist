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
	huma.Put(s.contract, Prefix+"/files/{id}/image-rotation", func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Key  string `header:"Idempotency-Key" required:"true"`
		Body struct {
			BlobKey string `json:"blob_key"`
			Degrees *int   `json:"clockwise_degrees" nullable:"true"`
		}
	}) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "PUT", Prefix+"/files/"+id.String()+"/image-rotation", in.Body, func(ctx context.Context) error {
			return s.library.SetImageRotation(ctx, domain.FileID{ID: id}, in.Body.BlobKey, in.Body.Degrees)
		})
	})
	s.queueRoutes("/ai/", map[string]*library.OCRQueue{"orientation": s.library.OrientationQueue(), "ocr": s.library.OCRQueue(), "desc": s.library.DescriptionQueue(), "facts": s.library.FactsQueue()})
	s.locationRoutes()
}

// RunOCR joins the server lifetime; requests only update the persistent queue.
func (s *Server) RunOCR(ctx context.Context) {
	if s.library != nil {
		s.library.OrientationQueue().Run(ctx, s.library.OCRQueue(), s.library.DescriptionQueue(), s.library.FactsQueue(), s.library.LocationQueue())
	}
}

func (s *Server) queueRoutes(prefix string, queues map[string]*library.OCRQueue) {
	for kind, q := range queues {
		huma.Get(s.contract, Prefix+prefix+kind+"/jobs", func(ctx context.Context, in *struct {
			Cursor string `query:"cursor" maxLength:"1024"`
			Limit  int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
		}) (*output[library.ExtractionQueuePage], error) {
			v, err := q.Jobs(ctx, in.Cursor, in.Limit)
			return response(v, err)
		})
		huma.Get(s.contract, Prefix+prefix+kind, func(ctx context.Context, _ *struct{}) (*output[library.OCRQueueStatus], error) {
			v, err := q.Status(ctx)
			return response(v, err)
		})
		huma.Post(s.contract, Prefix+prefix+kind+"/backfill", func(ctx context.Context, in *struct {
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
		huma.Post(s.contract, Prefix+prefix+kind+"/pause", func(ctx context.Context, in *struct {
			Body struct {
				Paused bool `json:"paused"`
			}
		}) (*output[library.OCRQueueStatus], error) {
			v, err := q.Pause(ctx, in.Body.Paused)
			return response(v, err)
		})
		huma.Get(s.contract, Prefix+"/files/{id}/"+kind, func(ctx context.Context, in *struct {
			ID string `path:"id"`
		}) (*output[library.OCRJob], error) {
			id, err := parseID(in.ID)
			if err != nil {
				return nil, err
			}
			v, err := q.FileStatus(ctx, domain.FileID{ID: id})
			return response(v, err)
		})
		huma.Get(s.contract, Prefix+"/files/{id}/"+kind+"/attempts", func(ctx context.Context, in *struct {
			ID     string `path:"id"`
			Before int64  `query:"before" minimum:"0"`
		}) (*output[[]library.ExtractionAttempt], error) {
			id, err := parseID(in.ID)
			if err != nil {
				return nil, err
			}
			v, err := q.Attempts(ctx, domain.FileID{ID: id}, in.Before)
			return response(v, err)
		})
		huma.Get(s.contract, Prefix+"/files/{id}/"+kind+"/attempts/{request_id}", func(ctx context.Context, in *struct {
			ID        string `path:"id"`
			RequestID string `path:"request_id"`
		}) (*output[library.ExtractionAttemptDetail], error) {
			id, err := parseID(in.ID)
			if err != nil {
				return nil, err
			}
			v, err := q.Attempt(ctx, domain.FileID{ID: id}, in.RequestID)
			return response(v, err)
		})
		huma.Post(s.contract, Prefix+"/files/{id}/"+kind, func(ctx context.Context, in *struct {
			ID   string `path:"id"`
			Body struct {
				Retry     bool   `json:"retry"`
				Force     bool   `json:"force,omitempty"`
				RequestID string `json:"request_id,omitempty"`
			}
		}) (*output[library.OCRJob], error) {
			id, err := parseID(in.ID)
			if err != nil {
				return nil, err
			}
			var v library.OCRJob
			if in.Body.Force {
				v, err = q.Reextract(ctx, domain.FileID{ID: id}, in.Body.RequestID)
			} else {
				v, err = q.Request(ctx, domain.FileID{ID: id}, in.Body.Retry)
			}
			if err != nil {
				return nil, huma.Error400BadRequest(err.Error())
			}
			return response(v, nil)
		})
	}
}
