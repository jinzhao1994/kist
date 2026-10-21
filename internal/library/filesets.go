package library

import (
	"context"
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

func (l *Library) Filesets(ctx context.Context) ([]domain.Fileset, error) {
	var out []domain.Fileset
	err := l.readTx(ctx, func(tx *store.ViewTx) error { var err error; out, err = store.Filesets(tx.Queryer()); return err })
	return out, err
}

// SetFileset creates, edits or removes a configuration without moving any files.
func (l *Library) SetFileset(ctx context.Context, box domain.BoxID, id domain.FolderID, before, after *domain.Fileset) (int64, error) {
	var p oplog.FilesetPayload
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		p, err = filesetCommandPayload(tx, box, id, before, after)
		return err
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
		var err error
		changes, err = entryFilesetChanges(tx, s)
		return err
	})
	return changes, err
}

func entryFilesetChanges(tx *store.ViewTx, s domain.EntryContents) ([]oplog.FilesetPayload, error) {
	var changes []oplog.FilesetPayload
	all, err := store.Filesets(tx.Queryer())
	if err != nil {
		return nil, err
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
			return nil, fmt.Errorf("choose another primary file before deleting it: %w", ErrInvalidRequest)
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
			return nil, err
		}
		if !ok {
			return nil, ErrInvalidRequest
		}
		changes = append(changes, oplog.FilesetPayload{BoxID: s.BoxID, FolderID: before.FolderID, Ancestors: append([]domain.FolderID{before.FolderID}, owner.Ancestors...), Before: &before, After: &after})
	}
	return changes, nil
}

// BrowseEntry carries the stable presentation identity separately from the primary.
type BrowseEntry = store.BrowseEntry

func (l *Library) Browse(ctx context.Context, q query.Query, include bool) ([]BrowseEntry, string, error) {
	var out []BrowseEntry
	var next string
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		out, next, err = store.ListEntries(tx.Queryer(), q, include, l.zone())
		return err
	})
	return out, next, err
}

// FilesetDetail resolves a combination and its location in one read transaction.
func (l *Library) FilesetDetail(ctx context.Context, id domain.FolderID) (domain.Fileset, domain.Folder, domain.BoxID, error) {
	var set domain.Fileset
	var folder domain.Folder
	var box domain.BoxID
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		current, err := tx.GetFileset(id)
		if err != nil {
			return err
		}
		if current == nil {
			return ErrNoSuchFile
		}
		set = *current
		var found bool
		folder, found, err = store.FolderForDisplay(tx.Queryer(), id)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchFile
		}
		box, _, err = store.FolderChain(tx.Queryer(), id)
		return err
	})
	return set, folder, box, err
}
