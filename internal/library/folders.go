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
	if parent.IsZero() {
		parent = b.RootFolderID
	}
	folders, err := l.Folders(ctx, box)
	if err != nil {
		return 0, err
	}
	byID := map[domain.FolderID]domain.Folder{}
	for _, f := range folders {
		byID[f.ID] = f
		if f.ParentID != nil && *f.ParentID == parent && f.NameFold == domain.Fold(name) {
			return 0, fmt.Errorf("a directory with that name already exists: %w", ErrInvalidRequest)
		}
	}
	seen := map[domain.FolderID]bool{}
	ancestors := []domain.FolderID{b.RootFolderID}
	depth := 1
	for id := parent; id != b.RootFolderID; {
		f, ok := byID[id]
		if !ok || f.ParentID == nil {
			return 0, ErrFolderNotInBox
		}
		if seen[id] || depth >= 32 {
			return 0, fmt.Errorf("directory depth exceeds 32: %w", ErrInvalidRequest)
		}
		seen[id] = true
		ancestors = append(ancestors, id)
		depth++
		id = *f.ParentID
	}
	payload, err := oplog.Marshal(oplog.FolderPayload{BoxID: box, Folder: oplog.ImportedFolder{FolderID: domain.NewFolderID(), ParentID: parent, Name: name}, Ancestors: ancestors})
	if err != nil {
		return 0, err
	}
	return l.write(ctx, oplog.ActionCreateFolder, payload, oplog.OriginUser, writeOptions{})
}
