package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func countUnder(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(_ string, e os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !e.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Restore is repair's primitive: take content from somewhere untrusted, hash
// it on the way in, and only keep it if it is what was asked for.
func TestRestoreWritesContentThatMatches(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "the original content").Key
	corrupt(t, d, k, "the corrupted content")

	outcome, err := d.Restore(context.Background(), strings.NewReader("the original content"), k)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != OutcomeRepaired {
		t.Errorf("outcome = %v, want OutcomeRepaired", outcome)
	}
	got, _, err := d.Rehash(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	if got != k.Hex() {
		t.Error("the blob still does not hash to its own name")
	}
}

// The property the whole feature rests on. A remote that hands back the wrong
// bytes — a truncated copy, a different file, a hostile one — must change
// nothing and must not leave the bad content anywhere.
func TestRestoreRefusesContentThatDoesNotMatch(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "the original content").Key
	corrupt(t, d, k, "rot")

	if _, err := d.Restore(context.Background(), strings.NewReader("something else entirely"), k); err == nil {
		t.Fatal("Restore accepted content that hashes to something else")
	}

	// The damaged blob is untouched — damaged content beats none — and the
	// wrong content was not stored anywhere.
	p, _ := d.LocalPath(k)
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "rot" {
		t.Errorf("the existing blob was modified: %q", body)
	}
	if n := countUnder(t, filepath.Join(d.Root(), "sha256")); n != 1 {
		t.Errorf("%d blobs on disk, want 1 — the rejected content was stored", n)
	}
	if n := countUnder(t, filepath.Join(d.Root(), "tmp")); n != 0 {
		t.Errorf("%d files left in tmp/", n)
	}
}

// A blob that is missing entirely is the other half of repair.
func TestRestoreCreatesAMissingBlob(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "content").Key
	p, _ := d.LocalPath(k)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}

	outcome, err := d.Restore(context.Background(), strings.NewReader("content"), k)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != OutcomeStored {
		t.Errorf("outcome = %v, want OutcomeStored", outcome)
	}
}

// Restoring something that is already fine is a no-op, not a rewrite. repair
// works from a report that may be minutes old, and rewriting healthy content
// would make it a command that touches files for no reason.
func TestRestoreOfHealthyContentChangesNothing(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "content").Key

	outcome, err := d.Restore(context.Background(), strings.NewReader("content"), k)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != OutcomeDeduped {
		t.Errorf("outcome = %v, want OutcomeDeduped", outcome)
	}
}

// A key with a collision suffix names one specific slot. Put would walk the
// suffixes from zero and land in the wrong one (spec 3.3).
func TestRestoreHonoursTheCollisionSuffix(t *testing.T) {
	d := newTestDir(t)
	k := NewKey(AlgoSHA256, sha256Hex("content"), 1)

	if _, err := d.Restore(context.Background(), strings.NewReader("content"), k); err != nil {
		t.Fatal(err)
	}
	p, _ := d.LocalPath(k)
	if _, err := os.Stat(p); err != nil {
		t.Errorf("the blob did not land at suffix 1: %v", err)
	}
	bare := NewKey(AlgoSHA256, sha256Hex("content"), 0)
	bp, _ := d.LocalPath(bare)
	if _, err := os.Stat(bp); err == nil {
		t.Error("the content also landed at suffix 0, which is a different blob")
	}
}

// Restore must leave nothing in tmp/ when it succeeds either.
func TestRestoreLeavesNoTempFiles(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "content").Key
	corrupt(t, d, k, "rotrotr")

	if _, err := d.Restore(context.Background(), strings.NewReader("content"), k); err != nil {
		t.Fatal(err)
	}
	if n := countUnder(t, filepath.Join(d.Root(), "tmp")); n != 0 {
		t.Errorf("%d files left in tmp/", n)
	}
}
