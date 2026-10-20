package projection

import (
	"fmt"
	"path"
	"slices"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// Entry is one thing that must exist under boxes/.
//
// ModTime is set for directories only. A hard link and its blob are the same
// inode, so stamping a file link's mtime would rewrite the blob's — and two
// files sharing a deduplicated blob would then fight over it, making relink
// non-idempotent. Directories have no such sharing (spec 3.4.1, amended by the
// M1c plan).
type Entry struct {
	// Path is slash-separated and relative to boxes/, starting with the box
	// directory itself.
	Path    string
	IsDir   bool
	BlobKey string
	ModTime time.Time
}

// Plan computes the desired projection of one box.
//
// It is a pure function: same entities in, same tree out, no filesystem and no
// clock. Name collisions — including the ones only Sanitize creates — are
// resolved here rather than discovered by the filesystem halfway through
// writing.
//
// Entries come out sorted by path, which puts a parent before everything
// inside it. Sync relies on that when creating and reverses it when removing.
func Plan(boxDir string, box domain.Box, folders []domain.Folder, files []domain.File, sets ...domain.Fileset) ([]Entry, error) {
	local := make([]domain.Fileset, 0, len(sets))
	folderIDs := make(map[domain.FolderID]bool, len(folders))
	for _, folder := range folders {
		folderIDs[folder.ID] = true
	}
	for _, set := range sets {
		if folderIDs[set.FolderID] {
			local = append(local, set)
		}
	}
	if len(local) > 0 {
		sets = local
		return planFilesets(boxDir, box, folders, files, sets)
	}
	var root *domain.Folder
	children := map[domain.FolderID][]domain.Folder{}
	for i := range folders {
		f := folders[i]
		if f.ParentID == nil {
			if root != nil {
				return nil, fmt.Errorf("box %s has more than one root folder", box.ID)
			}
			root = &folders[i]
			continue
		}
		children[*f.ParentID] = append(children[*f.ParentID], f)
	}
	if root == nil {
		return nil, fmt.Errorf("box %s has no root folder", box.ID)
	}

	inFolder := map[domain.FolderID][]domain.File{}
	for _, f := range files {
		inFolder[f.FolderID] = append(inFolder[f.FolderID], f)
	}

	out := []Entry{{Path: boxDir, IsDir: true, ModTime: box.UpdatedAt}}
	seen := 1
	var walk func(f domain.Folder, dir string) error
	walk = func(f domain.Folder, dir string) error {
		// Folders and files share a namespace on disk, so they are resolved
		// together — a folder called "notes" and a file called "notes" cannot
		// both exist.
		named := make([]Named, 0, len(children[f.ID])+len(inFolder[f.ID]))
		for _, sub := range children[f.ID] {
			named = append(named, Named{ID: sub.ID.ID, Name: sub.Name})
		}
		for _, file := range inFolder[f.ID] {
			named = append(named, Named{ID: file.ID.ID, Name: file.Name})
		}
		names := Resolve(named)

		for _, file := range inFolder[f.ID] {
			out = append(out, Entry{
				Path:    path.Join(dir, names[file.ID.ID]),
				BlobKey: file.BlobKey,
			})
			seen++
		}
		for _, sub := range children[f.ID] {
			subDir := path.Join(dir, names[sub.ID.ID])
			out = append(out, Entry{Path: subDir, IsDir: true, ModTime: sub.UpdatedAt})
			seen++
			if err := walk(sub, subDir); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(*root, boxDir); err != nil {
		return nil, err
	}

	// Anything not reached is a file or folder whose parent is missing — a
	// corrupt view, not something to project halfway (spec 2.1 says the answer
	// to a corrupt view is rebuild, and saying so beats silently dropping it).
	if want := 1 + len(folders) - 1 + len(files); seen != want {
		return nil, fmt.Errorf("box %s: %d of %d entries are unreachable from the root folder",
			box.ID, want-seen, want)
	}

	slices.SortFunc(out, func(a, b Entry) int {
		switch {
		case a.Path < b.Path:
			return -1
		case a.Path > b.Path:
			return 1
		}
		return 0
	})
	return out, nil
}
