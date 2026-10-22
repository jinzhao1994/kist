package library

import (
	"context"
	"errors"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

// ClientConflictDeletion compares a recorded deletion with its current scope.
// A missing Current means the target is gone; no replacement is offered.
type ClientConflictDeletion struct {
	BoxID    domain.BoxID           `json:"box_id"`
	ID       domain.ID              `json:"id"`
	Folder   bool                   `json:"folder"`
	WholeBox bool                   `json:"whole_box"`
	Before   ClientDeletionPreview  `json:"before"`
	Current  *ClientDeletionPreview `json:"current,omitempty"`
}

func (c *Client) conflictDeletion(ctx context.Context, original SyncProposal, view clientOverlay) (ClientConflictEdit, error) {
	d := &ClientConflictDeletion{}
	result := ClientConflictEdit{Fields: []ClientConflictField{}, Deletion: d}
	raw := original.Payload
	if oplog.Action(original.Action) == oplog.ActionDeleteBoxContents {
		p, err := oplog.Payload[domain.BoxContents](raw)
		if err != nil {
			return result, err
		}
		d.BoxID, d.ID, d.WholeBox = p.Box.ID, p.Box.ID.ID, true
		d.Before = ClientDeletionPreview{Name: p.Box.Name, FileCount: len(p.Files), FolderCount: max(0, len(p.Folders)-1)}
	} else {
		if oplog.Action(original.Action) == oplog.ActionDeleteFilesetEntry {
			p, err := oplog.Payload[oplog.FilesetEntryPayload](raw)
			if err != nil {
				return result, err
			}
			raw = p.Entry
		}
		p, err := oplog.Payload[domain.EntryContents](raw)
		if err != nil {
			return result, err
		}
		d.BoxID, d.ID, d.Folder = p.BoxID, p.ID, p.Folder
		d.Before = ClientDeletionPreview{Name: p.Name, FileCount: len(p.Files), FolderCount: max(0, len(p.Folders)-1)}
	}
	result.Summary = &HistorySummary{Name: d.Before.Name, Count: d.Before.FileCount}
	current, err := c.deletionLocked(ctx, view, d.BoxID, d.ID, d.Folder, d.WholeBox)
	if errors.Is(err, ErrNoSuchBox) || errors.Is(err, ErrNoSuchFile) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	d.Current = &current.Preview
	replacement := original
	replacement.Version = 2
	replacement.ID = domain.NewID().String()
	replacement.OccurredAt = time.Now().UTC()
	replacement.Action = string(current.Action)
	replacement.Payload = current.Payload
	replacement.Deletion = &current.Basis
	result.Replacement = &replacement
	return result, nil
}
