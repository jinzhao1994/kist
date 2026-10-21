package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// ClientSettings identifies the client and the single NAS library it follows.
// Cache limits never apply to pending intents or unconfirmed original files.
type ClientSettings struct {
	Version       int       `json:"version"`
	ID            domain.ID `json:"id"`
	LibraryID     domain.ID `json:"library_id"`
	Timezone      string    `json:"timezone"`
	CacheMaxAge   string    `json:"cache_max_age"`
	CacheMaxBytes int64     `json:"cache_max_bytes"`
}

func (s ClientSettings) validate() error {
	if s.Version != 1 {
		return ErrSyncVersion
	}
	age, err := time.ParseDuration(s.CacheMaxAge)
	if err != nil || age <= 0 || s.CacheMaxBytes <= 0 || s.ID.IsZero() || s.LibraryID.IsZero() {
		return ErrInvalidRequest
	}
	_, err = time.LoadLocation(s.Timezone)
	return err
}

// Client is a partial-content replica with a separate durable pending journal.
// The derived database is never opened through the NAS Library.Open path.
type Client struct {
	root    string
	db      *sql.DB
	lock    *lockfile.Locker
	queue   *pendingQueue
	catalog *journal.Dir
	blobs   *blob.Dir
}

func CreateClient(ctx context.Context, root string, settings ClientSettings) (*Client, error) {
	settings.Version = 1
	if settings.ID.IsZero() {
		settings.ID = domain.NewID()
	}
	if settings.Timezone == "" {
		settings.Timezone = "UTC"
	}
	if settings.CacheMaxAge == "" {
		settings.CacheMaxAge = "720h"
	}
	if settings.CacheMaxBytes == 0 {
		settings.CacheMaxBytes = 10 << 30
	}
	if err := settings.validate(); err != nil {
		return nil, err
	}
	layout, err := prepareRoot(root)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(layout.Root, 0o700); err != nil {
		return nil, err
	}
	for _, name := range []string{"pending", "catalog", "local-blobs", "cache"} {
		if err := os.Mkdir(filepath.Join(layout.Root, name), 0o700); err != nil {
			return nil, err
		}
	}
	if err := writeClientJSON(filepath.Join(layout.Root, "client.json"), settings); err != nil {
		return nil, err
	}
	if err := syncClientDir(filepath.Dir(layout.Root)); err != nil {
		return nil, err
	}
	return OpenClient(ctx, layout.Root)
}

func OpenClient(ctx context.Context, root string) (*Client, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	c := &Client{root: root, lock: lockfile.New(filepath.Join(root, "client.lock"))}
	if _, err := c.settings(); err != nil {
		return nil, err
	}
	if isLibrary(root) {
		return nil, ErrAlreadyLibrary
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if c.queue, err = openPendingQueue(filepath.Join(root, "pending")); err != nil {
		return nil, err
	}
	c.queue.lock = c.lock
	if c.catalog, err = journal.NewDir(filepath.Join(root, "catalog")); err != nil {
		return nil, err
	}
	placement := lockfile.New(filepath.Join(root, "local-blobs.lock"))
	if c.blobs, err = blob.NewDir(filepath.Join(root, "local-blobs"), slog.Default(), blob.WithPlacementLock(placement.Acquire)); err != nil {
		return nil, err
	}
	if c.db, err = store.Open(filepath.Join(root, "client.db")); err != nil {
		return nil, err
	}
	if err = store.Migrate(c.db); err == nil {
		_, err = c.rebuildLocked(ctx)
	}
	if err != nil {
		c.db.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) Close() error { return c.db.Close() }

func (c *Client) settings() (ClientSettings, error) {
	var settings ClientSettings
	if err := readClientJSON(filepath.Join(c.root, "client.json"), &settings); err != nil {
		return settings, err
	}
	return settings, settings.validate()
}

func (c *Client) boundary() (SyncBoundary, bool, error) {
	settings, err := c.settings()
	if err != nil {
		return SyncBoundary{}, false, err
	}
	var head SyncBoundary
	err = readClientJSON(filepath.Join(c.root, "catalog-head.json"), &head)
	if errors.Is(err, os.ErrNotExist) {
		return SyncBoundary{LibraryID: settings.LibraryID}, false, nil
	}
	if err != nil {
		return head, false, err
	}
	if head.LibraryID != settings.LibraryID || head.OpID < 0 || (head.OpID == 0) != (head.Sum == "") {
		return head, false, ErrSyncConflict
	}
	return head, true, nil
}

func (c *Client) rebuildLocked(ctx context.Context) (clientOverlay, error) {
	if _, err := c.catalog.RecoverTail(); err != nil {
		return clientOverlay{}, err
	}
	head, _, err := c.boundary()
	if err != nil {
		return clientOverlay{}, err
	}
	pending, err := c.queue.read(ctx)
	if err != nil {
		return clientOverlay{}, err
	}
	return rebuildClientPrefixInto(ctx, c.db, c.catalog, pending, head.OpID, head.Sum)
}

// Stage persists an already generated domain intent. Command adapters generate
// its IDs and before-state while holding the same client lock in later layers.
func (c *Client) Stage(ctx context.Context, p SyncProposal) error {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	state, err := c.queue.read(ctx)
	if err != nil {
		return err
	}
	if _, found := state.byID[p.ID]; found {
		if err := c.queue.appendLocked(ctx, "pending_enqueue", pendingEvent{Proposal: &p}); err != nil {
			return err
		}
		_, err = c.rebuildLocked(ctx)
		return err
	}
	if err := state.add(p, state.last+1); err != nil {
		return err
	}
	view, err := c.rebuildLocked(ctx)
	if err != nil {
		return err
	}
	if view.Conflict != nil {
		return fmt.Errorf("%w: resolve pending %s first", ErrSyncConflict, view.BlockedID)
	}
	localIDs := map[string]int64{}
	var last int64
	for id, ref := range view.References {
		if id > last {
			last = id
		}
		if ref.LocalID != "" {
			localIDs[ref.LocalID] = id
		}
	}
	// Confirmed local identities are also valid undo/group/dependency targets.
	for _, entry := range state.entries {
		_, id, _, found, err := store.SyncOp(c.db, entry.Proposal.ID)
		if err != nil {
			return err
		}
		if found {
			localIDs[entry.Proposal.ID] = id
		}
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := clientPendingRow(tx, p, last+1, localIDs, view.References)
	if err != nil {
		return err
	}
	vtx := store.NewViewTx(tx, row.TS)
	if err := validateWriteState(vtx, row); err != nil {
		return err
	}
	if err := checkSyncFileStates(vtx, oplog.Action(row.Action), row.Payload, p.Files, p.Undo != nil); err != nil {
		return err
	}
	handler, err := oplog.Lookup(oplog.Action(row.Action))
	if err != nil {
		return err
	}
	targets, err := handler.Targets(row.Payload)
	if err != nil {
		return err
	}
	existing := map[string]syncBlobClaim{}
	for _, key := range targets.Blobs {
		var claim syncBlobClaim
		err := tx.QueryRow(`SELECT sha256,size FROM blobs WHERE key=?`, key).Scan(&claim.digest, &claim.size)
		if err == nil {
			existing[key] = claim
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	_, err = applyOpChecked(tx, row, true)
	if err != nil {
		return err
	}
	if err := c.checkNewBlobs(ctx, tx, targets, existing); err != nil {
		return err
	}
	if err := tx.Rollback(); err != nil {
		return err
	}
	if err := c.queue.appendLocked(ctx, "pending_enqueue", pendingEvent{Proposal: &p}); err != nil {
		return err
	}
	_, err = c.rebuildLocked(ctx)
	return err
}

func (c *Client) checkNewBlobs(ctx context.Context, tx *sql.Tx, targets oplog.Targets, existing map[string]syncBlobClaim) error {
	if len(targets.Blobs) == 0 {
		return nil
	}
	head, _, err := c.boundary()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	if err := c.catalog.Iter(func(line journal.Line) error {
		if line.ID > head.OpID {
			return nil
		}
		h, err := oplog.Lookup(oplog.Action(line.Action))
		if err != nil {
			return err
		}
		t, err := h.Targets(line.Payload)
		for _, key := range t.Blobs {
			known[key] = true
		}
		return err
	}); err != nil {
		return err
	}
	var newTargets oplog.Targets
	for _, key := range targets.Blobs {
		if !known[key] {
			newTargets.Blobs = append(newTargets.Blobs, key)
		} else {
			var claim syncBlobClaim
			if err := tx.QueryRow(`SELECT sha256,size FROM blobs WHERE key=?`, key).Scan(&claim.digest, &claim.size); err != nil {
				return err
			}
			if old, found := existing[key]; !found || claim != old {
				return fmt.Errorf("%w: declared content differs from NAS blob", ErrSyncConflict)
			}
		}
	}
	return (&Library{blobs: c.blobs}).checkSyncBlobs(ctx, tx, newTargets)
}

// PutOriginal stores new local content outside the evictable cache.
func (c *Client) PutOriginal(ctx context.Context, r io.Reader) (blob.PutResult, error) {
	return c.blobs.Put(ctx, r)
}

func (c *Client) Box(ctx context.Context, id domain.BoxID) (domain.Box, bool, error) {
	return (&Library{db: c.db}).Box(ctx, id)
}

func readClientJSON(path string, value any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, value)
}

func writeClientJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".client-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncClientDir(filepath.Dir(path))
}
