//go:build windows

package journal

// syncDir is a no-op on Windows: directories cannot be opened for
// synchronization there. NTFS journals its metadata, so a created file's name
// is durable without an explicit flush.
func syncDir(string) error { return nil }
