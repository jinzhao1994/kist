package projection

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/blob"
)

// LinkProblem is a projected file that exists but is not what it should be.
type LinkProblem struct {
	Path   string
	Reason string
}

// CheckLinks confirms that the files already under boxes/ point at the content
// they are supposed to.
//
// How much can be confirmed depends on the link mode, and spec 3.8 accepts
// that rather than papering over it:
//
//	hardlink   the link and the blob are one inode — os.SameFile settles it
//	symlink    read the link target
//	reflink    size only: a reflink is a separate inode with shared extents
//	copy       size only: there is no identity at all
//
// Under the last two, damaged content is scrub's finding, not verify's.
// Hashing here would make verify read every byte in the library, and a verify
// that slow does not get run.
//
// Files that are not there at all are DiffDir's finding and are passed over
// here, so that one missing file is one line in the report rather than two.
func (s *Syncer) CheckLinks(want []Entry) ([]LinkProblem, error) {
	var out []LinkProblem
	for _, e := range want {
		if e.IsDir || e.BlobKey == "" {
			continue
		}
		abs := filepath.Join(s.Root, filepath.FromSlash(e.Path))
		info, err := os.Lstat(abs)
		if err != nil {
			continue // missing entirely — DiffDir says so
		}
		src, ok := s.Locate(e.BlobKey)
		if !ok {
			continue // the blob layer has already reported this
		}

		switch s.LinkMode {
		case blob.LinkSymlink:
			target, err := os.Readlink(abs)
			if err != nil {
				out = append(out, LinkProblem{e.Path, fmt.Sprintf("not a symlink: %v", err)})
				continue
			}
			if target != src {
				out = append(out, LinkProblem{e.Path, fmt.Sprintf("points at %s, not %s", target, src)})
			}
		case blob.LinkHardlink:
			blobInfo, err := os.Stat(src)
			if err != nil {
				continue
			}
			if !os.SameFile(info, blobInfo) {
				out = append(out, LinkProblem{e.Path, "not the same file as its blob"})
			}
		default:
			blobInfo, err := os.Stat(src)
			if err != nil {
				continue
			}
			if info.Size() != blobInfo.Size() {
				out = append(out, LinkProblem{
					e.Path,
					fmt.Sprintf("%d bytes, but its blob is %d", info.Size(), blobInfo.Size()),
				})
			}
		}
	}
	return out, nil
}
