package library

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

var ErrUploadTargetChanged = errors.New("upload target changed; select the destination again")

// UploadTarget identifies the box's own fields and the destination's ancestor
// chain in one read snapshot. Descendant changes and propagated UpdatedAt do
// not invalidate a long upload. Recheck inside ExecuteRequest before Commit.
func (l *Library) UploadTarget(ctx context.Context, box domain.BoxID, folder domain.FolderID) (string, error) {
	type ancestor struct {
		ID     domain.FolderID
		Parent *domain.FolderID
		Name   string
	}
	var target struct {
		Box               domain.BoxID
		Root              domain.FolderID
		Name, Description string
		Start, End        domain.LocalDate
		Ancestors         []ancestor
	}
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		b, found, err := tx.GetBox(box)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchBox
		}
		target.Box, target.Root, target.Name, target.Description = b.ID, b.RootFolderID, b.Name, b.Description
		target.Start, target.End = b.StartAt, b.EndAt
		seen := map[domain.FolderID]bool{}
		for id := folder; ; {
			if seen[id] || len(seen) >= 32 {
				return ErrFolderNotInBox
			}
			seen[id] = true
			f, found, err := tx.GetFolder(id)
			if err != nil {
				return err
			}
			if !found {
				return ErrFolderNotInBox
			}
			target.Ancestors = append(target.Ancestors, ancestor{f.ID, f.ParentID, f.Name})
			if id == b.RootFolderID {
				if f.ParentID != nil {
					return ErrInvariantViolated
				}
				return nil
			}
			if f.ParentID == nil {
				return ErrFolderNotInBox
			}
			id = *f.ParentID
		}
	})
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(target)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}
