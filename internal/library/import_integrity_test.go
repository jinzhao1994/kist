package library

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func identity(content string) *ImportIntegrity {
	sum := sha256.Sum256([]byte(content))
	return &ImportIntegrity{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(content))}
}

func TestVerifiedImportRejectsWholeBatch(t *testing.T) {
	for _, sizeMismatch := range []bool{false, true} {
		l, box := openWithBox(t)
		root := writeTree(t, map[string]string{"a.txt": "correct", "b.txt": "changed"})
		expected := identity("original")
		if sizeMismatch {
			expected = identity("changed")
			expected.Size++
		}
		prepared, err := l.PrepareFiles(t.Context(), []ImportSource{
			{Path: filepath.Join(root, "a.txt"), RelativePath: "a.txt", Expected: identity("correct")},
			{Path: filepath.Join(root, "b.txt"), RelativePath: "b.txt", Expected: expected},
		}, ImportOptions{Box: box})
		if prepared != nil || !errors.Is(err, ErrImportIntegrity) {
			t.Fatalf("prepare = %v, %v", prepared, err)
		}
		if files := listAll(t, l, box); len(files) != 0 {
			t.Fatalf("archived %d files", len(files))
		}
	}
}

func TestVerifiedImportReadsBackFinalBlob(t *testing.T) {
	l, box := openWithBox(t)
	root := writeTree(t, map[string]string{"a.txt": "original"})
	l.hooks.afterFile = func() {
		err := filepath.WalkDir(l.blobs.Root(), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && filepath.Base(path) == identity("original").SHA256 {
				if err := os.Chmod(path, 0600); err != nil {
					return err
				}
				return os.WriteFile(path, []byte("modified"), 0600)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	prepared, err := l.PrepareFiles(t.Context(), []ImportSource{{Path: filepath.Join(root, "a.txt"), RelativePath: "a.txt", Expected: identity("original")}}, ImportOptions{Box: box})
	if prepared != nil || !errors.Is(err, ErrImportIntegrity) {
		t.Fatalf("prepare = %v, %v", prepared, err)
	}
	if len(listAll(t, l, box)) != 0 {
		t.Fatal("corrupt file archived")
	}
}

func TestVerifiedImportAndDuplicate(t *testing.T) {
	l, box := openWithBox(t)
	root := writeTree(t, map[string]string{"a.txt": "original"})
	for range 2 {
		prepared, err := l.PrepareFiles(t.Context(), []ImportSource{{Path: filepath.Join(root, "a.txt"), RelativePath: "a.txt", Expected: identity("original")}}, ImportOptions{Box: box})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := prepared.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if len(listAll(t, l, box)) != 1 {
		t.Fatal("unexpected archived file count")
	}
}
