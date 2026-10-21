package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	staging "github.com/jinzhao1994/kist/internal/upload"
)

type sessionStatus struct {
	Conflicts *library.UploadConflicts `json:"conflicts,omitempty"`
	Session   *staging.State           `json:"session,omitempty"`
	OpID      int64                    `json:"op_id,omitempty"`
}

type sessionInput struct {
	ID string `path:"id" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
}
type sessionCommand struct {
	ID   string `path:"id" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
	Body struct{}
}
type sessionFinish struct {
	ID   string `path:"id" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
	Body struct {
		Digests []string `json:"digests,omitempty"`
	}
}
type sessionPrefix struct {
	ID   string `path:"id" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
	Body struct {
		Index  int    `json:"index" minimum:"0"`
		Offset int64  `json:"offset" minimum:"0"`
		SHA256 string `json:"sha256" pattern:"^[0-9a-f]{64}$"`
	}
}
type sessionCreate struct {
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		ConflictMode  string         `json:"conflict_mode,omitempty"`
		ConflictToken string         `json:"conflict_token,omitempty"`
		Version       int            `json:"version"`
		Box           string         `json:"box"`
		Folder        string         `json:"folder"`
		Files         []staging.File `json:"files"`
	}
}

func sessionRequest(id string, m staging.Manifest) library.Request {
	// The target snapshot is a server-side precondition, not business input.
	// A receipt must remain retrievable after the target changes or is deleted.
	version := staging.Version
	if m.Protocol == 2 {
		version = 2
	}
	raw, _ := json.Marshal(struct {
		Version       int
		Box, Folder   string
		Files         []staging.File
		ConflictMode  string `json:",omitempty"`
		ConflictToken string `json:",omitempty"`
	}{version, m.Box, m.Folder, m.Files, m.ConflictMode, m.ConflictToken})
	hash := sha256.Sum256(raw)
	return library.Request{ID: "upload." + id, Digest: hex.EncodeToString(hash[:])}
}

func (s *Server) uploadSessionRoutes() {
	huma.Post(s.contract, Prefix+"/upload-sessions", func(ctx context.Context, in *sessionCreate) (*output[sessionStatus], error) {
		if in.Body.Version != 1 && in.Body.Version != 2 {
			return nil, failure(&staging.Problem{Kind: "unsupported_version"})
		}
		if !(library.Request{ID: in.Key, Digest: strings.Repeat("0", 64)}).Valid() {
			return nil, failure(library.ErrInvalidRequest)
		}
		box, err := parseID(in.Body.Box)
		if err != nil {
			return nil, err
		}
		folder, err := parseID(in.Body.Folder)
		if err != nil {
			return nil, err
		}
		files := append([]staging.File(nil), in.Body.Files...)
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		m := staging.Manifest{Box: box.String(), Folder: folder.String(), Files: files, ConflictMode: in.Body.ConflictMode, ConflictToken: in.Body.ConflictToken}
		if m.ConflictMode != "" && m.ConflictMode != "overwrite" && m.ConflictMode != "keep_both" {
			return nil, invalid("Unknown conflict mode")
		}
		if len(files) == 0 || len(files) > s.maxUploadFiles {
			return nil, invalid("Invalid upload file count")
		}
		if in.Body.Version == 2 {
			m.Protocol = 2
		}
		id := staging.ID(in.Key)
		receipt, found, err := s.library.CheckRequest(ctx, sessionRequest(id, m))
		if err != nil {
			return nil, failure(err)
		}
		if found {
			return response(sessionStatus{OpID: receipt.OpID}, nil)
		}
		// A lost creation response must return the original snapshot, even if
		// the target changed since then. The final commit rechecks that snapshot.
		var existing staging.State
		existingErr := s.uploads.With(ctx, id, func(v *staging.Session) error {
			existing = v.State()
			if sessionRequest(id, existing.Manifest).Digest != sessionRequest(id, m).Digest {
				return library.ErrRequestConflict
			}
			return nil
		})
		if existingErr == nil {
			return response(sessionStatus{Session: &existing}, nil)
		}
		var absent *staging.Problem
		if !errors.As(existingErr, &absent) || absent.Kind != "not_found" {
			return nil, failure(existingErr)
		}
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.Path
		}
		preview, err := s.library.PreviewUpload(ctx, domain.BoxID{ID: box}, domain.FolderID{ID: folder}, paths)
		if err != nil {
			return nil, failure(err)
		}
		if m.ConflictToken != "" && m.ConflictToken != preview.Token && len(preview.Paths) == 0 {
			return nil, failure(library.ErrUploadTargetChanged)
		}
		if len(preview.Paths) > 0 && (m.ConflictMode == "" || m.ConflictToken != preview.Token) {
			return response(sessionStatus{Conflicts: &preview}, nil)
		}
		if m.ConflictMode == "overwrite" && !preview.CanOverwrite {
			return nil, invalid("A folder cannot be overwritten by a file")
		}
		// Retain an absent-name snapshot too, so a late collision is never silently overwritten.
		m.DestinationToken = preview.Token
		m.Target, err = s.library.UploadTarget(ctx, domain.BoxID{ID: box}, domain.FolderID{ID: folder})
		if err != nil {
			return nil, failure(err)
		}
		state, err := s.uploads.Create(ctx, in.Key, m)
		return response(sessionStatus{Session: &state}, err)
	})
	huma.Get(s.contract, Prefix+"/upload-sessions/{id}", func(ctx context.Context, in *sessionInput) (*output[sessionStatus], error) {
		status, err := s.sessionAction(ctx, in.ID, func(v *staging.Session) error { return nil })
		return response(status, err)
	})
	huma.Post(s.contract, Prefix+"/upload-sessions/{id}/resume", func(ctx context.Context, in *sessionCommand) (*output[sessionStatus], error) {
		status, err := s.sessionAction(ctx, in.ID, func(v *staging.Session) error { return v.Renew() })
		return response(status, err)
	})
	huma.Delete(s.contract, Prefix+"/upload-sessions/{id}", func(ctx context.Context, in *sessionInput) (*output[sessionStatus], error) {
		status, err := s.sessionAction(ctx, in.ID, func(v *staging.Session) error { return v.Cancel() })
		var p *staging.Problem
		gone := errors.As(err, &p) && (p.Kind == "not_found" || p.Kind == "canceled" || p.Kind == "expired")
		if (err == nil || gone) && status.OpID == 0 {
			// The cancellation marker is durable even if cleanup fails. Periodic
			// cleanup retries it; an archived receipt is never canceled.
			if sweepErr := s.uploads.Sweep(ctx); sweepErr != nil {
				slog.Warn("upload cleanup", "error", sweepErr)
			}
		}
		if gone {
			err = nil
		}
		return response(status, err)
	})
	huma.Post(s.contract, Prefix+"/upload-sessions/{id}/complete", func(ctx context.Context, in *sessionFinish) (*output[sessionStatus], error) {
		status, err := s.sessionAction(ctx, in.ID, func(v *staging.Session) error {
			if err := v.Seal(in.Body.Digests); err != nil {
				return err
			}
			return s.completeSession(ctx, v)
		})
		return response(status, err)
	})
	huma.Post(s.contract, Prefix+"/upload-sessions/{id}/prefix", func(ctx context.Context, in *sessionPrefix) (*output[sessionStatus], error) {
		status, err := s.sessionAction(ctx, in.ID, func(v *staging.Session) error {
			return v.MatchPrefix(ctx, in.Body.Index, in.Body.Offset, in.Body.SHA256)
		})
		return response(status, err)
	})
	s.uploadChunkRoute()
}

// sessionAction checks receipts again under the staging lock. A final response
// may be lost after commit but before the local completion marker was saved.
func (s *Server) sessionAction(ctx context.Context, id string, fn func(*staging.Session) error) (sessionStatus, error) {
	var status sessionStatus
	receipt, found, err := s.library.RequestStatus(ctx, "upload."+id)
	if err != nil {
		return status, err
	}
	if found {
		return sessionStatus{OpID: receipt.OpID}, nil
	}
	err = s.uploads.With(ctx, id, func(v *staging.Session) error {
		receipt, found, err := s.library.CheckRequest(ctx, sessionRequest(id, v.State().Manifest))
		if err != nil {
			return err
		}
		if found {
			status.OpID = receipt.OpID
			return nil
		}
		if err := fn(v); err != nil {
			return err
		}
		state := v.State()
		status.Session = &state
		status.OpID = state.OpID
		return nil
	})
	if err != nil {
		// Cleanup can remove a completed session while this request waits for
		// the lock. The archive receipt takes precedence over missing staging.
		receipt, found, lookupErr := s.library.RequestStatus(ctx, "upload."+id)
		if lookupErr == nil && found {
			return sessionStatus{OpID: receipt.OpID}, nil
		}
	}
	return status, err
}

func (s *Server) completeSession(ctx context.Context, v *staging.Session) error {
	state := v.State()
	started := time.Now()
	defer func() {
		slog.Info("upload processing", "session", state.ID, "elapsed_ms", time.Since(started).Milliseconds())
	}()
	paths, err := v.Paths(ctx)
	if err != nil {
		return err
	}
	box, err := parseID(state.Manifest.Box)
	if err != nil {
		return err
	}
	folder, err := parseID(state.Manifest.Folder)
	if err != nil {
		return err
	}
	sources := make([]library.ImportSource, len(paths))
	for i, path := range paths {
		file := state.Manifest.Files[i]
		if state.Manifest.Protocol == 2 {
			file.SHA256 = state.Digests[i]
		}
		sources[i] = library.ImportSource{Path: path, RelativePath: file.Path, Expected: &library.ImportIntegrity{Size: file.Size, SHA256: file.SHA256}}
	}
	checkDestinations := func() error {
		if state.Manifest.DestinationToken == "" {
			return nil
		}
		paths := make([]string, len(sources))
		for i, f := range sources {
			paths[i] = f.RelativePath
		}
		preview, err := s.library.PreviewUpload(ctx, domain.BoxID{ID: box}, domain.FolderID{ID: folder}, paths)
		if err != nil {
			return err
		}
		if preview.Token != state.Manifest.DestinationToken {
			return library.ErrUploadTargetChanged
		}
		return nil
	}
	if err := checkDestinations(); err != nil {
		return err
	}
	prepared, err := s.library.PrepareFiles(ctx, sources, library.ImportOptions{Box: domain.BoxID{ID: box}, Into: domain.FolderID{ID: folder}, Hidden: true, ConflictMode: state.Manifest.ConflictMode})
	if err != nil {
		return err
	}
	if s.uploadFault != nil {
		if err := s.uploadFault("before_commit"); err != nil {
			return err
		}
	}
	result, err := s.library.ExecuteRequest(ctx, sessionRequest(state.ID, state.Manifest), func(ctx context.Context) error {
		target, err := s.library.UploadTarget(ctx, domain.BoxID{ID: box}, domain.FolderID{ID: folder})
		if err != nil {
			return err
		}
		if target != state.Manifest.Target {
			return library.ErrUploadTargetChanged
		}
		if err := checkDestinations(); err != nil {
			return err
		}
		_, err = prepared.Commit(ctx)
		return err
	})
	if err != nil {
		return err
	}
	if s.uploadFault != nil {
		if err := s.uploadFault("after_commit"); err != nil {
			return err
		}
	}
	return v.Complete(result.OpID)
}

type chunkInput struct {
	ID     string `path:"id" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
	Index  int    `path:"index" minimum:"0"`
	Offset int64  `query:"offset" required:"true" minimum:"0"`
	SHA256 string `header:"Upload-SHA256" required:"true" pattern:"^[0-9a-f]{64}$"`
	Length int64  `header:"Upload-Length" required:"true" minimum:"1" maximum:"8388608"`
}

func (s *Server) uploadChunkRoute() {
	huma.Register(s.contract, huma.Operation{
		OperationID: "upload-session-chunk", Method: "PUT", Path: Prefix + "/upload-sessions/{id}/files/{index}",
		Summary:     "Append a verified sequential block, or retry an identical confirmed block",
		Responses:   map[string]*huma.Response{"200": {Description: "Durable progress or archive receipt", Content: map[string]*huma.MediaType{"application/json": {Schema: s.contract.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[sessionStatus](), true, "SessionStatus")}}}},
		RequestBody: &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"application/octet-stream": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}},
	}, func(ctx context.Context, in *chunkInput) (*huma.StreamResponse, error) {
		return &huma.StreamResponse{Body: func(hctx huma.Context) {
			r, w := humago.Unwrap(hctx)
			status, err := s.receiveChunk(r, in)
			if err != nil {
				writeFailure(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(status)
		}}, nil
	})
}

func (s *Server) receiveChunk(r *http.Request, in *chunkInput) (sessionStatus, error) {
	select {
	case s.uploadSlots <- struct{}{}:
		defer func() { <-s.uploadSlots }()
	case <-r.Context().Done():
		return sessionStatus{}, r.Context().Err()
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, staging.ChunkBytes+1))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			return sessionStatus{}, huma.Error413RequestEntityTooLarge("Upload block exceeds the chunk limit")
		}
		return sessionStatus{}, invalid("Incomplete upload block; query progress before retrying")
	}
	if int64(len(data)) != in.Length || len(data) > staging.ChunkBytes {
		return sessionStatus{}, failure(&staging.Problem{Kind: "invalid_chunk"})
	}
	return s.sessionAction(r.Context(), in.ID, func(v *staging.Session) error { return v.Append(r.Context(), in.Index, in.Offset, data, in.SHA256) })
}

// RunUploads sweeps discardable staging until shutdown. It never deletes a
// locked upload or writes an archive operation.
func (s *Server) RunUploads(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		if err := s.uploads.Sweep(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("upload cleanup", "error", err)
		}
		if err := s.syncUploads.Sweep(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("sync upload cleanup", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
