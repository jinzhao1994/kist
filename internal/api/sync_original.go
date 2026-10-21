package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/library"
	staging "github.com/jinzhao1994/kist/internal/upload"
)

type syncOriginalInput struct {
	Key string `path:"key"`
}

type syncOriginalUpload struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Size       int64  `json:"size"`
	Offset     int64  `json:"offset"`
	ChunkBytes int64  `json:"chunk_bytes"`
}

func originalUploadState(state staging.State) syncOriginalUpload {
	return syncOriginalUpload{ID: state.ID, Key: state.Manifest.DestinationToken, Size: state.Manifest.Files[0].Size, Offset: state.Offsets[0], ChunkBytes: staging.ChunkBytes}
}

func (s *Server) syncOriginalAction(ctx context.Context, id string, fn func(*staging.Session) error) (syncOriginalUpload, error) {
	var result syncOriginalUpload
	err := s.syncUploads.With(ctx, id, func(session *staging.Session) error {
		if fn != nil {
			if err := fn(session); err != nil {
				return err
			}
		}
		result = originalUploadState(session.State())
		return nil
	})
	return result, err
}

func (s *Server) syncOriginalRoutes() {
	huma.Register(s.contract, syncOperation("sync-original-status", "GET", "originals/{key}"), func(ctx context.Context, in *syncOriginalInput) (*output[library.SyncOriginalStatus], error) {
		return response(s.library.SyncOriginal(ctx, in.Key))
	})
	huma.Register(s.contract, syncOperation("sync-original-content", "GET", "originals/{key}/content"), func(ctx context.Context, in *syncOriginalInput) (*huma.StreamResponse, error) {
		reader, err := s.library.OpenSyncOriginal(ctx, in.Key)
		if err != nil {
			return nil, failure(err)
		}
		return &huma.StreamResponse{Body: func(hctx huma.Context) {
			defer reader.Close()
			r, w := humago.Unwrap(hctx)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("ETag", `"`+in.Key+`"`)
			http.ServeContent(w, r, "original", time.Time{}, reader)
		}}, nil
	})
	huma.Register(s.contract, syncOperation("sync-original-upload-create", "POST", "original-uploads"), func(ctx context.Context, in *struct {
		Body struct {
			Key  string `json:"key"`
			Size int64  `json:"size" minimum:"0"`
		}
	}) (*output[syncOriginalUpload], error) {
		key, err := blob.ParseKey(in.Body.Key)
		if err != nil {
			return nil, invalid("Invalid original address")
		}
		manifest := staging.Manifest{Box: "sync", Folder: "original", Target: key.Hex(), DestinationToken: string(key), Files: []staging.File{{Path: "original", Size: in.Body.Size, SHA256: key.Hex()}}}
		state, err := s.syncUploads.Create(ctx, "sync-original:"+string(key), manifest)
		if err != nil {
			return nil, failure(err)
		}
		return response(originalUploadState(state), nil)
	})
	huma.Register(s.contract, syncOperation("sync-original-upload-status", "GET", "original-uploads/{id}"), func(ctx context.Context, in *sessionInput) (*output[syncOriginalUpload], error) {
		return response(s.syncOriginalAction(ctx, in.ID, nil))
	})
	huma.Register(s.contract, syncOperation("sync-original-upload-prefix", "POST", "original-uploads/{id}/prefix"), func(ctx context.Context, in *sessionPrefix) (*output[syncOriginalUpload], error) {
		if in.Body.Index != 0 {
			return nil, invalid("An original upload contains one file")
		}
		return response(s.syncOriginalAction(ctx, in.ID, func(session *staging.Session) error {
			return session.MatchPrefix(ctx, 0, in.Body.Offset, in.Body.SHA256)
		}))
	})
	huma.Register(s.contract, syncOperation("sync-original-upload-finish", "POST", "original-uploads/{id}/finish"), func(ctx context.Context, in *sessionInput) (*output[syncOriginalUpload], error) {
		return response(s.syncOriginalAction(ctx, in.ID, func(session *staging.Session) error {
			paths, err := session.Paths(ctx)
			if err != nil {
				return err
			}
			file, err := os.Open(paths[0])
			if err != nil {
				return err
			}
			defer file.Close()
			state := originalUploadState(session.State())
			if err := s.library.StoreSyncOriginal(ctx, state.Key, state.Size, file); err != nil {
				return err
			}
			return session.Cancel()
		}))
	})
	op := syncOperation("sync-original-upload-chunk", "PUT", "original-uploads/{id}/files/{index}")
	op.RequestBody = &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"application/octet-stream": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}}
	op.Responses = map[string]*huma.Response{"200": {Description: "Durable original upload progress", Content: map[string]*huma.MediaType{"application/json": {Schema: s.contract.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[syncOriginalUpload](), true, "SyncOriginalUpload")}}}}
	huma.Register(s.contract, op, func(ctx context.Context, in *chunkInput) (*huma.StreamResponse, error) {
		if in.Index != 0 {
			return nil, invalid("An original upload contains one file")
		}
		return &huma.StreamResponse{Body: func(hctx huma.Context) {
			r, w := humago.Unwrap(hctx)
			select {
			case s.uploadSlots <- struct{}{}:
				defer func() { <-s.uploadSlots }()
			case <-ctx.Done():
				writeFailure(w, ctx.Err())
				return
			}
			data, err := io.ReadAll(io.LimitReader(r.Body, staging.ChunkBytes+1))
			if err != nil || int64(len(data)) != in.Length || len(data) > staging.ChunkBytes {
				writeFailure(w, invalid("Incomplete original block; query progress before retrying"))
				return
			}
			status, err := s.syncOriginalAction(ctx, in.ID, func(session *staging.Session) error {
				return session.Append(ctx, 0, in.Offset, data, in.SHA256)
			})
			if err != nil {
				writeFailure(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(status)
		}}, nil
	})
}
