package api

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/syncclient"
)

type clientConnectInput struct {
	Body struct {
		Server    string `json:"server" minLength:"1" maxLength:"4096"`
		Token     string `json:"token" minLength:"1" maxLength:"2048" writeOnly:"true"`
		AllowHTTP bool   `json:"allow_http"`
	}
}

type clientSyncResponse struct {
	Result library.ClientSyncResult `json:"result"`
	Status library.ClientStatus     `json:"status"`
}

func (s *Server) clientPeer(ctx context.Context) (*syncclient.Peer, error) {
	connection, token, err := s.client.Connection(ctx)
	if err != nil {
		return nil, err
	}
	return syncclient.New(connection.Server, token, syncclient.Options{AllowHTTP: connection.AllowHTTP})
}

func (s *Server) clientSyncRoutes() {
	huma.Get(s.contract, Prefix+"/client/pending/{id}/edit", func(ctx context.Context, in *struct {
		ID string `path:"id" minLength:"1" maxLength:"256"`
	}) (*output[library.ClientConflictEdit], error) {
		draft, err := s.client.ConflictEdit(ctx, in.ID)
		return response(draft, err)
	})
	huma.Post(s.contract, Prefix+"/client/pending/{id}/resolve", func(ctx context.Context, in *struct {
		ID   string `path:"id" minLength:"1" maxLength:"256"`
		Body struct {
			Replacement *library.SyncProposal `json:"replacement,omitempty"`
			Reason      string                `json:"reason" minLength:"1" maxLength:"4096"`
		}
	}) (*output[library.ClientStatus], error) {
		if err := s.client.Resolve(ctx, in.ID, in.Body.Replacement, in.Body.Reason); err != nil {
			return nil, failure(err)
		}
		status, err := s.client.Status(ctx)
		return response(status, err)
	})
	huma.Put(s.contract, Prefix+"/client/connection", func(ctx context.Context, in *clientConnectInput) (*output[library.ClientStatus], error) {
		peer, err := syncclient.New(in.Body.Server, in.Body.Token, syncclient.Options{AllowHTTP: in.Body.AllowHTTP})
		if err != nil {
			return nil, invalid(err.Error())
		}
		caps, err := peer.Capabilities(ctx)
		if err != nil {
			return nil, failure(err)
		}
		if err := library.ValidateSyncCapabilities(caps); err != nil {
			return nil, failure(err)
		}
		if err := s.client.SetConnection(ctx, caps.LibraryID, library.ClientConnection{Server: in.Body.Server, AllowHTTP: in.Body.AllowHTTP}, in.Body.Token); err != nil {
			return nil, failure(err)
		}
		if err := s.client.AdoptSyncSettings(ctx, caps); err != nil {
			return nil, failure(err)
		}
		status, err := s.client.Status(ctx)
		return response(status, err)
	})
	huma.Post(s.contract, Prefix+"/client/sync", func(ctx context.Context, _ *struct{ Body struct{} }) (*output[clientSyncResponse], error) {
		peer, err := s.clientPeer(ctx)
		if err != nil {
			return nil, failure(err)
		}
		result, err := s.client.Synchronize(ctx, peer)
		if err != nil && !errors.Is(err, library.ErrSyncConflict) {
			return nil, failure(err)
		}
		// A known merge conflict is an actionable result. It is retained in Status;
		// transport failures remain failures and never imply a rejected submission.
		status, err := s.client.Status(ctx)
		return response(clientSyncResponse{Result: result, Status: status}, err)
	})
	huma.Post(s.contract, Prefix+"/client/offline/{owner}/prepare", func(ctx context.Context, in *struct {
		Owner string `path:"owner" minLength:"1" maxLength:"4096"`
		Body  struct{}
	}) (*output[library.ClientOfflineStatus], error) {
		status, err := s.client.PrepareOffline(ctx, in.Owner, syncclient.NewClientSource(s.client), s.clientRenderer)
		return response(status, err)
	})
}
