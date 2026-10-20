package library

import (
	"context"
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func (l *Library) Filesets(ctx context.Context) ([]domain.Fileset, error) {
	var out []domain.Fileset
	err := l.readTx(ctx, func(tx *store.ViewTx) error { var err error; out, err = store.Filesets(tx.Queryer()); return err })
	return out, err
}

// SetFileset creates, edits or removes a configuration without moving any files.
func (l *Library) SetFileset(ctx context.Context, box domain.BoxID, id domain.FolderID, before, after *domain.Fileset) (int64, error) {
	if after != nil {
		if after.FolderID != id {
			return 0, ErrInvalidRequest
		}
		if err := after.Validate(); err != nil {
			return 0, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
	}
	var p oplog.FilesetPayload
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		s, ok, err := tx.EntryContents(box, id.ID, true)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoSuchFile
		}
		current, err := tx.GetFileset(id)
		if err != nil {
			return err
		}
		if !domain.EqualFileset(current, before) {
			return oplog.ErrStaleBefore
		}
		p = oplog.FilesetPayload{BoxID: box, FolderID: id, Before: before, After: after, Ancestors: append([]domain.FolderID{id}, s.Ancestors...)}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if domain.EqualFileset(before, after) {
		return 0, nil
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return 0, err
	}
	return l.write(ctx, oplog.ActionSetFileset, raw, oplog.OriginUser, writeOptions{})
}

// entryFilesetChanges records removal of roles referencing a deleted subtree.
func (l *Library) entryFilesetChanges(ctx context.Context, s domain.EntryContents) ([]oplog.FilesetPayload, error) {
	var changes []oplog.FilesetPayload
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		all, err := store.Filesets(tx.Queryer())
		if err != nil {
			return err
		}
		removedFiles := map[domain.FileID]bool{}
		removedFolders := map[domain.FolderID]bool{}
		for _, f := range s.Files {
			removedFiles[f.File.ID] = true
		}
		for _, f := range s.Folders {
			removedFolders[f.ID] = true
		}
		for _, before := range all {
			if removedFolders[before.FolderID] {
				continue
			}
			if removedFiles[before.Primary] {
				return fmt.Errorf("choose another primary file before deleting it: %w", ErrInvalidRequest)
			}
			after := before
			after.Members = nil
			for _, m := range before.Members {
				if !removedFiles[m.FileID] && !removedFolders[m.FolderID] {
					after.Members = append(after.Members, m)
				}
			}
			if domain.EqualFileset(&before, &after) {
				continue
			}
			owner, ok, err := tx.EntryContents(s.BoxID, before.FolderID.ID, true)
			if err != nil {
				return err
			}
			if !ok {
				return ErrInvalidRequest
			}
			changes = append(changes, oplog.FilesetPayload{BoxID: s.BoxID, FolderID: before.FolderID, Ancestors: append([]domain.FolderID{before.FolderID}, owner.Ancestors...), Before: &before, After: &after})
		}
		return nil
	})
	return changes, err
}
