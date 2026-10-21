package library

import (
	"context"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// ClientConflictField describes an editable value without exposing a JSON editor.
// Path points to the intended value in the replacement payload.
type ClientConflictField struct {
	Key      string   `json:"key" enum:"name,description"`
	Path     []string `json:"path"`
	Before   string   `json:"before"`
	Current  string   `json:"current"`
	Proposed string   `json:"proposed"`
}

type ClientConflictEdit struct {
	Summary     *HistorySummary       `json:"summary,omitempty"`
	Replacement *SyncProposal         `json:"replacement,omitempty"`
	Fields      []ClientConflictField `json:"fields"`
}

// ConflictEdit builds a replacement against the valid prefix before the blocked
// intent. It does not resolve anything. Resolve revalidates the exact candidate,
// so a later catalog update cannot silently change the user's reviewed decision.
func (c *Client) ConflictEdit(ctx context.Context, id string) (ClientConflictEdit, error) {
	result := ClientConflictEdit{Fields: []ClientConflictField{}}
	err := c.withLocalHistory(ctx, func(state pendingState, view clientOverlay) error {
		index, found := state.byID[id]
		if !found {
			return ErrNoSuchOp
		}
		entry := state.entries[index]
		if !entry.active() || entry.Sending || view.BlockedID != id {
			return ErrSyncConflict
		}
		original := entry.Proposal
		if original.Undo != nil {
			return nil
		}
		tx, err := c.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		vtx := store.NewViewTx(tx, time.Time{})
		summary, err := summarizeHistory(vtx, oplog.Action(original.Action), original.Payload)
		if err != nil {
			return err
		}
		result.Summary = &summary
		field := func(key string, path []string, before, current, proposed string) {
			result.Fields = append(result.Fields, ClientConflictField{key, path, before, current, proposed})
		}
		var payload any
		switch oplog.Action(original.Action) {
		case oplog.ActionRenameEntry:
			p, err := oplog.Payload[oplog.RenameEntryPayload](original.Payload)
			if err != nil {
				return err
			}
			current, found, err := vtx.EntryContents(p.BoxID, p.ID, p.Folder)
			if err != nil || !found {
				return err
			}
			updated, err := renameEntryPayload(current, current.Name, p.After)
			if err != nil {
				return err
			}
			field("name", []string{"after"}, p.Before, current.Name, p.After)
			payload = updated
		case oplog.ActionRenameTag:
			p, err := oplog.Payload[oplog.RenameTagPayload](original.Payload)
			if err != nil {
				return err
			}
			current, found, err := vtx.GetTag(p.TagID)
			if err != nil || !found {
				return err
			}
			field("name", []string{"after"}, p.Before, current.Name, p.After)
			p.Before = current.Name
			payload = p
		case oplog.ActionUpdateBox:
			p, err := oplog.Payload[oplog.UpdateBoxPayload](original.Payload)
			if err != nil {
				return err
			}
			// Dates and cover references require their corresponding domain pickers.
			if p.StartAt != nil || p.EndAt != nil || p.CoverFileID != nil {
				return nil
			}
			current, found, err := vtx.GetBox(p.BoxID)
			if err != nil || !found {
				return err
			}
			if p.Name != nil {
				field("name", []string{"name", "after"}, p.Name.Before, current.Name, p.Name.After)
				p.Name.Before = current.Name
			}
			if p.Description != nil {
				field("description", []string{"description", "after"}, p.Description.Before, current.Description, p.Description.After)
				p.Description.Before = current.Description
			}
			payload = p
		default:
			return nil
		}
		raw, err := oplog.Marshal(payload)
		if err != nil {
			return err
		}
		replacement := original
		replacement.ID = domain.NewID().String()
		replacement.OccurredAt = time.Now().UTC()
		replacement.Payload = raw
		result.Replacement = &replacement
		return nil
	})
	return result, err
}
