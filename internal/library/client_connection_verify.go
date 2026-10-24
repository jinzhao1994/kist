package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// VerifyAndSetConnection checks all known authoritative history before changing
// the connection. Per-event checksums are not cumulative prefix proofs.
func (c *Client) VerifyAndSetConnection(ctx context.Context, peer SyncPeer, connection ClientConnection, token, revision string) error {
	if err := ValidateClientConnection(connection, token); err != nil {
		return err
	}
	releaseSync, err := lockfile.New(filepath.Join(c.root, "sync.lock")).Acquire(ctx)
	if err != nil {
		return err
	}
	defer releaseSync()
	caps, err := peer.Capabilities(ctx)
	if err != nil {
		return err
	}
	if err := ValidateSyncCapabilities(caps); err != nil {
		return err
	}

	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	known := map[int64]string{}
	receipts := map[int64]pendingEntry{}
	var before ClientCatalogStatus
	var configRevision string
	err = func() error {
		settings, err := c.settings()
		if err != nil {
			return err
		}
		if settings.LibraryID != caps.LibraryID {
			return ErrSyncConflict
		}
		raw, err := os.ReadFile(filepath.Join(c.root, "kist.toml"))
		if err != nil {
			return err
		}
		configRevision = config.Revision(raw)
		if revision != "" && revision != configRevision {
			return config.ErrChanged
		}
		before, err = c.catalogStatusLocked()
		if err != nil {
			return err
		}
		if err := c.catalog.Iter(func(line journal.Line) error {
			known[line.ID] = line.Sum
			return ctx.Err()
		}); err != nil {
			return err
		}
		// A partially downloaded transfer must remain resumable at the same target.
		if before.Transfer != nil {
			known[before.Transfer.OpID] = before.Transfer.Sum
		}
		queue, err := c.queue.read(ctx)
		if err != nil {
			return err
		}
		for _, entry := range queue.entries {
			if entry.Receipt != nil {
				receipts[entry.Receipt.OpID] = entry
			}
		}
		return nil
	}()
	release()
	if err != nil {
		return err
	}

	head, err := peer.Head(ctx)
	if err != nil {
		return err
	}
	if head.LibraryID != caps.LibraryID || head.OpID < 0 || (head.OpID == 0) != (head.Sum == "") {
		return ErrSyncConflict
	}
	required := before.Downloaded
	for id := range known {
		required = max(required, id)
	}
	for id := range receipts {
		required = max(required, id)
	}
	if head.OpID < required {
		return fmt.Errorf("%w: remote history is behind this client", ErrSyncConflict)
	}
	for after := int64(0); after < required; {
		page, err := peer.Catalog(ctx, head, after, 256, 4<<20)
		if err != nil {
			return err
		}
		if page.Boundary != head || page.After != after || page.Next <= after || page.Next > head.OpID || page.Done != (page.Next == head.OpID) {
			return ErrSyncConflict
		}
		next := after
		for _, raw := range page.Lines {
			line, err := journal.UnmarshalLine(raw)
			if err != nil {
				return err
			}
			if line.ID != next+1 {
				return ErrSyncConflict
			}
			next = line.ID
			if sum, ok := known[line.ID]; ok && sum != line.Sum {
				return fmt.Errorf("%w: remote history differs at operation %d", ErrSyncConflict, line.ID)
			}
			if entry, ok := receipts[line.ID]; ok {
				if line.Sync == nil || line.Sync.ID != entry.Proposal.ID || line.Sync.Digest != entry.Digest || line.Sync.AcceptedAt != journal.FormatTime(entry.Receipt.AcceptedAt) {
					return fmt.Errorf("%w: remote history does not contain an acknowledged operation", ErrSyncConflict)
				}
			}
		}
		if next != page.Next {
			return ErrSyncConflict
		}
		after = next
	}

	release, err = c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	current, err := c.catalogStatusLocked()
	if err != nil {
		return err
	}
	if current.Confirmed != before.Confirmed || current.Downloaded != before.Downloaded || current.Complete != before.Complete || !sameTransfer(current.Transfer, before.Transfer) {
		return ErrSyncConflict
	}
	return updateInstanceConfig(ctx, c.root, configRevision, map[string]any{
		"client.server": connection.Server, "client.allow_http": connection.AllowHTTP,
		"client.token": token, "client.timezone": caps.Timezone,
	})
}

func sameTransfer(a, b *SyncBoundary) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
