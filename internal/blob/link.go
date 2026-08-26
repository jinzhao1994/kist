package blob

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// LinkMode is how the boxes/ projection points at a blob.
type LinkMode string

const (
	// LinkReflink shares blocks but not the inode: copy-on-write. Preferred on
	// macOS and on btrfs/XFS because editing the projected file in place
	// cannot corrupt the blob.
	LinkReflink LinkMode = "reflink"
	// LinkHardlink shares the inode. Free, but an in-place edit of the
	// projection would reach the blob — 0444 is the guard.
	LinkHardlink LinkMode = "hardlink"
	// LinkSymlink is a fallback for filesystems with neither. Not used on
	// Windows, where it needs elevation.
	LinkSymlink LinkMode = "symlink"
	// LinkCopy duplicates the bytes. Always works, costs the space.
	LinkCopy LinkMode = "copy"
)

// ErrUnsupportedLink means the platform or filesystem cannot do this.
var ErrUnsupportedLink = errors.New("link mode not supported here")

func isUnsupported(err error) bool { return errors.Is(err, ErrUnsupportedLink) }

// candidateModes lists the modes to try, best first. Symlink is skipped on
// Windows: it requires administrator rights or developer mode there, while
// hard links do not (spec 3.5).
func candidateModes() []LinkMode {
	if runtime.GOOS == "windows" {
		return []LinkMode{LinkReflink, LinkHardlink, LinkCopy}
	}
	return []LinkMode{LinkReflink, LinkHardlink, LinkSymlink, LinkCopy}
}

// DetectLinkMode finds the best mode that actually works between two
// directories, by trying each one for real.
//
// Probing beats inspecting the filesystem type: a type table has to be
// maintained forever, and it still cannot answer "are these two directories on
// the same volume". A genuine attempt answers both — a cross-device hard link
// simply fails with EXDEV — which is also why there is no separate st_dev
// check: it would need its own Windows implementation to say something the
// probe already established.
func DetectLinkMode(fromDir, toDir string) (LinkMode, error) {
	probe, err := os.CreateTemp(fromDir, ".kist-probe-*")
	if err != nil {
		return "", fmt.Errorf("create link probe: %w", err)
	}
	probePath := probe.Name()
	defer os.Remove(probePath)

	if _, err := probe.WriteString("kist link capability probe"); err != nil {
		probe.Close()
		return "", fmt.Errorf("write link probe: %w", err)
	}
	if err := probe.Close(); err != nil {
		return "", fmt.Errorf("close link probe: %w", err)
	}

	target := filepath.Join(toDir, ".kist-probe-target")
	for _, mode := range candidateModes() {
		os.Remove(target)
		if err := Link(mode, probePath, target); err == nil {
			os.Remove(target)
			return mode, nil
		}
	}
	os.Remove(target)
	return "", fmt.Errorf("no usable link mode between %s and %s", fromDir, toDir)
}

// Link points dst at src using the given mode.
func Link(mode LinkMode, src, dst string) error {
	switch mode {
	case LinkReflink:
		return reflink(src, dst)
	case LinkHardlink:
		return os.Link(src, dst)
	case LinkSymlink:
		if runtime.GOOS == "windows" {
			return fmt.Errorf("symlink on windows: %w", ErrUnsupportedLink)
		}
		return os.Symlink(src, dst)
	case LinkCopy:
		return copyFile(src, dst)
	default:
		return fmt.Errorf("unknown link mode %q", mode)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
