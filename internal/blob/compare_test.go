package blob

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func hexOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestCompareAndHashIdentical(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "the same bytes")
	b := writeFile(t, dir, "b", "the same bytes")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if !identical {
		t.Error("identical = false, want true")
	}
	if want := hexOf("the same bytes"); h != want {
		t.Errorf("existingHex = %q, want %q", h, want)
	}
}

func TestCompareAndHashDifferentSameLength(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "aaaaaaaa")
	b := writeFile(t, dir, "b", "aaaaaaab")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if identical {
		t.Error("identical = true, want false")
	}
	// The hash must describe the EXISTING file, not the candidate — the caller
	// uses it to decide whether the existing blob is corrupt.
	if want := hexOf("aaaaaaaa"); h != want {
		t.Errorf("existingHex = %q, want the hash of the existing file %q", h, want)
	}
}

func TestCompareAndHashDifferentLength(t *testing.T) {
	dir := t.TempDir()
	// The existing file is longer. A naive implementation that stops at the
	// first mismatch would return a hash of the prefix only, and the caller
	// would then wrongly conclude the existing blob is corrupt.
	a := writeFile(t, dir, "a", "hello world, and then some more")
	b := writeFile(t, dir, "b", "hello world")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if identical {
		t.Error("identical = true, want false")
	}
	if want := hexOf("hello world, and then some more"); h != want {
		t.Errorf("existingHex = %q, want the full hash of the existing file", h)
	}
}

func TestCompareAndHashCandidateLonger(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "hello")
	b := writeFile(t, dir, "b", "hello world")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if identical {
		t.Error("identical = true, want false")
	}
	if want := hexOf("hello"); h != want {
		t.Errorf("existingHex = %q, want %q", h, want)
	}
}

func TestCompareAndHashEmpty(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if !identical {
		t.Error("two empty files should compare identical")
	}
	if want := hexOf(""); h != want {
		t.Errorf("existingHex = %q, want the hash of the empty string", h)
	}
}

func TestCompareAndHashLargerThanBuffer(t *testing.T) {
	// Exercise the multi-block path, with the difference deliberately placed
	// past the first block boundary.
	dir := t.TempDir()
	base := strings.Repeat("x", 300_000)
	a := writeFile(t, dir, "a", base+"A")
	b := writeFile(t, dir, "b", base+"B")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if identical {
		t.Error("identical = true, want false")
	}
	if want := hexOf(base + "A"); h != want {
		t.Errorf("existingHex is wrong for a multi-block file")
	}
}

func TestCompareAndHashMissingFile(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "x")

	if _, _, err := compareAndHash(a, filepath.Join(dir, "nope")); err == nil {
		t.Error("expected an error for a missing candidate")
	}
	if _, _, err := compareAndHash(filepath.Join(dir, "nope"), a); err == nil {
		t.Error("expected an error for a missing existing file")
	}
}
