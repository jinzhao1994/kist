package oplog

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

func init() { Register(ActionSetMetadata, setMetadata{}) }

// MetadataState is what one (file, key) slot holds, or nil for an empty slot.
//
// Tombstone is a third state, distinct from both a value and an empty slot: a
// key a person removed. Without it a re-extraction would put the row back,
// because "absent" is exactly what an extractor is there to fill (spec 1.7.7).
type MetadataState struct {
	Value     string       `json:"value,omitempty"`
	Type      extract.Type `json:"type,omitempty"`
	Source    string       `json:"source"`
	Tombstone bool         `json:"tombstone,omitempty"`
}

// KeyChange is one slot's before and after. Inverse is a swap.
type KeyChange struct {
	Key    string         `json:"key"`
	Before *MetadataState `json:"before,omitempty"`
	After  *MetadataState `json:"after,omitempty"`
}

// FileMetadataChange is every slot this op touches on one file.
//
// CapturedAt travels with it because removing a key can change the kernel's
// conclusion about when the file was taken — deleting exif:DateTimeOriginal
// has to fall back to file:mtime. It is resolved when the op is generated, not
// inside Apply, for the same reason the import path resolves it there: the
// resolution reads the library's timezone, and configuration is not payload
// (spec 1.5.2, invariant 3).
type FileMetadataChange struct {
	FileID      domain.FileID  `json:"file_id"`
	Keys        []KeyChange    `json:"keys"`
	CapturedAt  domain.Instant `json:"captured_at,omitempty"`
	CapturedWas domain.Instant `json:"captured_was,omitempty"`
}

// SetMetadataPayload is one person's edit to stored metadata.
//
// It is deliberately not extract_metadata with Extractor set to "user": that
// op also maintains file_extractions, and a person is not an extractor that
// ran at a version.
type SetMetadataPayload struct {
	Files []FileMetadataChange `json:"files"`
}

type setMetadata struct{}

func (setMetadata) Apply(tx Tx, payload []byte) error {
	p, err := Payload[SetMetadataPayload](payload)
	if err != nil {
		return err
	}
	if len(p.Files) == 0 {
		return fmt.Errorf("set_metadata: %w", errEmptyBatch)
	}
	if err := checkFilesExist(tx, targetsOfMetadata(p)); err != nil {
		return err
	}

	for _, f := range p.Files {
		if len(f.Keys) == 0 {
			return fmt.Errorf("set_metadata: %s: %w", f.FileID, errEmptyBatch)
		}
		for _, k := range f.Keys {
			if k.Key == "" {
				return fmt.Errorf("set_metadata: %s: a change names no key", f.FileID)
			}
			if err := assertMetadataState(tx, f.FileID, k); err != nil {
				return err
			}
			if err := writeMetadataState(tx, f.FileID, k); err != nil {
				return err
			}
		}
		if err := tx.SetCapturedAt(f.FileID, f.CapturedAt); err != nil {
			return err
		}
	}
	return nil
}

// assertMetadataState refuses the op if the slot no longer holds what the
// payload recorded. It is an assertion, not a decision: the outcomes are
// "proceed exactly as recorded" and "fail" (spec 2.5).
func assertMetadataState(tx Tx, file domain.FileID, k KeyChange) error {
	row, ok, err := tx.GetFileMetadata(file, k.Key)
	if err != nil {
		return err
	}
	if !ok {
		if k.Before != nil {
			return fmt.Errorf("%s on %s is already gone: %w", k.Key, file, ErrStaleBefore)
		}
		return nil
	}
	if k.Before == nil {
		return fmt.Errorf("%s on %s was written after this op: %w", k.Key, file, ErrStaleBefore)
	}
	if row.Tombstone != k.Before.Tombstone || row.Source != k.Before.Source ||
		(!row.Tombstone && (row.Value != k.Before.Value || row.Type != k.Before.Type)) {
		return fmt.Errorf("%s on %s has changed since this op: %w", k.Key, file, ErrStaleBefore)
	}
	return nil
}

func writeMetadataState(tx Tx, file domain.FileID, k KeyChange) error {
	switch {
	case k.After == nil:
		return tx.DeleteFileMetadata(file, k.Key)
	case k.After.Tombstone:
		return tx.TombstoneFileMetadata(file, k.Key)
	default:
		// Same reason extract_metadata checks it: Apply routes on the type
		// tag, and a payload from a newer kist could carry one this build does
		// not know (spec 2.2).
		if !knownType(k.After.Type) {
			return fmt.Errorf("set_metadata: %s: unknown value type %q", k.Key, k.After.Type)
		}
		kv := extract.KV{Key: k.Key, Value: k.After.Value, Type: k.After.Type}
		return tx.SetFileMetadata(file, kv, k.After.Source)
	}
}

// Inverse swaps every before and after, including captured_at.
func (setMetadata) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[SetMetadataPayload](payload)
	if err != nil {
		return "", nil, err
	}
	inverse := SetMetadataPayload{Files: make([]FileMetadataChange, len(p.Files))}
	for i, f := range p.Files {
		keys := make([]KeyChange, len(f.Keys))
		for j, k := range f.Keys {
			keys[j] = KeyChange{Key: k.Key, Before: k.After, After: k.Before}
		}
		inverse.Files[i] = FileMetadataChange{
			FileID:      f.FileID,
			Keys:        keys,
			CapturedAt:  f.CapturedWas,
			CapturedWas: f.CapturedAt,
		}
	}
	raw, err := Marshal(inverse)
	if err != nil {
		return "", nil, err
	}
	return ActionSetMetadata, raw, nil
}

// Targets reports the files. No boxes: metadata is a label on content, not
// content, so it does not move a box's updated_at (spec 1.5.1 ②).
func (setMetadata) Targets(payload []byte) (Targets, error) {
	p, err := Payload[SetMetadataPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Files: targetsOfMetadata(p)}, nil
}

func targetsOfMetadata(p SetMetadataPayload) []domain.FileID {
	out := make([]domain.FileID, len(p.Files))
	for i, f := range p.Files {
		out[i] = f.FileID
	}
	return out
}
