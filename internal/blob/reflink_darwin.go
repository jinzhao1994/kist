//go:build darwin

package blob

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// reflink uses clonefile(2), available on APFS. Preferred over a hard link
// because the clone gets its own inode: editing the projected file cannot
// reach the blob (spec 3.5).
//
// On HFS+, exFAT, or across volumes the syscall refuses; those refusals are
// translated into ErrUnsupportedLink so callers can tell "this filesystem
// cannot" apart from "this attempt went wrong".
func reflink(src, dst string) error {
	err := unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW)
	if isUnsupportedErrno(err) {
		return fmt.Errorf("clonefile: %w", ErrUnsupportedLink)
	}
	return err
}

func isUnsupportedErrno(err error) bool {
	return errors.Is(err, unix.ENOTSUP) ||
		errors.Is(err, unix.EOPNOTSUPP) ||
		errors.Is(err, unix.EXDEV) ||
		errors.Is(err, unix.EINVAL)
}
