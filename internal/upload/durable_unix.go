//go:build !windows

package upload

import "os"

func replacePath(root *os.Root, from, to string) error { return root.Rename(from, to) }

func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
