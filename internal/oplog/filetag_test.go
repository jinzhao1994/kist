package oplog

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestTagFilesApplyAndInverse(t *testing.T) {
	tx := newFakeTx(opTime)
	box, root := seedBox(t, tx)
	imp := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, imp)

	tag := domain.NewTagID()
	p := TagFilesPayload{
		TagID:   tag,
		FileIDs: []domain.FileID{imp.Files[0].FileID, imp.Files[1].FileID},
		Source:  "user",
	}
	mustApply(t, tx, ActionTagFiles, p)

	for _, id := range p.FileIDs {
		if has, _ := tx.HasFileTag(id, tag); !has {
			t.Errorf("file %s did not get the tag", id)
		}
	}

	action, _ := inverseOf(t, ActionTagFiles, mustRaw(t, p))
	if action != ActionUntagFiles {
		t.Fatalf("inverse action = %q, want %q", action, ActionUntagFiles)
	}
	mustApply(t, tx, ActionUntagFiles, p)
	for _, id := range p.FileIDs {
		if has, _ := tx.HasFileTag(id, tag); has {
			t.Errorf("file %s still carries the tag after the inverse", id)
		}
	}
}

func mustRaw(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTagFilesRefusesAnEmptyBatch(t *testing.T) {
	tx := newFakeTx(opTime)
	err := applyPayload(t, tx, ActionTagFiles, TagFilesPayload{TagID: domain.NewTagID(), Source: "user"})
	if !errors.Is(err, errEmptyBatch) {
		t.Errorf("err = %v, want errEmptyBatch", err)
	}
}

// The readable-sentence rule from M1b applies here too: a foreign key error
// number is not an error message (spec 2.5).
func TestTagFilesNamesAMissingFile(t *testing.T) {
	tx := newFakeTx(opTime)
	p := TagFilesPayload{TagID: domain.NewTagID(), FileIDs: []domain.FileID{domain.NewFileID()}, Source: "user"}
	err := applyPayload(t, tx, ActionTagFiles, p)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore", err)
	}
}

func TestTagFilesTargets(t *testing.T) {
	h, err := Lookup(ActionTagFiles)
	if err != nil {
		t.Fatal(err)
	}
	p := TagFilesPayload{TagID: domain.NewTagID(), FileIDs: []domain.FileID{domain.NewFileID()}, Source: "user"}
	got, err := h.Targets(mustRaw(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || got.Files[0] != p.FileIDs[0] {
		t.Errorf("Files = %v, want %v", got.Files, p.FileIDs)
	}
	// A file-tag op names no box: op_boxes is the index behind "what happened
	// to this box", and widening it here would flood that view with every tag
	// anyone ever put on any file inside.
	if len(got.Boxes) != 0 {
		t.Errorf("Boxes = %v, want none", got.Boxes)
	}
}
