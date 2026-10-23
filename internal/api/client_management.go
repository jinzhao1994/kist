package api

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type clientCacheSettings struct {
	Body struct {
		MaxAge   string `json:"max_age" minLength:"1" maxLength:"64"`
		MaxBytes int64  `json:"max_bytes" minimum:"1"`
	}
}
type clientOfflineSelection struct {
	Files    []string `json:"files"`
	Filesets []string `json:"filesets"`
}
type clientOfflineInput struct {
	Owner string `path:"owner" minLength:"1" maxLength:"4096"`
}
type clientOfflineUpdate struct {
	Owner string `path:"owner" minLength:"1" maxLength:"4096"`
	Body  clientOfflineSelection
}

// Client settings and retention are local runtime state, never NAS archive ops.
func (s *Server) clientManagementRoutes() {
	s.clientConnectionStatusRoute()
	huma.Get(s.contract, Prefix+"/client/status", func(ctx context.Context, _ *struct{}) (*output[library.ClientStatus], error) {
		status, err := s.client.Status(ctx)
		return response(status, err)
	})
	huma.Get(s.contract, Prefix+"/client/cache", func(ctx context.Context, _ *struct{}) (*output[library.ClientCacheStatus], error) {
		status, err := s.client.CacheStatus(ctx)
		return response(status, err)
	})
	huma.Put(s.contract, Prefix+"/client/cache", func(ctx context.Context, in *clientCacheSettings) (*output[library.ClientCacheStatus], error) {
		age, err := time.ParseDuration(in.Body.MaxAge)
		if err != nil || age <= 0 {
			return nil, invalid("Invalid cache retention time")
		}
		status, err := s.client.SetCacheLimits(ctx, age, in.Body.MaxBytes)
		return response(status, err)
	})
	huma.Post(s.contract, Prefix+"/client/cache/prune", func(ctx context.Context, _ *struct{ Body struct{} }) (*output[library.ClientCacheStatus], error) {
		status, err := s.client.PruneCache(ctx)
		return response(status, err)
	})
	huma.Get(s.contract, Prefix+"/client/offline", func(ctx context.Context, _ *struct{}) (*output[map[string]clientOfflineSelection], error) {
		selections, err := s.client.OfflineSelections(ctx)
		result := map[string]clientOfflineSelection{}
		for owner, selection := range selections {
			item := clientOfflineSelection{Files: []string{}, Filesets: []string{}}
			for _, id := range selection.Files {
				item.Files = append(item.Files, id.String())
			}
			for _, id := range selection.Filesets {
				item.Filesets = append(item.Filesets, id.String())
			}
			result[owner] = item
		}
		return response(result, err)
	})
	huma.Put(s.contract, Prefix+"/client/offline/{owner}", func(ctx context.Context, in *clientOfflineUpdate) (*output[struct{}], error) {
		selection := library.ClientOfflineSelection{}
		for _, ref := range in.Body.Files {
			id, err := parseID(ref)
			if err != nil {
				return nil, err
			}
			selection.Files = append(selection.Files, domain.FileID{ID: id})
		}
		for _, ref := range in.Body.Filesets {
			id, err := parseID(ref)
			if err != nil {
				return nil, err
			}
			selection.Filesets = append(selection.Filesets, domain.FolderID{ID: id})
		}
		return response(struct{}{}, s.client.SetOfflineSelection(ctx, in.Owner, selection))
	})
	huma.Delete(s.contract, Prefix+"/client/offline/{owner}", func(ctx context.Context, in *clientOfflineInput) (*output[struct{}], error) {
		return response(struct{}{}, s.client.SetOfflineSelection(ctx, in.Owner, library.ClientOfflineSelection{}))
	})
	huma.Get(s.contract, Prefix+"/client/offline/{owner}", func(ctx context.Context, in *clientOfflineInput) (*output[library.ClientOfflineStatus], error) {
		status, err := s.client.OfflineStatus(ctx, in.Owner)
		return response(status, err)
	})
}
