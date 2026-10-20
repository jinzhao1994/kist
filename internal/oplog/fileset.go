package oplog

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
)

// FilesetPayload is a complete, optimistic transition of one folder's configuration.
type FilesetPayload struct {
	BoxID     domain.BoxID      `json:"box_id"`
	FolderID  domain.FolderID   `json:"folder_id"`
	Ancestors []domain.FolderID `json:"ancestors"`
	Before    *domain.Fileset   `json:"before,omitempty"`
	After     *domain.Fileset   `json:"after,omitempty"`
}
type filesetOp struct{}

func init() { Register(ActionSetFileset, filesetOp{}) }
func (filesetOp) Apply(tx Tx, raw []byte) error {
	p, err := Payload[FilesetPayload](raw)
	if err != nil {
		return err
	}
	current, err := tx.GetFileset(p.FolderID)
	if err != nil {
		return err
	}
	if !domain.EqualFileset(current, p.Before) {
		return fmt.Errorf("fileset changed: %w", ErrStaleBefore)
	}
	f, ok, err := tx.GetFolder(p.FolderID)
	if err != nil {
		return err
	}
	if !ok || f.ParentID == nil {
		return fmt.Errorf("fileset requires a non-root folder: %w", ErrStaleBefore)
	}
	at := p.FolderID
	for i, id := range p.Ancestors {
		if id != at {
			return ErrStaleBefore
		}
		parent, ok, err := tx.GetFolder(id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrStaleBefore
		}
		if i == len(p.Ancestors)-1 {
			b, ok, err := tx.GetBox(p.BoxID)
			if err != nil {
				return err
			}
			if !ok || b.RootFolderID != id || parent.ParentID != nil {
				return ErrStaleBefore
			}
		} else {
			if parent.ParentID == nil {
				return ErrStaleBefore
			}
			at = *parent.ParentID
		}
	}
	if len(p.Ancestors) == 0 {
		return ErrStaleBefore
	}
	if p.After != nil {
		s := p.After
		if s.FolderID != p.FolderID {
			return ErrStaleBefore
		}
		if err = s.Validate(); err != nil {
			return err
		}
		for _, m := range s.Members {
			if !m.FileID.IsZero() {
				file, ok, err := tx.GetFile(m.FileID)
				if err != nil {
					return err
				}
				if !ok || file.FolderID != p.FolderID {
					return fmt.Errorf("member is not a direct child: %w", ErrStaleBefore)
				}
			} else {
				child, ok, err := tx.GetFolder(m.FolderID)
				if err != nil {
					return err
				}
				if !ok || child.ParentID == nil || *child.ParentID != p.FolderID {
					return ErrStaleBefore
				}
				nested, err := tx.GetFileset(m.FolderID)
				if err != nil {
					return err
				}
				if nested == nil {
					return fmt.Errorf("page folder is not a fileset: %w", ErrStaleBefore)
				}
			}
		}
		if !s.Primary.IsZero() {
			file, ok, err := tx.GetFile(s.Primary)
			if err != nil {
				return err
			}
			if !ok {
				return ErrStaleBefore
			}
			at := file.FolderID
			seen := map[domain.FolderID]bool{}
			for at != p.FolderID {
				if seen[at] {
					return ErrStaleBefore
				}
				seen[at] = true
				parent, ok, err := tx.GetFolder(at)
				if err != nil {
					return err
				}
				if !ok || parent.ParentID == nil {
					return fmt.Errorf("primary is outside fileset: %w", ErrStaleBefore)
				}
				at = *parent.ParentID
			}
		}
	}
	return tx.SetFileset(p.FolderID, p.After)
}
func (filesetOp) Inverse(raw []byte) (Action, []byte, error) {
	p, err := Payload[FilesetPayload](raw)
	if err != nil {
		return "", nil, err
	}
	p.Before, p.After = p.After, p.Before
	b, err := Marshal(p)
	return ActionSetFileset, b, err
}
func (filesetOp) Targets(raw []byte) (Targets, error) {
	p, err := Payload[FilesetPayload](raw)
	t := Targets{Boxes: []domain.BoxID{p.BoxID}, Filesets: []domain.FolderID{p.FolderID}, FolderAncestors: p.Ancestors}
	for _, s := range []*domain.Fileset{p.Before, p.After} {
		if s == nil {
			continue
		}
		if !s.Primary.IsZero() {
			t.Files = append(t.Files, s.Primary)
		}
		for _, m := range s.Members {
			if !m.FileID.IsZero() {
				t.Files = append(t.Files, m.FileID)
			}
		}
	}
	return t, err
}
