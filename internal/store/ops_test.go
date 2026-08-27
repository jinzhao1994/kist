package store

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

func appendOp(t *testing.T, db *sql.DB, r OpRow) int64 {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	id, err := AppendOp(tx, r)
	if err != nil {
		t.Fatalf("AppendOp: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return id
}

func sampleOp() OpRow {
	return OpRow{
		TS:      time.Date(2026, 8, 27, 10, 30, 0, 123_000_000, time.UTC),
		Action:  "create_box",
		Source:  "user",
		Actor:   "cli",
		Payload: []byte(`{"box_id":"x"}`),
	}
}

func TestAppendOpAndGet(t *testing.T) {
	db := migrated(t)
	id := appendOp(t, db, sampleOp())
	if id <= 0 {
		t.Fatalf("AppendOp returned id %d", id)
	}

	got, ok, err := GetOp(db, id)
	if err != nil || !ok {
		t.Fatalf("GetOp: %v, ok=%v", err, ok)
	}
	if got.Action != "create_box" || got.Source != "user" || got.Actor != "cli" {
		t.Errorf("GetOp = %+v", got)
	}
	if string(got.Payload) != `{"box_id":"x"}` {
		t.Errorf("payload = %s", got.Payload)
	}
	want := sampleOp().TS
	if !got.TS.Equal(want) {
		t.Errorf("TS = %v, want %v", got.TS, want)
	}
	if got.TS.Location() != time.UTC {
		t.Errorf("TS came back in %v, want UTC", got.TS.Location())
	}
	if got.RevertsID != nil {
		t.Errorf("RevertsID = %v, want nil", got.RevertsID)
	}
}

func TestGetOpMissing(t *testing.T) {
	db := migrated(t)
	if _, ok, err := GetOp(db, 999); err != nil || ok {
		t.Errorf("GetOp(999) = ok %v, err %v; want ok=false, err=nil", ok, err)
	}
}

// The log's readability rests on this: ts is fixed width so its lexicographic
// order is its chronological order.
func TestTimestampIsFixedWidth(t *testing.T) {
	db := migrated(t)
	r := sampleOp()
	r.TS = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) // no millis at all
	id := appendOp(t, db, r)

	var raw string
	if err := db.QueryRow(`SELECT ts FROM ops WHERE id = ?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != "2026-01-02T03:04:05.000Z" {
		t.Errorf("stored ts = %q, want a zero-padded fixed-width form", raw)
	}
	if len(raw) != len("2006-01-02T15:04:05.000Z") {
		t.Errorf("ts is %d characters, must always be %d", len(raw), len("2006-01-02T15:04:05.000Z"))
	}
}

func TestTimestampIsNormalizedToUTC(t *testing.T) {
	db := migrated(t)
	shanghai := time.FixedZone("CST", 8*3600)
	r := sampleOp()
	r.TS = time.Date(2026, 8, 27, 18, 0, 0, 0, shanghai) // 10:00 UTC
	id := appendOp(t, db, r)

	var raw string
	db.QueryRow(`SELECT ts FROM ops WHERE id = ?`, id).Scan(&raw)
	if raw != "2026-08-27T10:00:00.000Z" {
		t.Errorf("stored ts = %q, want it converted to UTC", raw)
	}
}

func TestRevertsID(t *testing.T) {
	db := migrated(t)
	target := appendOp(t, db, sampleOp())

	inverse := sampleOp()
	inverse.Action = "delete_box"
	inverse.RevertsID = &target
	inverseID := appendOp(t, db, inverse)

	by, ok, err := RevertedBy(db, target)
	if err != nil {
		t.Fatalf("RevertedBy: %v", err)
	}
	if !ok || by != inverseID {
		t.Errorf("RevertedBy(%d) = %d, ok=%v; want %d", target, by, ok, inverseID)
	}

	if _, ok, _ := RevertedBy(db, inverseID); ok {
		t.Error("the inverse op should not itself be reported as reverted")
	}
}

func TestListOpsIsNewestFirst(t *testing.T) {
	db := migrated(t)
	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, appendOp(t, db, sampleOp()))
	}

	got, err := ListOps(db, ListOpsOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListOps: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d ops, want 3", len(got))
	}
	for i, want := range []int64{ids[4], ids[3], ids[2]} {
		if got[i].ID != want {
			t.Errorf("ops[%d].ID = %d, want %d", i, got[i].ID, want)
		}
	}
}

func TestListOpsByBox(t *testing.T) {
	db := migrated(t)
	boxA, boxB := domain.NewBoxID(), domain.NewBoxID()

	tx, _ := db.Begin()
	idA, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, idA, []domain.BoxID{boxA})
	idBoth, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, idBoth, []domain.BoxID{boxA, boxB})
	idB, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, idB, []domain.BoxID{boxB})
	tx.Commit()

	got, err := ListOps(db, ListOpsOptions{Box: &boxA, Limit: 10})
	if err != nil {
		t.Fatalf("ListOps: %v", err)
	}
	if len(got) != 2 || got[0].ID != idBoth || got[1].ID != idA {
		t.Errorf("ops for boxA = %v, want [%d %d]", opIDs(got), idBoth, idA)
	}
	_ = idB
}

func opIDs(ops []OpRow) []int64 {
	out := make([]int64, len(ops))
	for i, o := range ops {
		out[i] = o.ID
	}
	return out
}

// A batch operation is ONE op with many targets (spec 2.2.1), so naming the
// same box twice must not blow up on the primary key — and must not duplicate
// the row either.
func TestInsertOpBoxesIsIdempotent(t *testing.T) {
	db := migrated(t)
	box := domain.NewBoxID()

	tx, _ := db.Begin()
	id, _ := AppendOp(tx, sampleOp())
	if err := InsertOpBoxes(tx, id, []domain.BoxID{box, box}); err != nil {
		t.Fatalf("InsertOpBoxes with a repeated target: %v", err)
	}
	tx.Commit()

	var n int
	db.QueryRow(`SELECT count(*) FROM op_boxes WHERE op_id = ?`, id).Scan(&n)
	if n != 1 {
		t.Errorf("op_boxes has %d rows for one box, want 1", n)
	}
}

func TestLastOpTouchingBox(t *testing.T) {
	// spec 2.5: strict undo must be able to say WHICH op is in the way.
	db := migrated(t)
	box := domain.NewBoxID()

	tx, _ := db.Begin()
	first, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, first, []domain.BoxID{box})
	second, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, second, []domain.BoxID{box})
	tx.Commit()

	got, ok, err := LastOpTouchingBox(db, box, first)
	if err != nil {
		t.Fatalf("LastOpTouchingBox: %v", err)
	}
	if !ok || got != second {
		t.Errorf("LastOpTouchingBox after %d = %d (ok=%v), want %d", first, got, ok, second)
	}

	if _, ok, _ := LastOpTouchingBox(db, box, second); ok {
		t.Error("nothing touched the box after the last op, but something was reported")
	}
}

func TestDeleteOpIndexesLeavesOpsAlone(t *testing.T) {
	// Rebuild wipes the index tables and regenerates them from the payloads
	// (spec 2.2.1). Wiping ops as well would turn a repair into data loss.
	db := migrated(t)
	box := domain.NewBoxID()
	tx, _ := db.Begin()
	id, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, id, []domain.BoxID{box})
	tx.Commit()

	tx, _ = db.Begin()
	if err := DeleteOpIndexes(tx); err != nil {
		t.Fatalf("DeleteOpIndexes: %v", err)
	}
	tx.Commit()

	var indexes, ops int
	db.QueryRow(`SELECT count(*) FROM op_boxes`).Scan(&indexes)
	db.QueryRow(`SELECT count(*) FROM ops`).Scan(&ops)
	if indexes != 0 {
		t.Errorf("op_boxes still has %d rows", indexes)
	}
	if ops != 1 {
		t.Errorf("ops has %d rows, want 1 — it is authoritative and must survive", ops)
	}
}

// ListOps deliberately leaves Payload empty: the history view never renders
// it, and an import's payload is megabytes. GetOp is how you ask for the whole
// thing.
//
// This is a test rather than a comment because the natural way to add a field
// to the list query is to add it to opColumns, and that would silently undo
// the decision.
func TestListOpsDoesNotReadPayload(t *testing.T) {
	db := migrated(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	id, err := AppendOp(tx, OpRow{
		TS: testTime, Action: "import_files", Source: "user",
		Payload: []byte(`{"big":"` + strings.Repeat("x", 10000) + `"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	list, err := ListOps(db, ListOpsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("%d ops", len(list))
	}
	if len(list[0].Payload) != 0 {
		t.Errorf("ListOps returned %d payload bytes; the history view never uses them", len(list[0].Payload))
	}
	// Everything else about the row must still be there.
	if list[0].Action != "import_files" || list[0].ID != id || !list[0].TS.Equal(testTime) {
		t.Errorf("the header is incomplete: %+v", list[0])
	}

	full, ok, err := GetOp(db, id)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(full.Payload) == 0 {
		t.Error("GetOp must still return the payload")
	}
}

// IterOps is what Rebuild replays: oldest first, payload included, one row at
// a time.
func TestIterOpsWalksTheLogOldestFirst(t *testing.T) {
	db := migrated(t)
	for i := range 3 {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := AppendOp(tx, OpRow{
			TS: testTime, Action: "create_box", Source: "user",
			Payload: fmt.Appendf(nil, `{"n":%d}`, i),
		}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	var ids []int64
	var payloads []string
	if err := IterOps(db, func(r OpRow) error {
		ids = append(ids, r.ID)
		payloads = append(payloads, string(r.Payload))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.IsSorted(ids) {
		t.Errorf("ids = %v, want ascending — replay depends on it", ids)
	}
	if len(payloads) != 3 || payloads[0] != `{"n":0}` {
		t.Errorf("payloads = %v", payloads)
	}
}

// An error from the callback stops the walk and reaches the caller, so a
// replay failure is not silently swallowed halfway through.
func TestIterOpsStopsOnError(t *testing.T) {
	db := migrated(t)
	tx, _ := db.Begin()
	if _, err := AppendOp(tx, OpRow{TS: testTime, Action: "create_box", Source: "user", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	want := errors.New("stop")
	if err := IterOps(db, func(OpRow) error { return want }); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}
