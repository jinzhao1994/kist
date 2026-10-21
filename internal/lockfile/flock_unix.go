//go:build !windows

package lockfile

import (
	"errors"
	"os"
	"syscall"
)

// tryLock takes a shared or exclusive lock without blocking, reporting false
// if another holder prevents acquisition.
//
// flock is per open file description, not per process, so two Lockers in one
// process contend exactly as two processes would. The Locker's semaphore does
// not depend on that, but it does mean this file needs no special case for it.
func tryLock(f *os.File, shared bool) (bool, error) {
	mode := syscall.LOCK_EX
	if shared {
		mode = syscall.LOCK_SH
	}
	err := syscall.Flock(int(f.Fd()), mode|syscall.LOCK_NB)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.EWOULDBLOCK), errors.Is(err, syscall.EAGAIN):
		return false, nil
	default:
		return false, err
	}
}

func unlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
