//go:build !windows

package blob

import (
	"fmt"
	"os"
)

// syncDir flushes a directory entry so a rename survives a power cut. Without
// it the file's contents are durable but the name pointing at them may not be.
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
