//go:build !windows

package library

import "golang.org/x/sys/unix"

// freeSpace reports the bytes available to an unprivileged user. Failure is
// not fatal: init prints "unknown" and carries on.
func freeSpace(path string) (uint64, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, false
	}
	return st.Bavail * uint64(st.Bsize), true
}
