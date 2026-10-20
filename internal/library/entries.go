package library

import (
	"context"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// EntryDeletionPreview identifies the exact scope the user is confirming.
type EntryDeletionPreview struct {
	Name        string `json:"name"`
	FileCount   int    `json:"file_count"`
	FolderCount int    `json:"folder_count"`
	Revision    int64  `json:"revision"`
}

func (l *Library) entrySnapshot(ctx context.Context, box domain.BoxID, id domain.ID, folder bool) (domain.EntryContents, int64, error) {
	var s domain.EntryContents
	var rev int64
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var found bool
		var err error
		s, found, err = tx.EntryContents(box, id, folder)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchFile
		}
		rev, _, err = store.LastOpTouchingBox(tx.Queryer(), box, 0)
		return err
	})
	return s, rev, err
}
func (l *Library) PreviewEntryDeletion(ctx context.Context, box domain.BoxID, id domain.ID, folder bool) (EntryDeletionPreview, error) {
	s, rev, err := l.entrySnapshot(ctx, box, id, folder)
	count := len(s.Folders)
	if folder && count > 0 {
		count--
	}
	return EntryDeletionPreview{Name: s.Name, FileCount: len(s.Files), FolderCount: count, Revision: rev}, err
}
func (l *Library) DeleteEntry(ctx context.Context, box domain.BoxID, id domain.ID, folder bool, revision int64) (int64, error) {
	s, rev, err := l.entrySnapshot(ctx, box, id, folder)
	if err != nil {
		return 0, err
	}
	if rev != revision {
		return 0, oplog.ErrStaleBefore
	}
	raw, err := oplog.Marshal(s)
	if err != nil {
		return 0, err
	}
	changes, err := l.entryFilesetChanges(ctx, s)
	if err != nil {
		return 0, err
	}
	if len(changes) > 0 {
		payload, err := oplog.Marshal(oplog.FilesetEntryPayload{Entry: raw, Changes: changes})
		if err != nil {
			return 0, err
		}
		return l.write(ctx, oplog.ActionDeleteFilesetEntry, payload, oplog.OriginUser, writeOptions{})
	}
	return l.write(ctx, oplog.ActionDeleteEntryContents, raw, oplog.OriginUser, writeOptions{})
}
func (l *Library) RenameEntry(ctx context.Context, box domain.BoxID, id domain.ID, folder bool, before, name string) (int64, error) {
	name = domain.Normalize(name)
	if err := domain.ValidateName(name); err != nil {
		return 0, err
	}
	s, _, err := l.entrySnapshot(ctx, box, id, folder)
	if err != nil {
		return 0, err
	}
	if s.Name != before {
		return 0, oplog.ErrStaleBefore
	}
	ancestors := s.Ancestors
	if folder {
		ancestors = append(ancestors, domain.FolderID{ID: id})
	}
	raw, err := oplog.Marshal(oplog.RenameEntryPayload{BoxID: box, ID: id, Folder: folder, Parent: s.Parent, Before: before, After: name, Ancestors: ancestors})
	if err != nil {
		return 0, err
	}
	return l.write(ctx, oplog.ActionRenameEntry, raw, oplog.OriginUser, writeOptions{})
}
