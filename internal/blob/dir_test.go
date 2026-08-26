package blob

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newTestDir(t *testing.T) *Dir {
	t.Helper()
	d, err := NewDir(t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewDir: %v", err)
	}
	return d
}

func put(t *testing.T, d *Dir, content string) PutResult {
	t.Helper()
	res, err := d.Put(context.Background(), strings.NewReader(content))
	if err != nil {
		t.Fatalf("Put(%q): %v", content, err)
	}
	return res
}

func TestPutThenGet(t *testing.T) {
	d := newTestDir(t)
	res := put(t, d, "hello archive")

	if res.Outcome != OutcomeStored {
		t.Errorf("Outcome = %v, want OutcomeStored", res.Outcome)
	}
	if res.Size != int64(len("hello archive")) {
		t.Errorf("Size = %d, want %d", res.Size, len("hello archive"))
	}
	if res.Key.Hex() != hexOf("hello archive") {
		t.Errorf("Key = %q, does not address its content", res.Key)
	}

	r, err := d.Get(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "hello archive" {
		t.Errorf("Get returned %q, want %q", got, "hello archive")
	}
}

func TestPutIsContentAddressedAndDeduplicates(t *testing.T) {
	d := newTestDir(t)

	first := put(t, d, "same content")
	second := put(t, d, "same content")

	if first.Key != second.Key {
		t.Errorf("same content produced different keys: %q and %q", first.Key, second.Key)
	}
	if second.Outcome != OutcomeDeduped {
		t.Errorf("second Outcome = %v, want OutcomeDeduped", second.Outcome)
	}
}

func TestPutLeavesNoTempFiles(t *testing.T) {
	d := newTestDir(t)
	put(t, d, "one")
	put(t, d, "one") // dedup path must clean up its temp file too
	put(t, d, "two")

	entries, err := os.ReadDir(filepath.Join(d.Root(), "tmp"))
	if err != nil {
		t.Fatalf("read tmp: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("tmp/ holds %d leftovers, want none", len(entries))
	}
}

func TestStoredBlobIsReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not apply")
	}
	d := newTestDir(t)
	res := put(t, d, "immutable")

	p, ok := d.LocalPath(res.Key)
	if !ok {
		t.Fatal("LocalPath returned false for a local store")
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// 0444 is the second line of defence behind reflink: if a user edits a file
	// under boxes/ that happens to be a hard link, the write fails instead of
	// corrupting the blob (spec 3.5).
	if perm := fi.Mode().Perm(); perm != 0o444 {
		t.Errorf("stored blob mode = %v, want 0444", perm)
	}
}

func TestPutRepairsCorruptExistingBlob(t *testing.T) {
	d := newTestDir(t)
	res := put(t, d, "original content")

	// Simulate rot: overwrite the blob in place, keeping its name.
	p, _ := d.LocalPath(res.Key)
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := os.WriteFile(p, []byte("rotted!"), 0o600); err != nil {
		t.Fatalf("corrupt the blob: %v", err)
	}

	again := put(t, d, "original content")
	if again.Outcome != OutcomeRepaired {
		t.Errorf("Outcome = %v, want OutcomeRepaired", again.Outcome)
	}
	if again.Key != res.Key {
		t.Errorf("repair changed the key: %q -> %q", res.Key, again.Key)
	}

	r, err := d.Get(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != "original content" {
		t.Errorf("after repair Get returned %q, want the original content", got)
	}
}

func TestPutHandlesTrueCollision(t *testing.T) {
	// A real SHA-256 collision cannot be manufactured, so the collision branch
	// is reached by planting different content under a name that claims the
	// candidate's hash — which is exactly the state a collision produces.
	d := newTestDir(t)

	key := NewKey(AlgoSHA256, hexOf("the real content"), 0)
	p := filepath.Join(d.Root(), key.RelPath())
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Content whose own hash matches the filename, but which differs from what
	// we are about to Put: only possible under a collision.
	if err := os.WriteFile(p, []byte("the real content"), 0o444); err != nil {
		t.Fatalf("plant: %v", err)
	}

	// Now Put content that hashes to the same value. We cannot actually
	// produce one, so drive the branch directly.
	res, err := d.putWithHash(context.Background(), strings.NewReader("colliding content"), key.Hex(), int64(len("colliding content")))
	if err != nil {
		t.Fatalf("putWithHash: %v", err)
	}
	if res.Outcome != OutcomeCollided {
		t.Errorf("Outcome = %v, want OutcomeCollided", res.Outcome)
	}
	if res.Key.Suffix() != 1 {
		t.Errorf("Key = %q, want collision suffix .1", res.Key)
	}

	// The original must be untouched — git's SHAttered bug was exactly this:
	// silently keeping one and discarding the other (spec 3.3).
	r, err := d.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get original: %v", err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != "the real content" {
		t.Errorf("the pre-existing blob was modified: %q", got)
	}
}

func TestStat(t *testing.T) {
	d := newTestDir(t)
	res := put(t, d, "measure me")

	info, err := d.Stat(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Key != res.Key || info.Size != int64(len("measure me")) {
		t.Errorf("Stat = %+v, want key %q size %d", info, res.Key, len("measure me"))
	}

	missing := NewKey(AlgoSHA256, hexOf("never stored"), 0)
	if _, err := d.Stat(context.Background(), missing); err == nil {
		t.Error("Stat of a missing blob = nil error, want a rejection")
	}
}

func TestDelete(t *testing.T) {
	d := newTestDir(t)
	res := put(t, d, "temporary")

	// Blobs are 0444; Delete must cope with that on every platform.
	if err := d.Delete(context.Background(), res.Key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := d.Stat(context.Background(), res.Key); err == nil {
		t.Error("blob still present after Delete")
	}
}

func TestGetIsSeekable(t *testing.T) {
	// Range requests over HTTP need this (M4), and it is free for a file.
	d := newTestDir(t)
	res := put(t, d, "0123456789")

	r, err := d.Get(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()

	if _, err := r.Seek(5, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	got, _ := io.ReadAll(r)
	if string(got) != "56789" {
		t.Errorf("after Seek(5) read %q, want %q", got, "56789")
	}
}

func TestPutLargeStream(t *testing.T) {
	d := newTestDir(t)
	payload := bytes.Repeat([]byte("kist"), 500_000) // ~2 MB

	res, err := d.Put(context.Background(), bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if res.Size != int64(len(payload)) {
		t.Errorf("Size = %d, want %d", res.Size, len(payload))
	}
}

func TestNewDirCreatesLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "blobs")
	if _, err := NewDir(root, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatalf("NewDir: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(root, "tmp")); err != nil || !fi.IsDir() {
		t.Errorf("tmp/ was not created: %v", err)
	}
}
