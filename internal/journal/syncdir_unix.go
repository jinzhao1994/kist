//go:build !windows

package journal

import (
	"fmt"
	"os"
)

// syncDir flushes a directory entry so a newly created bucket survives a power
// cut. Without it the file's contents are durable but the name pointing at
// them may not be — and a bucket that vanishes after its ops were applied is
// the one direction spec 2.2.2 forbids: the database ahead of the log.
func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for fsync: %w", err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("fsync directory: %w", err)
	}
	return nil
}
