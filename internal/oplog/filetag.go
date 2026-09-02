package oplog

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

func init() {
	Register(ActionTagFiles, tagFiles{})
	Register(ActionUntagFiles, untagFiles{})
}

// TagFilesPayload attaches one tag to many files in a single op — the same
// batch granularity as TagBoxesPayload, and for the same reason: tagging 200
// invoices must be one undo, not 200 (spec 1.6.2).
//
// FileIDs names only the files that actually change. library filters out any
// file that already carries the tag before writing the op, or the inverse
// would strip a tag this op never added.
type TagFilesPayload struct {
	TagID   domain.TagID    `json:"tag_id"`
	FileIDs []domain.FileID `json:"file_ids"`
	// BoxIDs are the boxes those files were in, so the op lands in op_boxes
	// and `kist log --box` can find it. Tagging across boxes is the first
	// reason that table is many-to-many (spec 2.2.1).
	BoxIDs []domain.BoxID `json:"box_ids"`
	// Source is the extractor's id, or extract.UserSource for a person —
	// the same values file_metadata.source takes (spec 1.6.1 ①). It lives in
	// the payload rather than being a constant inside Apply because a rebuild
	// has to reproduce it.
	Source string `json:"source"`
}

// UntagFilesPayload is the same shape, so the two are exact inverses.
type UntagFilesPayload = TagFilesPayload

type tagFiles struct{}

func (tagFiles) Apply(tx Tx, payload []byte) error {
	p, err := Payload[TagFilesPayload](payload)
	if err != nil {
		return err
	}
	if len(p.FileIDs) == 0 {
		return fmt.Errorf("tag_files: %w", errEmptyBatch)
	}
	if err := checkFilesExist(tx, p.FileIDs); err != nil {
		return err
	}
	for _, id := range p.FileIDs {
		if err := tx.AddFileTag(id, p.TagID, p.Source); err != nil {
			return err
		}
	}
	return nil
}

func (tagFiles) Inverse(payload []byte) (Action, []byte, error) {
	return ActionUntagFiles, payload, nil
}

func (tagFiles) Targets(payload []byte) (Targets, error) { return fileTagTargets(payload) }

type untagFiles struct{}

func (untagFiles) Apply(tx Tx, payload []byte) error {
	p, err := Payload[UntagFilesPayload](payload)
	if err != nil {
		return err
	}
	if len(p.FileIDs) == 0 {
		return fmt.Errorf("untag_files: %w", errEmptyBatch)
	}
	if err := checkFilesExist(tx, p.FileIDs); err != nil {
		return err
	}
	for _, id := range p.FileIDs {
		if err := tx.RemoveFileTag(id, p.TagID, p.Source); err != nil {
			return err
		}
	}
	return nil
}

func (untagFiles) Inverse(payload []byte) (Action, []byte, error) {
	return ActionTagFiles, payload, nil
}

func (untagFiles) Targets(payload []byte) (Targets, error) { return fileTagTargets(payload) }

// checkFilesExist is checkBoxesExist for files, and exists for the same
// reason: the foreign key is the backstop, this is the sentence a person reads.
func checkFilesExist(tx Tx, files []domain.FileID) error {
	for _, id := range files {
		_, ok, err := tx.GetFile(id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s no longer exists: %w", id, ErrStaleBefore)
		}
	}
	return nil
}

// fileTagTargets names files and no box.
//
// op_boxes is the index behind "what happened to this box", and putting a
// file-tag op in it would flood that view with every tag anyone ever put on
// any file inside — which is a different question, answered by op_files.
func fileTagTargets(payload []byte) (Targets, error) {
	p, err := Payload[TagFilesPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Files: p.FileIDs, Boxes: p.BoxIDs}, nil
}
