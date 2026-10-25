package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// CatalogRevision is an opaque equality token, not a timestamp or sortable ID.
// It is derived from durable journals and does not contact the upstream.
func (l *Library) CatalogRevision(ctx context.Context) (string, error) {
	head, err := l.SyncHead(ctx)
	if err != nil {
		return "", err
	}
	return revisionToken(head), nil
}

func (c *Client) CatalogRevision(ctx context.Context) (string, error) {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	head, complete, err := c.boundary()
	if err != nil {
		return "", err
	}
	line, _, err := c.queue.log.Last()
	if err != nil {
		return "", err
	}
	return revisionToken(struct {
		Head       SyncBoundary
		Complete   bool
		PendingID  int64
		PendingSum string
	}{head, complete, line.ID, line.Sum}), nil
}

func revisionToken(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
