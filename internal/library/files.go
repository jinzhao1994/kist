package library

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// ErrNoSuchFile means the id names nothing in this library.
var ErrNoSuchFile = fmt.Errorf("no such file")

// ErrNoSuchFolder means the path names nothing in this box.
var ErrNoSuchFolder = fmt.Errorf("no such folder")

// Files runs a file query and returns the page plus the cursor for the next
// one (spec 4.2, the M1c subset).
func (l *Library) Files(ctx context.Context, opt ListFilesOptions) ([]domain.File, string, error) {
	return store.ListFiles(l.db, opt)
}

// File reads one file.
func (l *Library) File(ctx context.Context, id domain.FileID) (domain.File, bool, error) {
	var (
		f     domain.File
		found bool
	)
	err := l.readTx(ctx, func(vtx *store.ViewTx) error {
		var err error
		f, found, err = vtx.GetFile(id)
		return err
	})
	return f, found, err
}

// Folders lists every folder of a box, root first.
func (l *Library) Folders(ctx context.Context, box domain.BoxID) ([]domain.Folder, error) {
	return store.FoldersOfBox(l.db, box)
}

// FileTags returns the tags attached to a file.
func (l *Library) FileTags(ctx context.Context, id domain.FileID) ([]domain.TagID, error) {
	return store.FileTags(l.db, id)
}

// ResolveFolder turns a slash path inside a box into a folder.
//
// The empty path and "." are the box root. Matching is on the folded name, the
// same as the uniqueness constraint — so `kist ls 云南 Day1` finds `day1`,
// which is what anyone typing a path expects.
func (l *Library) ResolveFolder(ctx context.Context, box domain.BoxID, p string) (domain.Folder, error) {
	folders, err := store.FoldersOfBox(l.db, box)
	if err != nil {
		return domain.Folder{}, err
	}
	if len(folders) == 0 {
		return domain.Folder{}, fmt.Errorf("box %s has no root folder", box)
	}
	current := folders[0] // FoldersOfBox puts the root first
	p = path.Clean("/" + strings.ReplaceAll(p, "\\", "/"))
	if p == "/" {
		return current, nil
	}

	for _, part := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		fold := domain.Fold(part)
		found := false
		for _, f := range folders {
			if f.ParentID != nil && *f.ParentID == current.ID && f.NameFold == fold {
				current, found = f, true
				break
			}
		}
		if !found {
			return domain.Folder{}, fmt.Errorf("%q in box %s: %w", part, box, ErrNoSuchFolder)
		}
	}
	return current, nil
}

// FilePath renders where a file sits inside its box, as a slash path.
//
// Search results need it: a bare file name is not enough to go find the thing
// under boxes/, and "IMG_0001.jpg" on its own is the least useful line a
// search could print.
//
// The path is the database's names, not the projection's. The two differ
// wherever a name had to be sanitized (spec 3.4), and the database's is the
// real one — the projected name is what that becomes on this platform.
func (l *Library) FilePath(ctx context.Context, f domain.File) (string, error) {
	folders, err := store.FoldersOfBox(l.db, f.BoxID)
	if err != nil {
		return "", err
	}
	byID := make(map[domain.FolderID]domain.Folder, len(folders))
	for _, fo := range folders {
		byID[fo.ID] = fo
	}

	parts := []string{f.Name}
	for id := f.FolderID; ; {
		fo, ok := byID[id]
		if !ok || fo.ParentID == nil {
			break
		}
		parts = append(parts, fo.Name)
		id = *fo.ParentID
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return path.Join(parts...), nil
}

// ResolveTagRefs turns tag names or ids into one condition each, expanded to
// include descendants.
//
// This is step 1 of spec 4.3, and it is deliberately a memory-tree walk rather
// than a closure table: the tree is a few thousand nodes and already resident,
// while a closure table would turn "move a tag" into a library-wide rewrite.
func (l *Library) ResolveTagRefs(refs []string) ([][]domain.TagID, error) {
	out := make([][]domain.TagID, 0, len(refs))
	for _, ref := range refs {
		tag, err := l.ResolveTag(ref)
		if err != nil {
			return nil, err
		}
		out = append(out, l.tags.Descendants(tag.ID))
	}
	return out, nil
}

// TagFiles attaches one tag to many files in a single op.
//
// Only files that would actually change go into the op. A file that already
// carries the tag is left out, or the inverse would strip a tag this op never
// added — the same rule M1b established for boxes.
func (l *Library) TagFiles(ctx context.Context, tag domain.TagID, files []domain.FileID) (int, error) {
	changed, err := l.filterFileTags(ctx, tag, files, false)
	if err != nil || len(changed) == 0 {
		return 0, err
	}
	raw, err := oplog.Marshal(oplog.TagFilesPayload{TagID: tag, FileIDs: changed, Source: "user"})
	if err != nil {
		return 0, err
	}
	if _, err := l.write(ctx, oplog.ActionTagFiles, raw, oplog.SourceUser, nil); err != nil {
		return 0, err
	}
	return len(changed), nil
}

// UntagFiles is the mirror image.
func (l *Library) UntagFiles(ctx context.Context, tag domain.TagID, files []domain.FileID) (int, error) {
	changed, err := l.filterFileTags(ctx, tag, files, true)
	if err != nil || len(changed) == 0 {
		return 0, err
	}
	raw, err := oplog.Marshal(oplog.UntagFilesPayload{TagID: tag, FileIDs: changed, Source: "user"})
	if err != nil {
		return 0, err
	}
	if _, err := l.write(ctx, oplog.ActionUntagFiles, raw, oplog.SourceUser, nil); err != nil {
		return 0, err
	}
	return len(changed), nil
}

// filterFileTags keeps the files for which the operation would actually change
// something. wantPresent is what the file must currently look like to qualify.
func (l *Library) filterFileTags(
	ctx context.Context,
	tag domain.TagID,
	files []domain.FileID,
	wantPresent bool,
) ([]domain.FileID, error) {
	if _, ok := l.tags.ByID(tag); !ok {
		return nil, fmt.Errorf("%s: %w", tag, ErrNoSuchTag)
	}
	var changed []domain.FileID
	seen := map[domain.FileID]bool{}
	err := l.readTx(ctx, func(vtx *store.ViewTx) error {
		for _, id := range files {
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, ok, err := vtx.GetFile(id); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("%s: %w", id, ErrNoSuchFile)
			}
			has, err := vtx.HasFileTag(id, tag)
			if err != nil {
				return err
			}
			if has == wantPresent {
				changed = append(changed, id)
			}
		}
		return nil
	})
	return changed, err
}
