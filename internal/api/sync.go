package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

const maxSyncBody = 20 << 20

type syncCatalogInput struct {
	Body struct {
		Boundary library.SyncBoundary `json:"boundary"`
		After    int64                `json:"after" minimum:"0"`
		Limit    int                  `json:"limit" minimum:"1" maximum:"256"`
		MaxBytes int                  `json:"max_bytes" minimum:"1" maximum:"4194304"`
	}
}

// SyncSubmitResponse explicitly distinguishes definitive rejection from an
// uncertain commit. HTTP/proxy failures outside this envelope are uncertain.
type SyncSubmitResponse struct {
	Result   *library.SyncResult `json:"result,omitempty"`
	Rejected bool                `json:"rejected"`
	Error    *wire.Error         `json:"error,omitempty"`
}

func syncOperation(id, method, suffix string) huma.Operation {
	return huma.Operation{OperationID: id, Method: method, Path: Prefix + "/sync/" + suffix, Security: []map[string][]string{{"SyncBearer": {}}}, MaxBodyBytes: maxSyncBody}
}

func (s *Server) syncRoutes() {
	s.contract.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"SyncBearer": {Type: "http", Scheme: "bearer", Description: "Independently revocable NAS synchronization credential"},
	}
	s.syncOriginalRoutes()
	huma.Register(s.contract, syncOperation("sync-capabilities", "GET", "capabilities"), func(ctx context.Context, _ *struct{}) (*output[library.SyncCapabilities], error) {
		head, err := s.library.SyncHead(ctx)
		if err != nil {
			return nil, failure(err)
		}
		return response(library.SyncCapabilities{DeletionSnapshots: true, Timezone: s.library.Zone().String(), Version: 1, LibraryID: head.LibraryID, JournalVersion: journal.Version, Actions: library.SyncActions(), UndoReferences: true}, nil)
	})
	huma.Register(s.contract, syncOperation("sync-head", "GET", "head"), func(ctx context.Context, _ *struct{}) (*output[library.SyncBoundary], error) {
		return response(s.library.SyncHead(ctx))
	})
	huma.Register(s.contract, syncOperation("sync-catalog", "POST", "catalog"), func(ctx context.Context, in *syncCatalogInput) (*output[library.SyncCatalogPage], error) {
		return response(s.library.SyncCatalog(ctx, in.Body.Boundary, in.Body.After, in.Body.Limit, in.Body.MaxBytes))
	})
	huma.Register(s.contract, syncOperation("sync-submit", "POST", "submit"), func(ctx context.Context, in *struct{ Body library.SyncProposal }) (*output[SyncSubmitResponse], error) {
		result, err := s.library.AcceptSync(ctx, in.Body)
		if err == nil {
			return response(SyncSubmitResponse{Result: &result}, nil)
		}
		var rejected *library.SyncRejectedError
		problem := failure(err).(*HTTPError).ErrorFields
		return response(SyncSubmitResponse{Rejected: errors.As(err, &rejected), Error: &problem}, nil)
	})
}

func (s *Server) authorizeSync(w http.ResponseWriter, r *http.Request) bool {
	values := r.Header.Values("Authorization")
	var token string
	if len(values) == 1 {
		parts := strings.Fields(values[0])
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = parts[1]
		}
	}
	ok, err := s.library.CheckSyncCredential(r.Context(), token)
	if err != nil {
		writeFailure(w, err)
		return false
	}
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeFailure(w, huma.NewError(http.StatusUnauthorized, "A valid sync credential is required"))
		return false
	}
	return true
}
