package api

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/syncclient"
)

type clientConnectionStatus struct {
	CatalogComplete bool   `json:"catalog_complete"`
	CatalogOpID     int64  `json:"catalog_op_id"`
	Syncing         bool   `json:"syncing"`
	ManualRequired  bool   `json:"manual_required"`
	State           string `json:"state" enum:"connected,unreachable,unauthorized,incompatible,unconfigured"`
	Server          string `json:"server,omitempty"`
	PendingDownload *int64 `json:"pending_download,omitempty"`
	PendingUpload   int    `json:"pending_upload"`
}

func (s *Server) clientConnectionStatusRoute() {
	huma.Get(s.contract, Prefix+"/client/connection-status", func(ctx context.Context, _ *struct{}) (*output[clientConnectionStatus], error) {
		settings, err := s.client.InstanceSettings(ctx)
		if err != nil {
			return nil, failure(err)
		}
		local, err := s.client.Status(ctx)
		if err != nil {
			return nil, failure(err)
		}
		status := clientConnectionStatus{State: "unconfigured", PendingUpload: len(local.Pending), CatalogOpID: local.Catalog.Confirmed.OpID, CatalogComplete: local.Catalog.Complete, Syncing: s.autoSyncRunning.Load() > 0, ManualRequired: local.BlockedID != ""}
		if settings.Connection == nil || settings.Connection.Server == "" || !settings.CredentialConfigured {
			return response(status, nil)
		}
		status.Server = settings.Connection.Server
		status.State = "unreachable"
		probe, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		peer, err := s.clientPeer(probe)
		if err == nil {
			caps, probeErr := peer.Capabilities(probe)
			err = probeErr
			if err == nil {
				if caps.LibraryID.String() != settings.LibraryID || library.ValidateSyncCapabilities(caps) != nil {
					status.State = "incompatible"
				} else {
					head, headErr := peer.Head(probe)
					err = headErr
					if err == nil {
						if head.LibraryID.String() != settings.LibraryID || head.OpID < local.Catalog.Confirmed.OpID || (head.OpID == local.Catalog.Confirmed.OpID && head.Sum != local.Catalog.Confirmed.Sum) {
							status.State = "incompatible"
						} else {
							status.State = "connected"
							// Authoritative journal IDs form a contiguous sequence.
							pending := head.OpID - local.Catalog.Confirmed.OpID
							status.PendingDownload = &pending
							status.ManualRequired = status.ManualRequired || (pending > 0 && status.PendingUpload > 0) || (status.PendingUpload > 0 && !caps.ConditionalSubmit)
							for _, entry := range local.Pending {
								status.ManualRequired = status.ManualRequired || entry.Conflict != ""
							}
						}
					}
				}
			}
		}
		var remote *syncclient.HTTPError
		if errors.As(err, &remote) && (remote.Status == 401 || remote.Status == 403) {
			status.State = "unauthorized"
		}
		return response(status, nil)
	})
}

// Cache misses are fetched before entering any client mutation or history lock.
// CacheOriginal owns resumption, quota admission, identity and content checks.
func (s *Server) cacheClientOriginal(ctx context.Context, key string) error {
	settings, err := s.client.InstanceSettings(ctx)
	if err != nil {
		return err
	}
	if settings.Connection == nil || settings.Connection.Server == "" || !settings.CredentialConfigured {
		return library.ErrNotCached
	}
	err = s.client.CacheOriginal(ctx, key, syncclient.NewClientSource(s.client))
	var network net.Error
	var remote *syncclient.HTTPError
	if errors.As(err, &network) || errors.As(err, &remote) {
		return library.ErrNotCached
	}
	return err
}
