package library

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// TestACrashLeavesTheDatabaseBehindNeverAhead is invariant 2's new half.
//
// Verified to fail: move the Append below tx.Commit in write and the log comes
// up empty here, which is the unrecoverable direction — views claiming a
// change no log entry justifies.
func TestACrashLeavesTheDatabaseBehindNeverAhead(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	l, err := Open(root, WithClock(fixedClock()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	boom := errors.New("power cut")
	l.hooks.afterAppend = func() error { return boom }
	id, writeErr := l.write(ctx, oplog.ActionCreateBox,
		newBoxPayload(t, "云南", 1), oplog.OriginUser, writeOptions{})
	var outcome *WriteOutcomeError
	if !errors.Is(writeErr, boom) || !errors.As(writeErr, &outcome) || outcome.OpID != 1 || id != 1 {
		t.Fatalf("write = %d, %v; want committed op 1 with the injected cause", id, writeErr)
	}

	last, ok, err := l.ops.Last()
	if err != nil || !ok {
		t.Fatalf("the log lost the op it had already flushed: %v ok=%v", err, ok)
	}
	if last.ID != 1 {
		t.Fatalf("log ends at op %d, want 1", last.ID)
	}
	dbID, err := store.MaxOpID(l.db)
	if err != nil {
		t.Fatal(err)
	}
	if dbID != 0 {
		t.Fatalf("the database applied op %d after the write failed", dbID)
	}
	l.Close()

	// The next Open is the repair.
	l2, err := Open(root, WithClock(fixedClock()))
	if err != nil {
		t.Fatalf("Open did not recover: %v", err)
	}
	defer l2.Close()

	dbID, err = store.MaxOpID(l2.db)
	if err != nil {
		t.Fatal(err)
	}
	if dbID != 1 {
		t.Errorf("after Open the database is at op %d, want 1", dbID)
	}
	boxes, err := l2.Boxes(context.Background(), store.ListBoxesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes) != 1 || boxes[0].Name != "云南" {
		t.Errorf("the replayed op did not produce its box: %+v", boxes)
	}
}

// TestOpenRefusesALibraryWhoseLogIsGone covers both readings of that state: a
// pre-release library whose log is still in kist.db, and one whose ops/ was
// deleted. Neither has an automatic answer — the authority is the missing half.
func TestOpenRefusesALibraryWhoseLogIsGone(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	l, err := Open(root, WithClock(fixedClock()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.write(context.Background(), oplog.ActionCreateBox,
		newBoxPayload(t, "云南", 1), oplog.OriginUser, writeOptions{}); err != nil {
		t.Fatal(err)
	}
	l.Close()

	if err := os.RemoveAll(filepath.Join(root, "ops")); err != nil {
		t.Fatal(err)
	}
	_, err = Open(root)
	if !errors.Is(err, ErrNoJournal) {
		t.Fatalf("Open = %v, want ErrNoJournal", err)
	}
}

// TestDeepVerifyCatchesACorruptedOpsTable is the check the old shape could not
// perform.
//
// When the replay read the ops table, a rewritten op row was replayed into
// itself and matched. The scratch database is built from ops/ now, so the two
// sides are independent (spec 3.8).
func TestDeepVerifyCatchesACorruptedOpsTable(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	if _, err := l.write(ctx, oplog.ActionCreateBox,
		newBoxPayload(t, "云南", 1), oplog.OriginUser, writeOptions{}); err != nil {
		t.Fatal(err)
	}

	if _, err := l.db.Exec(`UPDATE ops SET origin = 'maintenance' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	report, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Count(integrity.ViewDrift) == 0 {
		t.Error("a rewritten ops row went unreported by verify --deep")
	}
}

// newBoxPayload builds a create_box payload with fixed ids, so a replay
// produces exactly the same rows as the original write.
func newBoxPayload(t *testing.T, name string, n byte) []byte {
	t.Helper()
	boxID, err := domain.ParseID(fmt.Sprintf("01900000-0000-7000-8000-0000000000%02d", n))
	if err != nil {
		t.Fatal(err)
	}
	rootID, err := domain.ParseID(fmt.Sprintf("01900000-0000-7000-8000-0000000001%02d", n))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := oplog.Marshal(oplog.CreateBoxPayload{
		BoxID:        domain.BoxID{ID: boxID},
		RootFolderID: domain.FolderID{ID: rootID},
		BoxFields:    oplog.BoxFields{Name: name},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestVerifyReportsTheLagInsteadOfFixingIt is spec 3.8's line, seen from the
// new failure mode.
//
// Open repairs a database that is behind the log, which is right for every
// ordinary command. verify is not an ordinary command: it promises not to
// write to the library, and a verify in cron that quietly finished someone
// else's interrupted write would be reporting on a library it had just
// changed.
func TestVerifyReportsTheLagInsteadOfFixingIt(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	l, err := Open(root, WithClock(fixedClock()))
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("power cut")
	l.hooks.afterAppend = func() error { return boom }
	if _, err := l.write(context.Background(), oplog.ActionCreateBox,
		newBoxPayload(t, "云南", 1), oplog.OriginUser, writeOptions{}); !errors.Is(err, boom) {
		t.Fatalf("write = %v, want the injected failure", err)
	}
	l.Close()

	checker, err := Open(root, WithClock(fixedClock()), WithoutRecovery())
	if err != nil {
		t.Fatal(err)
	}
	defer checker.Close()

	before, err := store.MaxOpID(checker.db)
	if err != nil {
		t.Fatal(err)
	}
	report, err := checker.Verify(context.Background(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.MaxOpID(checker.db)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("verify advanced the database from op %d to %d; it must not write", before, after)
	}
	if report.Count(integrity.LogAhead) != 1 {
		t.Errorf("verify did not report the lag: %d LogAhead findings", report.Count(integrity.LogAhead))
	}
}

func TestRejectedWritesLeaveTheJournalReplayable(t *testing.T) {
	for _, kind := range []string{"tag in use", "stale undo", "unique constraint"} {
		t.Run(kind, func(t *testing.T) {
			l := openTestLibrary(t)
			ctx := context.Background()
			parent, err := l.CreateTag(ctx, "parent", nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := l.CreateTag(ctx, "child", &parent.ID); err != nil {
				t.Fatal(err)
			}
			before, _, err := l.ops.Last()
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "tag in use":
				err = l.DeleteTag(ctx, parent.ID)
			case "stale undo":
				if err := l.RenameTag(ctx, parent.ID, "renamed"); err != nil {
					t.Fatal(err)
				}
				before, _, err = l.ops.Last()
				if err != nil {
					t.Fatal(err)
				}
				_, err = l.Undo(ctx, 1)
			case "unique constraint":
				raw, marshalErr := oplog.Marshal(oplog.CreateTagPayload{TagID: domain.NewTagID(), Name: "parent"})
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				_, err = l.write(ctx, oplog.ActionCreateTag, raw, oplog.OriginUser, writeOptions{})
			}
			if err == nil {
				t.Fatal("invalid operation succeeded")
			}
			after, _, err := l.ops.Last()
			if err != nil {
				t.Fatal(err)
			}
			if after.ID != before.ID {
				t.Fatalf("rejected write advanced journal: %d -> %d", before.ID, after.ID)
			}
			root := l.Layout().Root
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(root)
			if err != nil {
				t.Fatalf("reopen after rejection: %v", err)
			}
			defer reopened.Close()
			if _, err := reopened.CreateTag(ctx, "next", nil); err != nil {
				t.Fatalf("write after rejection: %v", err)
			}
			if err := reopened.Rebuild(ctx); err != nil {
				t.Fatalf("rebuild: %v", err)
			}
			report, err := reopened.Verify(ctx, VerifyOptions{Deep: true})
			if err != nil {
				t.Fatal(err)
			}
			if report.Count(integrity.ViewDrift) != 0 {
				t.Fatal("rebuild differs from live state")
			}
		})
	}
}
