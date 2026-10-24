package api

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/syncclient"
	"github.com/jinzhao1994/kist/internal/wire"
)

type syncAccessBackend interface {
	SyncCredentials(context.Context) ([]library.SyncCredential, error)
	AuthorizeSyncWithNote(context.Context, string) (library.NewSyncCredential, error)
	SetSyncCredentialNote(context.Context, string, string) (bool, error)
	RevokeSync(context.Context, string) (bool, error)
	CheckSyncCredential(context.Context, string) (bool, error)
}

func (s *Server) syncAccessBackend() syncAccessBackend {
	if s.client != nil {
		return s.client
	}
	return s.library
}

type syncOriginalBackend interface {
	SyncOriginal(context.Context, string) (library.SyncOriginalStatus, error)
	StoreSyncOriginal(context.Context, string, int64, io.Reader) error
}

func (s *Server) syncOriginalBackend() syncOriginalBackend {
	if s.client != nil {
		return s.client
	}
	return s.library
}
func (s *Server) openSyncOriginal(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	if s.client == nil {
		return s.library.OpenSyncOriginal(ctx, key)
	}
	r, err := s.client.OpenOriginal(ctx, key)
	if errors.Is(err, library.ErrNotCached) {
		if err = s.cacheClientOriginal(ctx, key); err == nil {
			return s.client.OpenOriginal(ctx, key)
		}
	}
	return r, err
}
func (s *Server) describeSyncDerived(ctx context.Context, key, recipe string, retry bool) (library.SyncDerived, error) {
	if s.client == nil {
		return s.media.DescribeSyncDerived(ctx, key, recipe, retry)
	}
	return s.previews.(*library.ClientMedia).DescribeSyncDerived(ctx, key, recipe, retry)
}
func (s *Server) openSyncDerived(ctx context.Context, key, recipe, generation, content string) (library.MediaJob, io.ReadSeekCloser, error) {
	if s.client == nil {
		return s.media.OpenSyncDerived(ctx, key, recipe, generation, content)
	}
	d, r, err := s.client.OpenDerived(ctx, key, recipe)
	if err != nil {
		return d.Job, nil, err
	}
	if d.Job.Generation != generation || d.ContentKey != content {
		r.Close()
		return d.Job, nil, library.ErrSyncConflict
	}
	return d.Job, r, nil
}
func (s *Server) syncRouteContext(r *http.Request) (context.Context, error) {
	route := []domain.ID{}
	values := r.Header.Values("X-Kist-Route")
	if len(values) > 1 {
		return r.Context(), library.ErrSyncConflict
	}
	if len(values) == 1 {
		if len(values[0]) > 32*37 {
			return r.Context(), library.ErrSyncConflict
		}
		for _, raw := range strings.Split(values[0], ",") {
			id, err := domain.ParseID(raw)
			if err != nil {
				return r.Context(), err
			}
			route = append(route, id)
		}
	}
	var node domain.ID
	if s.client != nil {
		caps, err := s.client.RelayCapabilities(r.Context())
		if err != nil {
			return r.Context(), err
		}
		node = caps.NodeID
	} else {
		head, err := s.library.SyncHead(r.Context())
		if err != nil {
			return r.Context(), err
		}
		node = head.LibraryID
	}
	return library.AppendRelayRoute(library.WithRelayRoute(r.Context(), route), node)
}
func (s *Server) relayCapabilities(ctx context.Context) (library.SyncCapabilities, error) {
	caps, err := s.client.RelayCapabilities(ctx)
	if err != nil {
		return caps, err
	}
	peer, err := s.clientPeer(ctx)
	if err != nil {
		return caps, nil
	}
	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	parent, err := peer.Capabilities(probe)
	if err != nil {
		var network net.Error
		var httpErr *syncclient.HTTPError
		if errors.As(err, &network) || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &httpErr) && (httpErr.Status == 401 || httpErr.Status == 403 || httpErr.Status >= 500)) {
			return caps, nil
		}
		return caps, err
	}
	if err := s.client.ValidateBoundUpstream(ctx, parent); err != nil {
		return caps, err
	}
	node := parent.NodeID
	if node.IsZero() {
		node = parent.LibraryID
	}
	caps.Ancestors = append([]domain.ID{node}, parent.Ancestors...)
	return caps, nil
}
func (s *Server) relayRoutes() {
	s.contract.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{"SyncBearer": {Type: "http", Scheme: "bearer", Description: "Independently revocable relay credential"}}
	s.syncOriginalRoutes()
	s.syncDerivedRoutes()
	huma.Register(s.contract, syncOperation("sync-capabilities", "GET", "capabilities"), func(ctx context.Context, _ *struct{}) (*output[library.SyncCapabilities], error) {
		return response(s.relayCapabilities(ctx))
	})
	huma.Register(s.contract, syncOperation("sync-head", "GET", "head"), func(ctx context.Context, _ *struct{}) (*output[library.SyncBoundary], error) {
		status, err := s.client.CatalogStatus(ctx)
		if err == nil && !status.Complete {
			err = library.ErrSyncDependency
		}
		return response(status.Confirmed, err)
	})
	huma.Register(s.contract, syncOperation("sync-catalog", "POST", "catalog"), func(ctx context.Context, in *syncCatalogInput) (*output[library.SyncCatalogPage], error) {
		return response(s.client.RelayRootCatalog(ctx, in.Body.Boundary, in.Body.After, in.Body.Limit, in.Body.MaxBytes))
	})
	huma.Register(s.contract, syncOperation("relay-head", "GET", "relay/head"), func(ctx context.Context, _ *struct{}) (*output[library.RelayHead], error) {
		return response(s.client.RelayHead(ctx))
	})
	huma.Register(s.contract, syncOperation("relay-catalog", "POST", "relay/catalog"), func(ctx context.Context, in *struct {
		Body struct {
			Head     library.RelayHead `json:"head"`
			After    int               `json:"after"`
			Limit    int               `json:"limit"`
			MaxBytes int               `json:"max_bytes"`
		}
	}) (*output[library.RelayPage], error) {
		return response(s.client.RelayCatalog(ctx, in.Body.Head, in.Body.After, in.Body.Limit, in.Body.MaxBytes))
	})
	huma.Register(s.contract, syncOperation("relay-submit", "POST", "relay/submit"), func(ctx context.Context, in *struct {
		Body struct {
			LibraryID domain.ID            `json:"library_id"`
			Proposal  library.SyncProposal `json:"proposal"`
			Expected  *library.RelayHead   `json:"expected,omitempty"`
		}
	}) (*output[relaySubmitResponse], error) {
		var result library.RelayOutcome
		var err error
		if in.Body.Expected != nil {
			result, err = s.client.ReceiveRelayAt(ctx, in.Body.LibraryID, in.Body.Proposal, *in.Body.Expected)
		} else {
			result, err = s.client.ReceiveRelay(ctx, in.Body.LibraryID, in.Body.Proposal)
		}
		if err == nil {
			s.wakeAutoSync()
			return response(relaySubmitResponse{Result: &result}, nil)
		}
		var rejected *library.SyncRejectedError
		problem := failure(err).(*HTTPError).ErrorFields
		return response(relaySubmitResponse{Rejected: errors.As(err, &rejected), Error: &problem}, nil)
	})
}

type relaySubmitResponse struct {
	Result   *library.RelayOutcome `json:"result,omitempty"`
	Rejected bool                  `json:"rejected"`
	Error    *wire.Error           `json:"error,omitempty"`
}
