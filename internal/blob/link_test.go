package blob

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectLinkModeReturnsSomethingUsable(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()

	mode, err := DetectLinkMode(from, to)
	if err != nil {
		t.Fatalf("DetectLinkMode: %v", err)
	}
	switch mode {
	case LinkReflink, LinkHardlink, LinkSymlink, LinkCopy:
	default:
		t.Fatalf("DetectLinkMode returned %q, not one of the four modes", mode)
	}

	// Copy always works, so detection must never fall below it.
	t.Logf("detected %q on %s", mode, runtime.GOOS)
}

func TestDetectLinkModeLeavesNothingBehind(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()

	if _, err := DetectLinkMode(from, to); err != nil {
		t.Fatalf("DetectLinkMode: %v", err)
	}
	for _, dir := range []string{from, to} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		if len(entries) != 0 {
			t.Errorf("%s holds %d probe leftovers, want none", dir, len(entries))
		}
	}
}

func TestLinkReproducesContentInEverySupportedMode(t *testing.T) {
	for _, mode := range []LinkMode{LinkReflink, LinkHardlink, LinkSymlink, LinkCopy} {
		t.Run(string(mode), func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "src")
			dst := filepath.Join(dir, "dst")
			if err := os.WriteFile(src, []byte("linked content"), 0o444); err != nil {
				t.Fatalf("write source: %v", err)
			}

			err := Link(mode, src, dst)
			if isUnsupported(err) {
				t.Skipf("%s is not available here", mode)
			}
			if err != nil {
				t.Fatalf("Link(%s): %v", mode, err)
			}

			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read destination: %v", err)
			}
			if string(got) != "linked content" {
				t.Errorf("destination holds %q, want %q", got, "linked content")
			}
		})
	}
}

func TestLinkRejectsUnknownMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Link(LinkMode("teleport"), src, filepath.Join(dir, "dst")); err == nil {
		t.Error("Link with an unknown mode = nil error, want a rejection")
	}
}

func TestNoSymlinksOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only policy")
	}
	// Symlinks need administrator rights or developer mode there, hard links
	// do not (spec 3.5). Detection must never choose symlink on Windows.
	from, to := t.TempDir(), t.TempDir()
	mode, err := DetectLinkMode(from, to)
	if err != nil {
		t.Fatalf("DetectLinkMode: %v", err)
	}
	if mode == LinkSymlink {
		t.Error("detection chose symlink on Windows")
	}
}
