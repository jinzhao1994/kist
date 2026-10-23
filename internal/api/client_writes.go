package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func (s *Server) clientMutate(ctx context.Context, key, method, target string, body any, fn func(context.Context) error) (*output[library.ClientRequestResult], error) {
	canonical, err := json.Marshal(struct {
		Method, Target string
		Body           any
	}{method, target, body})
	if err != nil {
		return nil, failure(err)
	}
	digest := sha256.Sum256(canonical)
	result, err := s.client.ExecuteRequest(ctx, library.Request{ID: key, Digest: hex.EncodeToString(digest[:])}, fn)
	if err == nil {
		s.wakeAutoSync()
	}
	return response(result, err)
}

func (s *Server) clientWriteRoutes() {
	huma.Get(s.contract, Prefix+"/requests/{key}", func(ctx context.Context, in *struct {
		Key string `path:"key"`
	}) (*output[library.ClientRequestResult], error) {
		result, found, err := s.client.RequestStatus(ctx, in.Key)
		if err == nil && !found {
			return nil, huma.Error404NotFound("No committed request with this key")
		}
		return response(result, err)
	})
	huma.Post(s.contract, Prefix+"/boxes", func(ctx context.Context, in *createBoxInput) (*output[library.ClientRequestResult], error) {
		fields, err := in.Body.fields()
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "POST", Prefix+"/boxes", in.Body, func(ctx context.Context) error {
			_, err := s.client.CreateBox(ctx, fields.Name, fields.Description, fields.StartAt, fields.EndAt)
			return err
		})
	})
	huma.Put(s.contract, Prefix+"/boxes/{id}", func(ctx context.Context, in *updateBoxInput) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		fields, err := in.Body.fields()
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "PUT", Prefix+"/boxes/"+id.String(), in.Body, func(ctx context.Context) error {
			_, err := s.client.UpdateBox(ctx, domain.BoxID{ID: id}, fields)
			return err
		})
	})
	huma.Put(s.contract, Prefix+"/filesets/{id}", func(ctx context.Context, in *filesetWriteInput) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		box, err := parseID(in.Body.Box)
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "PUT", Prefix+"/filesets/"+id.String(), in.Body, func(ctx context.Context) error {
			_, err := s.client.SetFileset(ctx, domain.BoxID{ID: box}, domain.FolderID{ID: id}, in.Body.Before, in.Body.After)
			return err
		})
	})
	s.clientTagWriteRoutes()
	s.clientEntryRoutes()
	s.clientSavedSearchWriteRoutes()
	s.clientBoxWriteRoutes()
}
