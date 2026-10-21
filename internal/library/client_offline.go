package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

// ClientOfflineSelection records logical targets rather than a frozen list of
// content keys. Callers must resolve it again after catalog or local edits.
type ClientOfflineSelection struct {
	Files    []domain.FileID   `json:"files"`
	Filesets []domain.FolderID `json:"filesets"`
}

type ClientOfflineOriginal struct {
	Key   string          `json:"key"`
	Size  int64           `json:"size"`
	Files []domain.FileID `json:"files"`
}

// OfflineOriginals resolves a selection against one coherent local view. It
// includes every configured member and explicit primary, but not unrelated
// files merely sharing a directory. This is a dependency list, not a claim
// that any bytes have been downloaded or that the catalog is current.
func (c *Client) OfflineOriginals(ctx context.Context, selection ClientOfflineSelection) ([]ClientOfflineOriginal, error) {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if _, err := c.rebuildLocked(ctx); err != nil {
		return nil, err
	}
	sets, err := store.Filesets(c.db)
	if err != nil {
		return nil, err
	}
	files, err := offlineSelectionFiles(selection, sets)
	if err != nil {
		return nil, err
	}
	out := []ClientOfflineOriginal{}
	positions := map[string]int{}
	for _, id := range files {
		var key string
		var size int64
		err := c.db.QueryRowContext(ctx, `SELECT f.blob_key,b.size FROM files f JOIN blobs b ON b.key=f.blob_key WHERE f.id=?`, id).Scan(&key, &size)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("offline file %s: %w", id, ErrInvalidRequest)
		}
		if err != nil {
			return nil, err
		}
		if _, err := blob.ParseKey(key); err != nil || size < 0 {
			return nil, ErrInvariantViolated
		}
		if position, ok := positions[key]; ok {
			out[position].Files = append(out[position].Files, id)
		} else {
			positions[key] = len(out)
			out = append(out, ClientOfflineOriginal{Key: key, Size: size, Files: []domain.FileID{id}})
		}
	}
	return out, nil
}

func offlineSelectionFiles(selection ClientOfflineSelection, sets []domain.Fileset) ([]domain.FileID, error) {
	byID := map[domain.FolderID]domain.Fileset{}
	for _, set := range sets {
		byID[set.FolderID] = set
	}
	files := []domain.FileID{}
	seen := map[domain.FileID]bool{}
	add := func(id domain.FileID) error {
		if id.IsZero() {
			return ErrInvalidRequest
		}
		if !seen[id] {
			seen[id] = true
			files = append(files, id)
		}
		return nil
	}
	for _, id := range selection.Files {
		if err := add(id); err != nil {
			return nil, err
		}
	}
	// An explicit stack bounds call-stack use even for damaged deep nesting.
	type visit struct {
		id   domain.FolderID
		exit bool
	}
	state := map[domain.FolderID]uint8{}
	for _, root := range selection.Filesets {
		stack := []visit{{id: root}}
		for len(stack) > 0 {
			step := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if step.exit {
				state[step.id] = 2
				continue
			}
			if state[step.id] == 2 {
				continue
			}
			if state[step.id] == 1 {
				return nil, fmt.Errorf("cyclic offline fileset: %w", ErrInvariantViolated)
			}
			set, ok := byID[step.id]
			if !ok {
				return nil, fmt.Errorf("offline fileset %s: %w", step.id, ErrInvalidRequest)
			}
			if err := set.Validate(); err != nil {
				return nil, ErrInvariantViolated
			}
			state[step.id] = 1
			stack = append(stack, visit{id: step.id, exit: true})
			if !set.Primary.IsZero() {
				if err := add(set.Primary); err != nil {
					return nil, err
				}
			}
			for _, m := range set.Members {
				if !m.FileID.IsZero() {
					if err := add(m.FileID); err != nil {
						return nil, err
					}
				}
			}
			for i := len(set.Members) - 1; i >= 0; i-- {
				m := set.Members[i]
				if !m.FolderID.IsZero() {
					stack = append(stack, visit{id: m.FolderID})
				}
			}
		}
	}
	return files, nil
}
