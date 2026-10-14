//go:build !windows

package errcodes

import (
	"errors"
	"syscall"
)

func isStorageFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}
