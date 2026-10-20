package oplog

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"reflect"
	"time"
)

func init() {
	Register(ActionDeleteBoxContents, boxContents{deleting: true})
	Register(ActionRestoreBoxContents, boxContents{})
}

type boxContents struct{ deleting bool }

func (h boxContents) Apply(tx Tx, raw []byte) error {
	s, err := Payload[domain.BoxContents](raw)
	if err != nil {
		return err
	}
	if !h.deleting {
		return tx.RestoreBoxContents(s)
	}
	current, found, err := tx.BoxContents(s.Box.ID)
	if err != nil {
		return err
	}
	// updated_at on boxes and folders is propagated by the write path. It
	// cannot block undoing a restore; all semantic content must still match.
	normalizeContents(&current)
	normalizeContents(&s)
	if !found || !reflect.DeepEqual(current, s) {
		return fmt.Errorf("box contents changed: %w", ErrStaleBefore)
	}
	return tx.DeleteBoxContents(s)
}

func normalizeContents(s *domain.BoxContents) {
	s.Box.UpdatedAt = time.Time{}
	for i := range s.Folders {
		s.Folders[i].UpdatedAt = time.Time{}
	}
}

func (h boxContents) Inverse(raw []byte) (Action, []byte, error) {
	if _, err := Payload[domain.BoxContents](raw); err != nil {
		return "", nil, err
	}
	if h.deleting {
		return ActionRestoreBoxContents, raw, nil
	}
	return ActionDeleteBoxContents, raw, nil
}

func (boxContents) Targets(raw []byte) (Targets, error) {
	s, err := Payload[domain.BoxContents](raw)
	if err != nil {
		return Targets{}, err
	}
	t := Targets{Boxes: []domain.BoxID{s.Box.ID}}
	for _, set := range s.Filesets {
		t.Filesets = append(t.Filesets, set.FolderID)
	}
	for _, f := range s.Files {
		t.Files = append(t.Files, f.File.ID)
		t.Blobs = append(t.Blobs, f.File.BlobKey)
	}
	return t, nil
}
