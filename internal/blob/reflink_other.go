//go:build !darwin && !linux

package blob

import "fmt"

// reflink is unavailable: Windows has no copy-on-write clone API that NTFS
// supports, and other platforms are not targeted.
func reflink(_, _ string) error {
	return fmt.Errorf("reflink: %w", ErrUnsupportedLink)
}
