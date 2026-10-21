package library

import (
	"strconv"

	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// checkImportUndoMetadata runs after catch-up under the writer lock. Legacy
// delete events did not assert metadata and must remain replayable; this guard
// only refuses new undo commands, never changes the meaning of recorded events.
func checkImportUndoMetadata(tx *store.ViewTx, payload []byte, target int64) error {
	p, err := oplog.Payload[oplog.ImportFilesPayload](payload)
	if err != nil {
		return err
	}
	for _, f := range p.Files {
		got, ok, err := tx.GetFile(f.FileID)
		if err != nil {
			return err
		}
		if !ok {
			continue // The existing Apply assertion explains missing files.
		}
		rows, err := store.FileMetadata(tx.Queryer(), f.FileID)
		if err != nil {
			return err
		}
		versions, err := store.Extractions(tx.Queryer(), f.FileID)
		if err != nil {
			return err
		}
		expected := map[string]string{"file:size": strconv.FormatInt(f.Size, 10)}
		if f.MTime != 0 {
			expected["file:mtime"] = strconv.FormatInt(f.MTime, 10)
		}
		matches := got.CapturedAt == f.CapturedAt && len(versions) == 0 && len(rows) == len(expected)
		for _, row := range rows {
			value, exists := expected[row.Key]
			if !exists || row.Tombstone || row.Source != extract.ImportSource || row.Type != extract.TypeInt || row.Value != value {
				matches = false
			}
		}
		if matches {
			continue
		}
		// A later rename or tag edit in the box does not explain this failure.
		ops, err := store.ListOps(tx.Queryer(), store.ListOpsOptions{
			File: &f.FileID, Limit: 1,
			Actions: []string{string(oplog.ActionExtractMetadata), string(oplog.ActionSetMetadata)},
		})
		if err != nil {
			return err
		}
		blocked := &BlockedUndoError{OpID: target}
		if len(ops) != 0 && ops[0].ID > target {
			blocked.BlockingOp = ops[0].ID
		}
		return blocked
	}
	return nil
}
