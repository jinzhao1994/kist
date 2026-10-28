package oplog

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

func init() { Register(ActionExtractMetadata, extractMetadata{}) }

// FileExtraction is what one extraction did to one file.
//
// Only what actually changed is listed. That is the same rule tag_files
// follows, and for the same reason: an op that claims to have changed
// something it did not would invert into an undo that changes something the
// user never did (spec 1.7.1).
//
// Was and CapturedWas carry the previous values, which is what makes Inverse a
// plain swap and strict undo possible at all (spec 2.3, 2.5). On a first
// extraction they are empty, so nothing is doubled — the doubling only appears
// where a re-extraction really did overwrite something.
type FileExtraction struct {
	FileID domain.FileID `json:"file_id"`
	// Set are the rows this extraction writes.
	Set []extract.KV `json:"set,omitempty"`
	// Was are the values those same keys held before, plus the values of any
	// key in Removed. A key absent here did not exist before.
	Was []extract.KV `json:"was,omitempty"`
	// Removed carries explicit undo and previously recorded removals. New
	// extraction runs preserve unreported fields and do not populate it.
	Removed []string `json:"removed,omitempty"`
	// CapturedAt is the kernel's re-resolved conclusion (spec 1.5.2), carried
	// in the same op because it is computed from the rows above.
	CapturedAt  domain.Instant `json:"captured_at,omitempty"`
	CapturedWas domain.Instant `json:"captured_was,omitempty"`
}

// ExtractMetadataPayload is one batch from one extractor.
//
// Batched rather than one op per file, and bounded rather than one op for the
// library: spec 3.7.1 rejected whole-batch rollback for imports because a run
// interrupted at 99% would lose everything and would hold a write transaction
// for hours. Re-extraction has exactly that shape.
type ExtractMetadataPayload struct {
	Extractor string `json:"extractor"`
	// Version is what file_extractions will record. Inverse swaps it back to
	// PreviousVersion, without which an undo would leave the index claiming
	// these files had already been seen — and the next run would skip them,
	// silently and forever.
	Version         int `json:"version"`
	PreviousVersion int `json:"previous_version,omitempty"`

	Files []FileExtraction `json:"files"`
}

type extractMetadata struct{}

func (extractMetadata) Apply(tx Tx, payload []byte) error {
	p, err := Payload[ExtractMetadataPayload](payload)
	if err != nil {
		return err
	}
	if p.Extractor == "" {
		return fmt.Errorf("extract_metadata: no extractor named")
	}
	if len(p.Files) == 0 {
		return fmt.Errorf("extract_metadata: %w", errEmptyBatch)
	}
	if err := checkFilesExist(tx, targetsOfExtraction(p)); err != nil {
		return err
	}

	for _, f := range p.Files {
		before := make(map[string]*MetadataState, len(f.Was))
		for _, kv := range f.Was {
			before[kv.Key] = &MetadataState{Value: kv.Value, Type: kv.Type, Source: p.Extractor}
		}
		for _, kv := range f.Set {
			if err := assertMetadataState(tx, f.FileID, KeyChange{Key: kv.Key, Before: before[kv.Key]}); err != nil {
				return err
			}
		}
		for _, key := range f.Removed {
			if err := assertMetadataState(tx, f.FileID, KeyChange{Key: key, Before: before[key]}); err != nil {
				return err
			}
		}
		if err := assertCapturedChange(tx, f.FileID, f.CapturedWas, f.CapturedAt); err != nil {
			return err
		}
		for _, kv := range f.Set {
			// The type tag is checked here rather than trusted, because Apply
			// routes on it and a payload written by a newer kist could carry
			// one this build does not know. Half-applying such an op would be
			// worse than refusing it (spec 2.2).
			if !knownType(kv.Type) {
				return fmt.Errorf("extract_metadata: %s: unknown value type %q", kv.Key, kv.Type)
			}
			if err := tx.SetFileMetadata(f.FileID, kv, p.Extractor); err != nil {
				return err
			}
		}
		for _, key := range f.Removed {
			if err := tx.DeleteFileMetadata(f.FileID, key); err != nil {
				return err
			}
		}
		if f.CapturedAt != f.CapturedWas {
			if err := tx.SetCapturedAt(f.FileID, f.CapturedAt); err != nil {
				return err
			}
		}
		if p.Version == 0 {
			if err := tx.DeleteFileExtraction(f.FileID, p.Extractor); err != nil {
				return err
			}
			continue
		}
		if err := tx.SetFileExtraction(f.FileID, p.Extractor, p.Version); err != nil {
			return err
		}
	}
	return nil
}

// Inverse swaps every before and after.
//
// The version swap is the part that is easy to miss and impossible to notice:
// without it an undo restores the rows but leaves file_extractions saying this
// extractor already ran at the new version, so the next extraction skips these
// files. Nothing errors. The data is simply never recomputed.
//
// A first extraction has PreviousVersion zero, and Apply reads that as "remove
// the row" rather than "record version 0" — which is what makes undoing a
// first extraction leave the file looking untouched.
func (extractMetadata) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[ExtractMetadataPayload](payload)
	if err != nil {
		return "", nil, err
	}
	inverse := ExtractMetadataPayload{
		Extractor:       p.Extractor,
		Version:         p.PreviousVersion,
		PreviousVersion: p.Version,
		Files:           make([]FileExtraction, len(p.Files)),
	}
	for i, f := range p.Files {
		had := map[string]bool{}
		for _, kv := range f.Was {
			had[kv.Key] = true
		}
		// Keys this op created have no previous value, so undoing it removes
		// them. Keys it removed had one, so undoing it writes them back.
		var removed []string
		for _, kv := range f.Set {
			if !had[kv.Key] {
				removed = append(removed, kv.Key)
			}
		}
		inverse.Files[i] = FileExtraction{
			FileID:      f.FileID,
			Set:         f.Was,
			Was:         f.Set,
			Removed:     removed,
			CapturedAt:  f.CapturedWas,
			CapturedWas: f.CapturedAt,
		}
	}
	raw, err := Marshal(inverse)
	if err != nil {
		return "", nil, err
	}
	return ActionExtractMetadata, raw, nil
}

// Targets reports the files. No boxes: metadata changes nothing about a box,
// and an extractor-sourced op does not propagate updated_at anyway (spec
// 1.5.1 ②).
func (extractMetadata) Targets(payload []byte) (Targets, error) {
	p, err := Payload[ExtractMetadataPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Files: targetsOfExtraction(p)}, nil
}

func targetsOfExtraction(p ExtractMetadataPayload) []domain.FileID {
	out := make([]domain.FileID, len(p.Files))
	for i, f := range p.Files {
		out[i] = f.FileID
	}
	return out
}

func knownType(t extract.Type) bool {
	switch t {
	case extract.TypeText, extract.TypeInt, extract.TypeReal:
		return true
	}
	return false
}
