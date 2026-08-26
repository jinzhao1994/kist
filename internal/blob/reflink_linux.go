//go:build linux

package blob

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// reflink uses the FICLONE ioctl, available on btrfs and XFS. It fails with
// EOPNOTSUPP on ext4, which is what makes detection fall through to hard links.
//
// Those refusals are translated into ErrUnsupportedLink so callers can tell
// "this filesystem cannot" apart from "this attempt went wrong".
func reflink(src, dst string) error {
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

	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		out.Close()
		os.Remove(dst)
		if isUnsupportedErrno(err) {
			return fmt.Errorf("ficlone: %w", ErrUnsupportedLink)
		}
		return err
	}
	return out.Close()
}

func isUnsupportedErrno(err error) bool {
	return errors.Is(err, unix.ENOTSUP) ||
		errors.Is(err, unix.EOPNOTSUPP) ||
		errors.Is(err, unix.EXDEV) ||
		errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.ENOTTY)
}
