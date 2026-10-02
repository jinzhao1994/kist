package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

type createTagInput struct {
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		Name     string  `json:"name" minLength:"1"`
		ParentID *string `json:"parent_id,omitempty"`
	}
}
type renameTagInput struct {
	ID   string `path:"id"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		Name string `json:"name" minLength:"1"`
	}
}
type moveTagInput struct {
	ID   string `path:"id"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		ParentID *string `json:"parent_id"`
	}
}
type deleteTagInput struct {
	ID  string `path:"id"`
	Key string `header:"Idempotency-Key" required:"true"`
}
type attachTagInput struct {
	ID   string `path:"id"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		IDs []string `json:"ids" minItems:"1" maxItems:"1000"`
	}
}

func optionalTagID(raw *string) (*domain.TagID, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := parseID(*raw)
	if err != nil {
		return nil, err
	}
	out := domain.TagID{ID: id}
	return &out, nil
}
func (s *Server) tagRoutes() {
	huma.Get(s.contract, Prefix+"/tags", func(ctx context.Context, _ *struct{}) (*output[wire.TagTree], error) {
		tree, counts, err := s.library.TagTree()
		if err != nil {
			return nil, failure(err)
		}
		var render func([]domain.Tag) []wire.Tag
		render = func(tags []domain.Tag) []wire.Tag {
			out := make([]wire.Tag, 0, len(tags))
			for _, tag := range tags {
				item := wire.NewTagCounted(tag, counts[tag.ID])
				item.Children = render(tree.Children(tag.ID))
				out = append(out, item)
			}
			return out
		}
		return response(wire.TagTree{Tags: render(tree.Roots())}, nil)
	})
	huma.Post(s.contract, Prefix+"/tags", func(ctx context.Context, in *createTagInput) (*output[library.RequestResult], error) {
		parent, err := optionalTagID(in.Body.ParentID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "POST", Prefix+"/tags", in.Body, func(ctx context.Context) error { _, err := s.library.CreateTag(ctx, in.Body.Name, parent); return err })
	})
	huma.Put(s.contract, Prefix+"/tags/{id}/name", func(ctx context.Context, in *renameTagInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "PUT", Prefix+"/tags/"+id.String()+"/name", in.Body, func(ctx context.Context) error { return s.library.RenameTag(ctx, domain.TagID{ID: id}, in.Body.Name) })
	})
	huma.Put(s.contract, Prefix+"/tags/{id}/parent", func(ctx context.Context, in *moveTagInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		parent, err := optionalTagID(in.Body.ParentID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "PUT", Prefix+"/tags/"+id.String()+"/parent", in.Body, func(ctx context.Context) error { return s.library.MoveTag(ctx, domain.TagID{ID: id}, parent) })
	})
	huma.Delete(s.contract, Prefix+"/tags/{id}", func(ctx context.Context, in *deleteTagInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "DELETE", Prefix+"/tags/"+id.String(), nil, func(ctx context.Context) error { return s.library.DeleteTag(ctx, domain.TagID{ID: id}) })
	})
	for _, kind := range []string{"files", "boxes"} {
		for _, detach := range []bool{false, true} {
			method := "PUT"
			if detach {
				method = "DELETE"
			}
			route := Prefix + "/tags/{id}/" + kind
			huma.Register(s.contract, huma.Operation{OperationID: method + "-tag-" + kind, Method: method, Path: route}, func(ctx context.Context, in *attachTagInput) (*output[library.RequestResult], error) {
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
				return s.mutate(ctx, in.Key, method, Prefix+"/tags/"+id.String()+"/"+kind, in.Body, func(ctx context.Context) error {
					var err error
					if kind == "files" {
						if detach {
							_, err = s.library.UntagFiles(ctx, tag, files)
						} else {
							_, err = s.library.TagFiles(ctx, tag, files)
						}
					} else {
						if detach {
							_, err = s.library.UntagBoxes(ctx, tag, boxes)
						} else {
							_, err = s.library.TagBoxes(ctx, tag, boxes)
						}
					}
					return err
				})
			})
		}
	}
}
