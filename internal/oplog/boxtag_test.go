package oplog

import (
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func threeBoxes(t *testing.T, tx Tx) []domain.BoxID {
	t.Helper()
	var out []domain.BoxID
	for _, name := range []string{"一", "二", "三"} {
		p, raw := createBoxPayload(t, name)
		if err := apply(t, tx, ActionCreateBox, raw); err != nil {
			t.Fatal(err)
		}
		out = append(out, p.BoxID)
	}
	return out
}

func TestTagBoxesIsOneOpForManyTargets(t *testing.T) {
	// spec 2.2.1: batching is the unit. Tagging 12 boxes must be one op, or
	// undoing it means twelve clicks.
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")

	raw, _ := Marshal(TagBoxesPayload{TagID: tag, BoxIDs: boxes})
	if err := apply(t, tx, ActionTagBoxes, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, b := range boxes {
		if has, _ := tx.HasBoxTag(b, tag); !has {
			t.Errorf("box %s did not get the tag", b)
		}
	}

	h, _ := Lookup(ActionTagBoxes)
	targets, err := h.Targets(raw)
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	if len(targets.Boxes) != 3 {
		t.Errorf("Targets.Boxes has %d entries, want 3", len(targets.Boxes))
	}
}

func TestTagBoxesInverseIsUntag(t *testing.T) {
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")

	raw, _ := Marshal(TagBoxesPayload{TagID: tag, BoxIDs: boxes})
	apply(t, tx, ActionTagBoxes, raw)

	action, inv := inverseOf(t, ActionTagBoxes, raw)
	if action != ActionUntagBoxes {
		t.Fatalf("inverse action = %q, want untag_boxes", action)
	}
	if err := apply(t, tx, action, inv); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	for _, b := range boxes {
		if has, _ := tx.HasBoxTag(b, tag); has {
			t.Errorf("box %s still carries the tag after undo", b)
		}
	}
}

func TestUntagBoxesInverseIsTag(t *testing.T) {
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")
	for _, b := range boxes {
		tx.AddBoxTag(b, tag)
	}

	raw, _ := Marshal(UntagBoxesPayload{TagID: tag, BoxIDs: boxes})
	apply(t, tx, ActionUntagBoxes, raw)

	action, inv := inverseOf(t, ActionUntagBoxes, raw)
	if action != ActionTagBoxes {
		t.Fatalf("inverse action = %q, want tag_boxes", action)
	}
	apply(t, tx, action, inv)
	for _, b := range boxes {
		if has, _ := tx.HasBoxTag(b, tag); !has {
			t.Errorf("box %s did not get its tag back", b)
		}
	}
}

// The payload names only the boxes that actually changed. If it named boxes
// that already carried the tag, the inverse would strip a tag the op never
// added — an undo doing more than the op did.
func TestTagBoxesPayloadOnlyNamesRealChanges(t *testing.T) {
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")

	// The first box already carries the tag; library would have filtered it
	// out when building the op, so the payload holds only the other two.
	tx.AddBoxTag(boxes[0], tag)

	raw, _ := Marshal(TagBoxesPayload{TagID: tag, BoxIDs: boxes[1:]})
	apply(t, tx, ActionTagBoxes, raw)

	action, inv := inverseOf(t, ActionTagBoxes, raw)
	apply(t, tx, action, inv)

	if has, _ := tx.HasBoxTag(boxes[0], tag); !has {
		t.Error("undo removed a tag the op never added")
	}
}

func TestTagBoxesRejectsAnEmptyBatch(t *testing.T) {
	// An op that changes nothing should never have been written. Letting it
	// through would litter the history with entries that undo to nothing.
	tx := newFakeTx(opTime)
	tag := createTag(t, tx, nil, "云南")
	raw, _ := Marshal(TagBoxesPayload{TagID: tag})
	if err := apply(t, tx, ActionTagBoxes, raw); err == nil {
		t.Error("an empty batch was accepted")
	}
}

func TestBoxTagOpsRefuseADeletedBox(t *testing.T) {
	// Found by walking the CLI by hand: undo a tagging, undo the box's
	// creation, then undo the first undo. The last step tries to re-tag a box
	// that is gone, and without this check the user is shown
	// "FOREIGN KEY constraint failed (787)" instead of a sentence.
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")

	// Delete one of the boxes out from under the op.
	tx.DeleteFoldersOfBox(boxes[1])
	tx.DeleteBox(boxes[1])

	for _, action := range []Action{ActionTagBoxes, ActionUntagBoxes} {
		raw, _ := Marshal(TagBoxesPayload{TagID: tag, BoxIDs: boxes})
		err := apply(t, tx, action, raw)
		if err == nil {
			t.Fatalf("%s on a deleted box succeeded", action)
		}
		if !errors.Is(err, ErrStaleBefore) {
			t.Errorf("%s error = %v, want ErrStaleBefore", action, err)
		}
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			t.Errorf("%s leaked a database constraint error: %v", action, err)
		}
	}
}
