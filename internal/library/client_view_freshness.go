package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/journal"
)

type clientViewStamp struct {
	Version       int    `json:"version"`
	CatalogID     int64  `json:"catalog_id"`
	CatalogDigest string `json:"catalog_digest"`
	PendingID     int64  `json:"pending_id"`
	PendingDigest string `json:"pending_digest"`
}

func (c *Client) viewStampPath() string { return filepath.Join(c.root, "client-view.json") }

// journalPrefixDigest validates every record and fingerprints the entire prefix,
// not just the final record (journal checksums are not a hash chain).
func journalPrefixDigest(ctx context.Context, log *journal.Dir, through int64, sum string) (string, error) {
	hash := sha256.New()
	var last int64
	var lastSum string
	end := errors.New("prefix reached")
	err := log.Iter(func(line journal.Line) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID > through {
			return end
		}
		if line.ID != last+1 {
			return fmt.Errorf("%w: noncontiguous client prefix", journal.ErrCorrupt)
		}
		last, lastSum = line.ID, line.Sum
		hash.Write([]byte(line.Sum))
		return nil
	})
	if err != nil && !errors.Is(err, end) {
		return "", err
	}
	if last != through || lastSum != sum {
		return "", fmt.Errorf("%w: incomplete client prefix", journal.ErrCorrupt)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (c *Client) currentViewStamp(ctx context.Context, head SyncBoundary, pending pendingState) (clientViewStamp, error) {
	stamp := clientViewStamp{Version: 1, CatalogID: head.OpID, PendingID: pending.last}
	var err error
	stamp.CatalogDigest, err = journalPrefixDigest(ctx, c.catalog, head.OpID, head.Sum)
	if err != nil {
		return stamp, err
	}
	stamp.PendingDigest, err = journalPrefixDigest(ctx, c.queue.log, pending.last, pending.sum)
	return stamp, err
}

// refreshViewLocked skips SQL replay only after validating both disk journals.
// The stamp is disposable, shared across processes, and invalidated durably
// before any rebuild can change the database.
func (c *Client) refreshViewLocked(ctx context.Context) error {
	if _, err := c.catalog.RecoverTail(); err != nil {
		return err
	}
	head, _, err := c.boundary()
	if err != nil {
		return err
	}
	pending, err := c.queue.read(ctx)
	if err != nil {
		return err
	}
	current, err := c.currentViewStamp(ctx, head, pending)
	if err != nil {
		return err
	}
	var saved clientViewStamp
	if readClientJSON(c.viewStampPath(), &saved) == nil && saved == current {
		return nil
	}
	_, err = c.rebuildLocked(ctx)
	return err
}

func (c *Client) invalidateViewStamp() error {
	if err := os.Remove(c.viewStampPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncClientDir(c.root)
}
