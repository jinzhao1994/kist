package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

// CreateFolder adds an empty directory. Its inverse refuses to remove the
// directory once it has contents.
func (l *Library) CreateFolder(ctx context.Context, box domain.BoxID, parent domain.FolderID, name string) (int64, error) {
	name = domain.Normalize(name)
	if err := domain.ValidateName(name); err != nil {
		return 0, err
	}
	b, found, err := l.Box(ctx, box)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, ErrNoSuchBox
	}
	folders, err := l.Folders(ctx, box)
	if err != nil {
		return 0, err
	}
	p, err := createFolderPayload(b, folders, parent, name)
	if err != nil {
		return 0, err
	}

	payload, err := oplog.Marshal(p)
	if err != nil {
		return 0, err
	}
	return l.write(ctx, oplog.ActionCreateFolder, payload, oplog.OriginUser, writeOptions{})
}

// createFolderPayload shares ownership, duplicate-name and depth checks across
// NAS and client commands. The caller supplies one coherent folder view.
func createFolderPayload(b domain.Box, folders []domain.Folder, parent domain.FolderID, name string) (oplog.FolderPayload, error) {
	name = domain.Normalize(name)
	if err := domain.ValidateName(name); err != nil {
		return oplog.FolderPayload{}, err
	}
	if parent.IsZero() {
		parent = b.RootFolderID
	}
	byID := map[domain.FolderID]domain.Folder{}
	for _, f := range folders {
		byID[f.ID] = f
		if f.ParentID != nil && *f.ParentID == parent && f.NameFold == domain.Fold(name) {
			return oplog.FolderPayload{}, fmt.Errorf("a directory with that name already exists: %w", ErrInvalidRequest)
		}
	}
	seen := map[domain.FolderID]bool{}
	ancestors := []domain.FolderID{b.RootFolderID}
	depth := 1
	for id := parent; id != b.RootFolderID; {
		f, ok := byID[id]
		if !ok || f.ParentID == nil {
			return oplog.FolderPayload{}, ErrFolderNotInBox
		}
		if seen[id] || depth >= 32 {
			return oplog.FolderPayload{}, fmt.Errorf("directory depth exceeds 32: %w", ErrInvalidRequest)
		}
		seen[id] = true
		ancestors = append(ancestors, id)
		depth++
		id = *f.ParentID
	}
	return oplog.FolderPayload{BoxID: b.ID, Folder: oplog.ImportedFolder{FolderID: domain.NewFolderID(), ParentID: parent, Name: name}, Ancestors: ancestors}, nil
}
