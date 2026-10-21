package library

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// Both NAS and offline commands validate the same current configuration before
// recording the full before/after pair and its ownership ancestors.
func filesetCommandPayload(tx *store.ViewTx, box domain.BoxID, id domain.FolderID, before, after *domain.Fileset) (oplog.FilesetPayload, error) {
	var p oplog.FilesetPayload
	if after != nil {
		if after.FolderID != id {
			return p, ErrInvalidRequest
		}
		if err := after.Validate(); err != nil {
			return p, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
	}
	contents, found, err := tx.EntryContents(box, id.ID, true)
	if err != nil {
		return p, err
	}
	if !found {
		return p, ErrNoSuchFile
	}
	current, err := tx.GetFileset(id)
	if err != nil {
		return p, err
	}
	if !domain.EqualFileset(current, before) {
		return p, oplog.ErrStaleBefore
	}
	return oplog.FilesetPayload{BoxID: box, FolderID: id, Before: before, After: after, Ancestors: append([]domain.FolderID{id}, contents.Ancestors...)}, nil
}
