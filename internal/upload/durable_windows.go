package upload

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func replacePath(root *os.Root, oldName, newName string) error {
	from, err := windows.UTF16PtrFromString(filepath.Join(root.Name(), oldName))
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(filepath.Join(root.Name(), newName))
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

// Windows does not expose directory fsync. Data is flushed before the
// write-through replacement; process restarts retain confirmed progress.
func syncDirectory(string) error { return nil }
