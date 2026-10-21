package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func (s *Server) clientTagWriteRoutes() {
	huma.Post(s.contract, Prefix+"/tags", func(ctx context.Context, in *createTagInput) (*output[library.ClientRequestResult], error) {
		parent, err := optionalTagID(in.Body.ParentID)
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "POST", Prefix+"/tags", in.Body, func(ctx context.Context) error { _, err := s.client.CreateTag(ctx, in.Body.Name, parent); return err })
	})
	huma.Put(s.contract, Prefix+"/tags/{id}/name", func(ctx context.Context, in *renameTagInput) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "PUT", Prefix+"/tags/"+id.String()+"/name", in.Body, func(ctx context.Context) error { return s.client.RenameTag(ctx, domain.TagID{ID: id}, in.Body.Name) })
	})
	huma.Put(s.contract, Prefix+"/tags/{id}/parent", func(ctx context.Context, in *moveTagInput) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		parent, err := optionalTagID(in.Body.ParentID)
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "PUT", Prefix+"/tags/"+id.String()+"/parent", in.Body, func(ctx context.Context) error { return s.client.MoveTag(ctx, domain.TagID{ID: id}, parent) })
	})
	huma.Delete(s.contract, Prefix+"/tags/{id}", func(ctx context.Context, in *deleteTagInput) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "DELETE", Prefix+"/tags/"+id.String(), nil, func(ctx context.Context) error { return s.client.DeleteTag(ctx, domain.TagID{ID: id}) })
	})
	for _, kind := range []string{"files", "boxes"} {
		for _, detach := range []bool{false, true} {
			method := "PUT"
			if detach {
				method = "DELETE"
			}
			route := Prefix + "/tags/{id}/" + kind
			huma.Register(s.contract, huma.Operation{OperationID: method + "-tag-" + kind, Method: method, Path: route}, func(ctx context.Context, in *attachTagInput) (*output[library.ClientRequestResult], error) {
				id, err := parseID(in.ID)
				if err != nil {
					return nil, err
				}
				tag := domain.TagID{ID: id}
				var files []domain.FileID
				var boxes []domain.BoxID
				for _, raw := range in.Body.IDs {
					entity, err := parseID(raw)
					if err != nil {
						return nil, err
					}
					if kind == "files" {
						files = append(files, domain.FileID{ID: entity})
					} else {
						boxes = append(boxes, domain.BoxID{ID: entity})
					}
				}
				return s.clientMutate(ctx, in.Key, method, Prefix+"/tags/"+id.String()+"/"+kind, in.Body, func(ctx context.Context) error {
					var err error
					if kind == "files" {
						if detach {
							_, err = s.client.UntagFiles(ctx, tag, files)
						} else {
							_, err = s.client.TagFiles(ctx, tag, files)
						}
					} else {
						if detach {
							_, err = s.client.UntagBoxes(ctx, tag, boxes)
						} else {
							_, err = s.client.TagBoxes(ctx, tag, boxes)
						}
					}
					return err
				})
			})
		}
	}
}
