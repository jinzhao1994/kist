package library

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestSyncEnvelopeSurvivesCommitRecoveryAndRebuild(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "recover"}[crash], func(t *testing.T) {
			l := openTestLibrary(t)
			accepted := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
			occurred := accepted.Add(-72 * time.Hour)
			l.now = func() time.Time { return accepted }
			p := oplog.CreateBoxPayload{BoxID: domain.NewBoxID(), RootFolderID: domain.NewFolderID(), BoxFields: oplog.BoxFields{Name: "offline"}}
			raw, err := oplog.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			if crash {
				l.hooks.afterAppend = func() error { return errors.New("power cut") }
			}
			id, err := l.write(t.Context(), oplog.ActionCreateBox, raw, oplog.OriginUser, writeOptions{sync: &syncWrite{ID: "pending-create-box", Digest: strings.Repeat("a", 64), OccurredAt: occurred}})
			if id != 1 || (err != nil) != crash {
				t.Fatalf("id=%d err=%v", id, err)
			}
			root := l.Layout().Root
			l.Close()
			l, err = Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			var local, digest string
			var at, created int64
			if err := l.db.QueryRow(`SELECT local_id,digest,accepted_at FROM op_sync WHERE op_id=1`).Scan(&local, &digest, &at); err != nil {
				t.Fatal(err)
			}
			if local != "pending-create-box" || digest != strings.Repeat("a", 64) || at != accepted.UnixNano() {
				t.Fatalf("sync=%s %s %d", local, digest, at)
			}
			row, found, err := store.GetOp(l.db, 1)
			if err != nil || !found || !row.TS.Equal(occurred) || row.Origin != "user" {
				t.Fatalf("row=%+v err=%v", row, err)
			}
			if err := l.db.QueryRow(`SELECT created_at FROM boxes WHERE id=?`, p.BoxID).Scan(&created); err != nil {
				t.Fatal(err)
			}
			if created != accepted.UnixNano() {
				t.Fatalf("applied at %d, want acceptance", created)
			}
			before := snapshotOf(t, l)
			if err := l.Rebuild(t.Context()); err != nil {
				t.Fatal(err)
			}
			if snapshotOf(t, l) != before {
				t.Fatal("rebuild changed sync semantics")
			}
			if _, err := l.db.Exec(`UPDATE op_sync SET digest='corrupt'`); err != nil {
				t.Fatal(err)
			}
			report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
			if err != nil || report.Count(integrity.ViewDrift) == 0 {
				t.Fatalf("drift not detected: %v %v", report, err)
			}
			if err := l.Rebuild(t.Context()); err != nil {
				t.Fatal(err)
			}
			if snapshotOf(t, l) != before {
				t.Fatal("rebuild did not restore sync metadata")
			}
		})
	}
}
