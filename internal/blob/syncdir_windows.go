//go:build windows

package blob

// syncDir is a no-op on Windows: directories cannot be opened for
// synchronization there. NTFS journals its metadata, so a committed rename is
// durable without an explicit flush.
func syncDir(string) error { return nil }
