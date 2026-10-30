package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageUsageCountsLocalFilesWithoutProjections(t *testing.T) {
	for _, client := range []bool{false, true} {
		root := t.TempDir()
		files := map[string]int{"kist.toml": 3, "kist.db": 7, "kist.db-wal": 11, "kist.db-shm": 13, "boxes/duplicate": 1000, "uploads/temp": 1000}
		expected := StorageUsage{MetadataBytes: 34}
		if client {
			files["local-blobs/original"] = 17
			files["catalog/op"] = 19
			files["pending/op"] = 23
			files["cache/objects/original.data"] = 29
			files["cache/objects/preview.data"] = 31
			files["cache/objects/original.json"] = 5
			files["cache/derived/descriptor.json"] = 7
			expected.OriginalsBytes = 46
			expected.MetadataBytes += 54
			expected.PreviewsBytes = 31
		} else {
			files["blobs/original"] = 17
			files["ops/op"] = 19
			files["derived/thumb"] = 31
			files["derived/preview_1080p_v1.mp4"] = 37
			files["transcodes/segment.ts"] = 41
			expected.OriginalsBytes = 17
			expected.MetadataBytes += 19
			expected.PreviewsBytes = 31
			expected.VideoBytes = 78
		}
		for name, size := range files {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, make([]byte, size), 0600); err != nil {
				t.Fatal(err)
			}
		}
		got, err := measureStorage(t.Context(), root, client, map[string]int64{"sha256:preview": 31}, nil)
		if err != nil || got != expected {
			t.Fatalf("client=%v: got %+v want %+v: %v", client, got, expected, err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := measureStorage(ctx, root, client, nil, nil); err != context.Canceled {
			t.Fatalf("canceled scan: %v", err)
		}
	}
}

func TestStorageUsageSkipsMissingDirectoriesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "blobs"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(target, []byte("not archive content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "blobs", "link")); err != nil {
		t.Skip("symlinks unavailable")
	}
	got, err := measureStorage(t.Context(), root, false, nil, nil)
	if err != nil || got != (StorageUsage{}) {
		t.Fatalf("got %+v: %v", got, err)
	}
}
