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
	Key      string   `json:"key" enum:"name,description,start_at,end_at"`
	Path     []string `json:"path"`
	Before   string   `json:"before"`
	Current  string   `json:"current"`
	Proposed string   `json:"proposed"`
}

type ClientConflictFileset struct {
	BoxID    domain.BoxID    `json:"box_id"`
	FolderID domain.FolderID `json:"folder_id"`
	Name     string          `json:"name"`
	Before   *domain.Fileset `json:"before,omitempty"`
	Current  *domain.Fileset `json:"current,omitempty"`
	Proposed *domain.Fileset `json:"proposed,omitempty"`
}

type ClientConflictCoverFile struct {
	ID        domain.FileID `json:"id"`
	Name      string        `json:"name"`
	Available bool          `json:"available"`
}

type ClientConflictCover struct {
	Before   ClientConflictCoverFile `json:"before"`
	Current  ClientConflictCoverFile `json:"current"`
	Proposed ClientConflictCoverFile `json:"proposed"`
}

type ClientConflictEdit struct {
	Deletion    *ClientConflictDeletion `json:"deletion,omitempty"`
	Cover       *ClientConflictCover    `json:"cover,omitempty"`
	Fileset     *ClientConflictFileset  `json:"fileset,omitempty"`
	Summary     *HistorySummary         `json:"summary,omitempty"`
	Replacement *SyncProposal           `json:"replacement,omitempty"`
	Fields      []ClientConflictField   `json:"fields"`
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
		switch oplog.Action(original.Action) {
		case oplog.ActionDeleteBoxContents, oplog.ActionDeleteEntryContents, oplog.ActionDeleteFilesetEntry:
			var err error
			result, err = c.conflictDeletion(ctx, original, view)
			return err
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
		case oplog.ActionSetFileset:
			p, err := oplog.Payload[oplog.FilesetPayload](original.Payload)
			if err != nil {
				return err
			}
			folder, found, err := vtx.GetFolder(p.FolderID)
			if err != nil || !found {
				return err
			}
			current, err := vtx.GetFileset(p.FolderID)
			if err != nil {
				return err
			}
			updated, err := filesetCommandPayload(vtx, p.BoxID, p.FolderID, current, p.After)
			if err != nil {
				return err
			}
			result.Fileset = &ClientConflictFileset{BoxID: p.BoxID, FolderID: p.FolderID, Name: folder.Name, Before: p.Before, Current: current, Proposed: p.After}
			payload = updated
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
			current, found, err := vtx.GetBox(p.BoxID)
			if err != nil || !found {
				return err
			}
			if p.CoverFileID != nil {
				describe := func(id domain.FileID) (ClientConflictCoverFile, error) {
					value := ClientConflictCoverFile{ID: id, Available: id.IsZero()}
					if id.IsZero() {
						return value, nil
					}
					file, found, err := vtx.GetFile(id)
					if err != nil || !found {
						return value, err
					}
					owner, _, err := store.FolderChain(vtx.Queryer(), file.FolderID)
					if err != nil {
						return value, err
					}
					value.Name, value.Available = file.Name, owner == p.BoxID
					return value, nil
				}
				cover := &ClientConflictCover{}
				if cover.Before, err = describe(p.CoverFileID.Before); err != nil {
					return err
				}
				if cover.Current, err = describe(current.CoverFileID); err != nil {
					return err
				}
				if cover.Proposed, err = describe(p.CoverFileID.After); err != nil {
					return err
				}
				result.Cover = cover
				p.CoverFileID.Before = current.CoverFileID
			}
			if p.Name != nil {
				field("name", []string{"name", "after"}, p.Name.Before, current.Name, p.Name.After)
				p.Name.Before = current.Name
			}
			if p.Description != nil {
				field("description", []string{"description", "after"}, p.Description.Before, current.Description, p.Description.After)
				p.Description.Before = current.Description
			}
			if p.StartAt != nil {
				field("start_at", []string{"start_at", "after"}, p.StartAt.Before.String(), current.StartAt.String(), p.StartAt.After.String())
				p.StartAt.Before = current.StartAt
			}
			if p.EndAt != nil {
				field("end_at", []string{"end_at", "after"}, p.EndAt.Before.String(), current.EndAt.String(), p.EndAt.After.String())
				p.EndAt.Before = current.EndAt
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
