package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// HistorySummary is display-only context for one event, never a search index.
// Names embedded in an event take precedence over mutable current names.
type HistorySummary struct {
	Name         string `json:"name"`
	Count        int    `json:"count"`
	Tag          string `json:"tag,omitempty"`
	Before       string `json:"before,omitempty"`
	After        string `json:"after,omitempty"`
	Context      string `json:"context,omitempty"`
	CurrentNames bool   `json:"current_names"`
}

// HistorySummary reads a single requested event. Header pagination stays payload-free.
func (l *Library) HistorySummary(ctx context.Context, id int64) (HistorySummary, error) {
	tx, err := l.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return HistorySummary{}, err
	}
	defer tx.Rollback()
	row, found, err := store.GetOp(tx, id)
	if err != nil {
		return HistorySummary{}, err
	}
	if !found {
		return HistorySummary{}, ErrNoSuchOp
	}
	return summarizeHistory(store.NewViewTx(tx, time.Time{}), oplog.Action(row.Action), row.Payload)
}

func summarizeHistory(view *store.ViewTx, action oplog.Action, raw []byte) (HistorySummary, error) {
	var s HistorySummary
	// Decode only the small fields used for presentation; file bodies remain raw
	// until their first representative is read. Never serialize metadata to the UI.
	var p struct {
		Name         json.RawMessage   `json:"name"`
		Before       json.RawMessage   `json:"before"`
		After        json.RawMessage   `json:"after"`
		BoxID        domain.BoxID      `json:"box_id"`
		TagID        domain.TagID      `json:"tag_id"`
		BoxIDs       []domain.BoxID    `json:"box_ids"`
		FileIDs      []domain.FileID   `json:"file_ids"`
		Files        []json.RawMessage `json:"files"`
		Box          domain.Box        `json:"box"`
		Folder       json.RawMessage   `json:"folder"`
		Import       json.RawMessage   `json:"import"`
		Replacements []json.RawMessage `json:"replacements"`
		Extractor    string            `json:"extractor"`
		NewParent    *domain.TagID     `json:"new_parent"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return s, err
	}
	boxName := func(id domain.BoxID) (string, error) {
		b, ok, err := view.GetBox(id)
		s.CurrentNames = true
		if !ok {
			return id.String(), err
		}
		return b.Name, err
	}
	fileName := func(id domain.FileID) (string, error) {
		f, ok, err := view.GetFile(id)
		s.CurrentNames = true
		if !ok {
			return id.String(), err
		}
		return f.Name, err
	}
	tagName := func(id domain.TagID) (string, error) {
		t, ok, err := view.GetTag(id)
		s.CurrentNames = true
		if !ok {
			return id.String(), err
		}
		return t.Name, err
	}
	var err error
	s.Count = 1
	switch action {
	case oplog.ActionDeleteFilesetEntry, oplog.ActionRestoreFilesetEntry:
		payload, e := oplog.Payload[oplog.FilesetEntryPayload](raw)
		if e != nil {
			return s, e
		}
		return summarizeHistory(view, oplog.ActionDeleteEntryContents, payload.Entry)
	case oplog.ActionSetFileset:
		payload, e := oplog.Payload[oplog.FilesetPayload](raw)
		if e != nil {
			return s, e
		}
		f, ok, e := view.GetFolder(payload.FolderID)
		if e != nil {
			return s, e
		}
		s.Name = payload.FolderID.String()
		if ok {
			s.Name = f.Name
		}
		s.CurrentNames = true

	case oplog.ActionCreateBox, oplog.ActionDeleteBox, oplog.ActionCreateTag, oplog.ActionDeleteTag, oplog.ActionDeleteEntryContents, oplog.ActionRestoreEntryContents:
		err = json.Unmarshal(p.Name, &s.Name)
	case oplog.ActionDeleteBoxContents, oplog.ActionRestoreBoxContents:
		s.Name = p.Box.Name
	case oplog.ActionRenameEntry, oplog.ActionRenameTag:
		if err = json.Unmarshal(p.Before, &s.Before); err != nil {
			break
		}
		err = json.Unmarshal(p.After, &s.After)
		s.Name = s.Before
	case oplog.ActionUpdateBox:
		if len(p.Name) > 0 {
			var change oplog.TextChange
			err = json.Unmarshal(p.Name, &change)
			s.Before = change.Before
			s.After = change.After
			s.Name = change.Before
		} else {
			s.Name, err = boxName(p.BoxID)
		}
	case oplog.ActionCreateFolder, oplog.ActionDeleteFolder:
		var folder oplog.ImportedFolder
		err = json.Unmarshal(p.Folder, &folder)
		s.Name = folder.Name
	case oplog.ActionCreateSavedSearch, oplog.ActionUpdateSavedSearch, oplog.ActionDeleteSavedSearch:
		var saved oplog.SavedSearchPayload
		err = json.Unmarshal(raw, &saved)
		if saved.After != nil {
			s.Name = saved.After.Name
		} else if saved.Before != nil {
			s.Name = saved.Before.Name
		}
		if saved.Before != nil && saved.After != nil && saved.Before.Name != saved.After.Name {
			s.Before = saved.Before.Name
			s.After = saved.After.Name
		}
	case oplog.ActionTagBoxes, oplog.ActionUntagBoxes:
		s.Count = len(p.BoxIDs)
		if s.Count > 0 {
			s.Name, err = boxName(p.BoxIDs[0])
		}
		if err == nil {
			s.Tag, err = tagName(p.TagID)
		}
	case oplog.ActionTagFiles, oplog.ActionUntagFiles:
		s.Count = len(p.FileIDs)
		if s.Count > 0 {
			s.Name, err = fileName(p.FileIDs[0])
		}
		if err == nil {
			s.Tag, err = tagName(p.TagID)
		}
	case oplog.ActionMoveTag:
		s.Name, err = tagName(p.TagID)
		if err == nil && p.NewParent != nil {
			s.After, err = tagName(*p.NewParent)
		}
	case oplog.ActionUploadFiles, oplog.ActionUndoUploadFiles:
		var imported HistorySummary
		imported, err = summarizeHistory(view, oplog.ActionImportFiles, p.Import)
		s = imported
		s.Count += len(p.Replacements)
		if err == nil && s.Name == "" && len(p.Replacements) > 0 {
			var first struct {
				After struct {
					Name string `json:"name"`
				} `json:"after"`
			}
			err = json.Unmarshal(p.Replacements[0], &first)
			s.Name = first.After.Name
		}
	case oplog.ActionImportFiles, oplog.ActionDeleteFiles, oplog.ActionExtractMetadata, oplog.ActionSetMetadata:
		s.Count = len(p.Files)
		s.Context = p.Extractor
		if s.Count > 0 {
			var first struct {
				Name   string        `json:"name"`
				FileID domain.FileID `json:"file_id"`
				File   domain.File   `json:"file"`
			}
			if err = json.Unmarshal(p.Files[0], &first); err != nil {
				break
			}
			s.Name = first.Name
			if s.Name == "" {
				s.Name = first.File.Name
			}
			if s.Name == "" {
				s.Name, err = fileName(first.FileID)
			}
		}
	case oplog.ActionRequestReceipt:
		s.Count = 0
	}
	return s, err
}
