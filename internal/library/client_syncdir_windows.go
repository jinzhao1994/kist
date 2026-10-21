//go:build windows

package library

// Windows does not support directory fsync. This follows the existing journal
// and blob durability boundary; data files are still explicitly flushed.
func syncClientDir(string) error { return nil }
