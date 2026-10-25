package library

import (
	"encoding/json"
	"github.com/jinzhao1994/kist/internal/journal"
	"os"
	"path/filepath"
	"testing"
)

func TestClientViewFreshnessSkipsReplayAndRecoversMissingStamp(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "photo.png", "original")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	// Fail if an unchanged read clears and replays the materialized view.
	if _, err := c.db.Exec(`CREATE TRIGGER reject_replay BEFORE DELETE ON files BEGIN SELECT RAISE(ABORT, 'unexpected replay'); END`); err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.File(ctx, file.ID); err != nil || !found {
		t.Fatalf("read: %v %v", found, err)
	}
	if size, err := c.originalSize(ctx, file.BlobKey); err != nil || size != 8 {
		t.Fatalf("size: %d %v", size, err)
	}
	if err := os.Remove(c.viewStampPath()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.File(ctx, file.ID); err == nil {
		t.Fatal("missing stamp must force replay")
	}
	if _, err := os.Stat(c.viewStampPath()); !os.IsNotExist(err) {
		t.Fatalf("failed replay published stamp: %v", err)
	}
	if _, err := c.db.Exec(`DROP TRIGGER reject_replay`); err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.File(ctx, file.ID); err != nil || !found {
		t.Fatalf("recovery: %v %v", found, err)
	}
	// A malformed disposable stamp must also recover from the journals.
	if err := os.WriteFile(c.viewStampPath(), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.File(ctx, file.ID); err != nil || !found {
		t.Fatalf("corrupt stamp: %v %v", found, err)
	}
}

func TestClientViewFreshnessObservesOtherHandle(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	other, err := OpenClient(t.Context(), c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.UpdateBox(t.Context(), box, BoxFields{Name: "changed elsewhere"}); err != nil {
		t.Fatal(err)
	}
	got, found, err := c.Box(t.Context(), box)
	if err != nil || !found || got.Name != "changed elsewhere" {
		t.Fatalf("stale view: %+v %v %v", got, found, err)
	}
	// Simulate interruption after invalidation but before replay/commit.
	if err := other.invalidateViewStamp(); err != nil {
		t.Fatal(err)
	}
	if _, err := other.db.Exec(`UPDATE boxes SET name='stale' WHERE id=?`, box); err != nil {
		t.Fatal(err)
	}
	got, found, err = c.Box(t.Context(), box)
	if err != nil || !found || got.Name != "changed elsewhere" {
		t.Fatalf("interrupted rebuild: %+v %v %v", got, found, err)
	}
}

func TestClientViewDigestCoversEarlierRecords(t *testing.T) {
	var digests []string
	var tails []string
	for _, payload := range []string{`{"value":1}`, `{"value":2}`} {
		log, err := journal.NewDir(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		for i, body := range []string{payload, `{}`} {
			if err := log.Append(journal.Line{ID: int64(i + 1), TS: "2026-10-25T00:00:00Z", Action: "test", Origin: "user", Payload: json.RawMessage(body)}); err != nil {
				t.Fatal(err)
			}
		}
		var tail string
		if err := log.Iter(func(line journal.Line) error { tail = line.Sum; return nil }); err != nil {
			t.Fatal(err)
		}
		digest, err := journalPrefixDigest(t.Context(), log, 2, tail)
		if err != nil {
			t.Fatal(err)
		}
		digests = append(digests, digest)
		tails = append(tails, tail)
	}
	if tails[0] != tails[1] || digests[0] == digests[1] {
		t.Fatal("fingerprint must detect different histories with identical last records")
	}
}

func TestClientViewFreshnessRecreatesDatabaseWithOldStamp(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "photo.png", "original")
	c := newSyncClient(t, nas)
	root := c.root
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	// The stamp must never substitute for rebuilding a deleted derived database.
	if err := os.Remove(filepath.Join(root, "client.db")); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenClient(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, found, err := reopened.File(t.Context(), file.ID); err != nil || !found {
		t.Fatalf("recreated database: %v %v", found, err)
	}
}
