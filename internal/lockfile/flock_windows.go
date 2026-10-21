//go:build windows

package lockfile

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// Windows locks byte ranges, and a locked range is unreadable to other
// handles. Locking one byte far past any content therefore keeps kist.lock
// readable — which matters, because the process that failed to get the lock is
// about to read the file to find out whose fault that is.
const (
	lockOffsetHigh = 0x4000_0000
	lockLength     = 1
)

func overlapped() *windows.Overlapped {
	return &windows.Overlapped{OffsetHigh: lockOffsetHigh}
}

func tryLock(f *os.File, shared bool) (bool, error) {
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if !shared {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	err := windows.LockFileEx(windows.Handle(f.Fd()), flags,
		0, lockLength, 0, overlapped())
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION), errors.Is(err, windows.ERROR_IO_PENDING):
		return false, nil
	default:
		return false, err
	}
}

func unlock(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, lockLength, 0, overlapped())
}
