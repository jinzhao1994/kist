package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/buildinfo"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

type entityInput struct {
	ID string `path:"id"`
}
type boxBody struct {
	Name        string `json:"name" minLength:"1"`
	Description string `json:"description,omitempty"`
	StartAt     string `json:"start_at,omitempty"`
	EndAt       string `json:"end_at,omitempty"`
}
type createBoxInput struct {
	Key  string `header:"Idempotency-Key" required:"true"`
	Body boxBody
}
type updateBoxInput struct {
	ID   string `path:"id"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body boxBody
}

func parseID(raw string) (domain.ID, error) {
	id, err := domain.ParseID(raw)
	if err != nil || id.IsZero() {
		return domain.ID{}, invalid("Invalid entity ID")
	}
	return id, nil
}
func (b boxBody) fields() (library.BoxFields, error) {
	f := library.BoxFields{Name: b.Name, Description: b.Description}
	var err error
	if b.StartAt != "" {
		f.StartAt, err = domain.ParseLocalDate(b.StartAt)
		if err != nil {
			return f, invalid("Invalid start_at")
		}
	}
	if b.EndAt != "" {
		f.EndAt, err = domain.ParseLocalDate(b.EndAt)
		if err != nil {
			return f, invalid("Invalid end_at")
		}
	}
	if !f.StartAt.IsZero() && !f.EndAt.IsZero() && f.StartAt.Start() > f.EndAt.End() {
		return f, invalid("end_at precedes start_at")
	}
	return f, nil
}

type libraryInfo struct {
	Version         string `json:"version"`
	Timezone        string `json:"timezone"`
	ProjectionDirty bool   `json:"projection_dirty"`
}

func (s *Server) readRoutes() {
	huma.Get(s.contract, Prefix+"/info", func(ctx context.Context, _ *struct{}) (*output[libraryInfo], error) {
		return response(libraryInfo{Version: buildinfo.Get().Version, Timezone: s.library.Zone().String(), ProjectionDirty: s.library.ProjectionDirty()}, nil)
	})
	huma.Get(s.contract, Prefix+"/boxes", func(ctx context.Context, _ *struct{}) (*output[wire.BoxList], error) {
		boxes, err := s.library.Boxes(ctx, library.ListBoxesOptions{})
		return response(wire.BoxList{Boxes: wire.NewBoxes(boxes)}, err)
	})
	huma.Get(s.contract, Prefix+"/boxes/{id}", func(ctx context.Context, in *entityInput) (*output[wire.Box], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		box, found, err := s.library.Box(ctx, domain.BoxID{ID: id})
		if err == nil && !found {
			err = library.ErrNoSuchBox
		}
		return response(wire.NewBox(box), err)
	})
	huma.Get(s.contract, Prefix+"/requests/{key}", func(ctx context.Context, in *struct {
		Key string `path:"key"`
	}) (*output[library.RequestResult], error) {
		result, found, err := s.library.RequestStatus(ctx, in.Key)
		if err == nil && !found {
			return nil, huma.Error404NotFound("No committed request with this key")
		}
		return response(result, err)
	})
}

// mutate hashes typed business input, so JSON whitespace and key order do not
// change its identity. Method and target are bound to the same key as the body.
func (s *Server) mutate(ctx context.Context, key, method, target string, body any, fn func(context.Context) error) (*output[library.RequestResult], error) {
	canonical, err := json.Marshal(struct {
		Method, Target string
		Body           any
	}{method, target, body})
	if err != nil {
		return nil, failure(err)
	}
	sum := sha256.Sum256(canonical)
	result, err := s.library.ExecuteRequest(ctx, library.Request{ID: key, Digest: hex.EncodeToString(sum[:])}, fn)
	return response(result, err)
}
func (s *Server) writeRoutes() {
	huma.Post(s.contract, Prefix+"/boxes", func(ctx context.Context, in *createBoxInput) (*output[library.RequestResult], error) {
		fields, err := in.Body.fields()
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "POST", Prefix+"/boxes", in.Body, func(ctx context.Context) error {
			_, err := s.library.CreateBox(ctx, fields.Name, fields.Description, fields.StartAt, fields.EndAt)
			return err
		})
	})
	huma.Put(s.contract, Prefix+"/boxes/{id}", func(ctx context.Context, in *updateBoxInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		fields, err := in.Body.fields()
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "PUT", Prefix+"/boxes/"+id.String(), in.Body, func(ctx context.Context) error {
			_, err := s.library.UpdateBox(ctx, domain.BoxID{ID: id}, fields)
			return err
		})
	})
}
