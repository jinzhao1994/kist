package library

import (
	"fmt"
	"slices"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// validateWriteState checks current-state constraints for new writes and
// rebased pending intents. It is deliberately absent from historical replay.
func validateWriteState(tx *store.ViewTx, row store.OpRow) error {
	action, payload := oplog.Action(row.Action), row.Payload
	if action == oplog.ActionTagBoxes || action == oplog.ActionUntagBoxes {
		p, err := oplog.Payload[oplog.TagBoxesPayload](payload)
		if err != nil {
			return err
		}
		for _, box := range p.BoxIDs {
			present, err := tx.HasBoxTag(box, p.TagID)
			if err != nil {
				return err
			}
			// Commands record only changed associations. Accepting a no-op here
			// would let its inverse undo another writer's association instead.
			if present != (action == oplog.ActionUntagBoxes) {
				return fmt.Errorf("box %s tag association changed: %w", box, oplog.ErrStaleBefore)
			}
		}
	}
	if action == oplog.ActionUpdateBox && row.RevertsID == nil {
		p, err := oplog.Payload[oplog.UpdateBoxPayload](payload)
		if err != nil {
			return err
		}
		if p.CoverFileID != nil {
			if err := checkBoxCover(tx, p.BoxID, p.CoverFileID.After); err != nil {
				return err
			}
		}
	}

	// Recheck state-dependent constraints after recovery, while no writer can
	// invalidate them. This also covers inverse ops, which bypass commands.
	if row.RevertsID != nil {
		if by, done, err := store.RevertedBy(tx.Queryer(), *row.RevertsID); err != nil {
			return err
		} else if done {
			return &AlreadyRevertedError{OpID: *row.RevertsID, RevertedBy: by}
		}
		if action == oplog.ActionUndoUploadFiles {
			p, err := oplog.Payload[oplog.UploadFilesPayload](payload)
			if err != nil {
				return err
			}
			raw, err := oplog.Marshal(p.Import)
			if err != nil {
				return err
			}
			if err := checkImportUndoMetadata(tx, raw, *row.RevertsID); err != nil {
				return err
			}
		}
		if action == oplog.ActionDeleteFiles {
			if err := checkImportUndoMetadata(tx, payload, *row.RevertsID); err != nil {
				return err
			}
		}
	}
	if action == oplog.ActionMoveTag {
		p, err := oplog.Payload[oplog.MoveTagPayload](payload)
		if err != nil {
			return err
		}
		if p.NewParent != nil {
			above, err := store.TagAncestors(tx.Queryer(), *p.NewParent)
			if err != nil {
				return err
			}
			if slices.Contains(above, p.TagID) {
				return ErrTagCycle
			}
		}
	}

	return nil
}
