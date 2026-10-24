package library

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
)

type relayRouteKey struct{}

func RelayRoute(ctx context.Context) []domain.ID {
	v, _ := ctx.Value(relayRouteKey{}).([]domain.ID)
	return slices.Clone(v)
}
func WithRelayRoute(ctx context.Context, route []domain.ID) context.Context {
	return context.WithValue(ctx, relayRouteKey{}, slices.Clone(route))
}
func AppendRelayRoute(ctx context.Context, id domain.ID) (context.Context, error) {
	route := RelayRoute(ctx)
	if len(route) >= 32 || slices.Contains(route, id) {
		return ctx, ErrSyncConflict
	}
	return WithRelayRoute(ctx, append(route, id)), nil
}
func (c *Client) RelayCapabilities(ctx context.Context) (SyncCapabilities, error) {
	cfg, err := config.Load(filepath.Join(c.root, "kist.toml"))
	if err != nil {
		return SyncCapabilities{}, err
	}
	return SyncCapabilities{Relay: true, ConditionalSubmit: true, NodeID: cfg.Client.ID, Ancestors: cfg.Client.Ancestors, DerivedPreviews: true, DeletionSnapshots: true, Timezone: cfg.Client.Timezone, Version: 1, LibraryID: cfg.Client.LibraryID, JournalVersion: journal.Version, Actions: SyncActions(), UndoReferences: true}, ctx.Err()
}
func (c *Client) ValidateUpstream(ctx context.Context, caps SyncCapabilities) error {
	if err := ValidateSyncCapabilities(caps); err != nil {
		return err
	}
	settings, err := c.settings()
	if err != nil {
		return err
	}
	if caps.LibraryID != settings.LibraryID || caps.NodeID == settings.ID || slices.Contains(caps.Ancestors, settings.ID) || len(caps.Ancestors) >= 31 {
		return ErrSyncConflict
	}
	seen := map[domain.ID]bool{caps.NodeID: true}
	for _, id := range caps.Ancestors {
		if id.IsZero() || seen[id] {
			return ErrSyncConflict
		}
		seen[id] = true
	}
	return nil
}

// ValidateBoundUpstream prevents a configured address from silently changing nodes.
func (c *Client) ValidateBoundUpstream(ctx context.Context, caps SyncCapabilities) error {
	if err := c.ValidateUpstream(ctx, caps); err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(c.root, "kist.toml"))
	if err != nil {
		return err
	}
	node := caps.NodeID
	if node.IsZero() {
		node = caps.LibraryID
	}
	if !cfg.Client.ParentID.IsZero() && cfg.Client.ParentID != node {
		return ErrSyncConflict
	}
	return nil
}
