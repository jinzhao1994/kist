package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// FileStatus is what happened to one file.
type FileStatus int

const (
	// FileImported: the file is now in the library.
	FileImported FileStatus = iota
	// FileSkipped: this folder already holds a file with this name and the
	// same content, so there was nothing to do (spec 3.7.1 ②).
	FileSkipped
	// FileFailed: this one file did not make it. The rest of the batch did
	// (spec 3.7.1 ③).
	FileFailed
)

func (s FileStatus) String() string {
	switch s {
	case FileImported:
		return "imported"
	case FileSkipped:
		return "skipped"
	default:
		return "failed"
	}
}

// FileResult is the per-file outcome.
//
// Import returns one of these for every file it looked at, which is what lets
// a batch report "4998 in, 1 skipped, 1 unreadable" instead of a single yes or
// no (spec 3.7.1 ③).
type FileResult struct {
	Source string // where it came from on disk
	Rel    string // where it went inside the box, slash-separated
	Name   string // the name it ended up with, after any numbering
	Status FileStatus
	// Outcome is what blob storage did. Deduped means the content was already
	// there; Collided and Repaired are events worth showing a person rather
	// than folding into "imported" (spec 3.3).
	Outcome blob.Outcome
	Err     error
}

// ImportResult is one import.
type ImportResult struct {
	// OpID is the single op this import produced. Zero when nothing was
	// imported — an op with no files would be an undo that does nothing.
	OpID  int64
	Files []FileResult
	// Interrupted is true when the caller's context was cancelled partway.
	// The files already processed are committed; the rest were never touched
	// (spec 3.7.1 ①).
	Interrupted bool
	// Remaining counts the candidates never looked at because of that.
	Remaining int
}

// Count returns how many results have the given status.
func (r *ImportResult) Count(s FileStatus) int {
	n := 0
	for _, f := range r.Files {
		if f.Status == s {
			n++
		}
	}
	return n
}

// ImportOptions configures one import.
type ImportOptions struct {
	// Box must already exist. Import never creates one and never infers one
	// (spec 3.7).
	Box domain.BoxID
	// Into is the folder to import under. Zero means the box's root.
	Into domain.FolderID
	// Hidden includes dot-files. Off by default: .DS_Store, Thumbs.db and
	// desktop.ini are operating-system droppings, not archive material, and an
	// archive that quietly accumulates one per directory forever is worse than
	// one that made you pass a flag.
	Hidden bool
	// RootOp, when set, joins this import's op to a group headed by that op —
	// `kist import a/ b/ c/` is three ops today, and will be more once a large
	// import is written in chunks. It changes nothing about what is imported.
	RootOp *int64
}

// scanned is one candidate found on disk, before anything has been decided.
type scanned struct {
	abs string
	// rel is the path relative to the import root, slash-separated so the
	// folder chain comes out the same on every platform.
	rel string
	err error
}

// scan walks src and returns the candidates in a deterministic order.
//
// Determinism matters more than it looks: the numbering of conflicting names
// depends on the order files are seen in, so a non-deterministic walk would
// make the same import produce different names on different runs.
// filepath.WalkDir sorts each directory's entries, which is exactly what is
// needed.
func scan(src string, hidden bool) ([]scanned, error) {
	info, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", src, err)
	}
	if !info.IsDir() {
		return []scanned{{abs: src, rel: filepath.Base(src)}}, nil
	}

	var out []scanned
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// One unreadable directory does not end the batch; it becomes a
			// per-file failure like any other (spec 3.7.1 ③).
			out = append(out, scanned{abs: p, rel: filepath.Base(p), err: err})
			return nil
		}
		if p == src {
			return nil
		}
		if !hidden && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// Folders are created lazily, when a file actually lands in one.
			// An empty source directory therefore leaves no trace, which keeps
			// the payload to what was really used and keeps undo's "the folder
			// must be empty" assertion from tripping over empty shells.
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		s := scanned{abs: p, rel: filepath.ToSlash(rel)}
		if !d.Type().IsRegular() {
			s.err = fmt.Errorf("not a regular file (%s)", d.Type())
		}
		out = append(out, s)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", src, err)
	}
	return out, nil
}

// Import puts a file or a directory tree into an existing box.
//
// The shape follows spec 3.7: content goes to blob storage first, one file at
// a time and outside any transaction, and only then does a single transaction
// append one op describing everything that made it in. A crash between the two
// leaves an orphan blob, which costs space and nothing else (spec 3.9).
//
// Cancelling ctx is a graceful stop, not an abort: the loop stops taking new
// files and the ones already processed are committed as one op (spec 3.7.1 ①).
// That is also why the commit runs on a context.WithoutCancel — the context
// that told us to stop cannot be the one we ask to write with.
func (l *Library) Import(ctx context.Context, src string, opt ImportOptions) (*ImportResult, error) {
	prepared, err := l.PrepareImport(ctx, src, opt)
	if err != nil {
		return nil, err
	}
	// CLI imports preserve the completed portion on graceful cancellation.
	return prepared.Commit(context.WithoutCancel(ctx))
}

// PreparedImport holds immutable input for one import commit. Preparing stores
// blobs but does not append history or change views; abandoning it can only
// leave unreferenced blobs, which the existing integrity tools report.
type PreparedImport struct {
	library *Library
	payload []byte
	result  ImportResult
	rootOp  *int64
}

// Result returns preparation outcomes. Imported files are ready to commit, not
// yet visible in the library. Callers must inspect Interrupted and FileFailed
// before deciding whether their protocol permits committing a partial batch.
func (p *PreparedImport) Result() ImportResult {
	r := p.result
	r.Files = append([]FileResult(nil), r.Files...)
	return r
}

// Commit appends the prepared batch using the caller's cancellation policy.
// ExecuteRequest supplies retry protection; outside it a stale or repeated plan
// is rejected by the normal write-path assertions and constraints.
func (p *PreparedImport) Commit(ctx context.Context) (*ImportResult, error) {
	res := p.Result()
	if len(p.payload) != 0 {
		id, err := p.library.write(ctx, oplog.ActionImportFiles, p.payload, oplog.OriginUser, writeOptions{RootOpID: p.rootOp})
		if err != nil {
			return nil, err
		}
		res.OpID = id
	}
	return &res, nil
}

// PrepareImport scans and hashes outside the commit lock. HTTP callers first
// receive the complete upload, prepare it here, then commit within ExecuteRequest.
func (l *Library) PrepareImport(ctx context.Context, src string, opt ImportOptions) (*PreparedImport, error) {
	if ctx.Value(requestContextKey{}) != nil {
		return nil, ErrRequestScope
	}

	// Import is the only thing that writes to blobs/tmp, so it is also the
	// only thing that leaves residue there when it crashes. Clearing it here
	// costs one ReadDir and keeps the mess bounded by one interrupted run
	// rather than by how long it has been since somebody ran verify (spec 3.9).
	l.cleanTmp()

	box, ok, err := l.Box(ctx, opt.Box)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoSuchBox, opt.Box)
	}

	folders, err := store.FoldersOfBox(l.db, box.ID)
	if err != nil {
		return nil, err
	}
	tree, err := newImportTree(box.ID, folders, opt.Into)
	if err != nil {
		return nil, err
	}
	taken, err := store.ExistingNames(l.db, box.ID)
	if err != nil {
		return nil, err
	}

	candidates, err := scan(src, opt.Hidden)
	if err != nil {
		return nil, err
	}

	res := &ImportResult{}
	payload := oplog.ImportFilesPayload{BoxID: box.ID}

	for i, c := range candidates {
		if ctx.Err() != nil {
			res.Interrupted = true
			res.Remaining = len(candidates) - i
			break
		}
		r := l.importOne(ctx, c, tree, taken, &payload)
		// The file in flight when the signal arrived is not a failure — the
		// user pressed Ctrl-C. Reporting it as one puts a frightening
		// "failed: context canceled" line in front of somebody who did exactly
		// what they meant to, and makes "I stopped it" indistinguishable from
		// "something is wrong with my files". It counts as not looked at.
		if r.Status == FileFailed && errors.Is(r.Err, context.Canceled) {
			res.Interrupted = true
			res.Remaining = len(candidates) - i
			break
		}
		res.Files = append(res.Files, r)
		if l.hooks.afterFile != nil {
			l.hooks.afterFile()
		}
	}

	prepared := &PreparedImport{library: l, result: *res}
	if opt.RootOp != nil {
		id := *opt.RootOp
		prepared.rootOp = &id
	}
	if len(payload.Files) > 0 || len(payload.Folders) > 0 {
		raw, err := oplog.Marshal(payload)
		if err != nil {
			return nil, err
		}
		prepared.payload = raw
	}
	return prepared, nil
}

// importOne processes a single candidate.
//
// It never returns an error: a failure here is this file's outcome, not the
// batch's (spec 3.7.1 ③), and a failed file never enters the payload — which
// is what keeps the op atomic even though the batch is not.
func (l *Library) importOne(
	ctx context.Context,
	c scanned,
	tree *importTree,
	taken map[domain.FolderID]map[string]string,
	payload *oplog.ImportFilesPayload,
) FileResult {
	r := FileResult{Source: c.abs, Rel: c.rel, Name: path.Base(c.rel), Status: FileFailed}
	if c.err != nil {
		r.Err = c.err
		return r
	}
	if err := domain.ValidateName(r.Name); err != nil {
		// Trailing dots and edge whitespace are dropped or refused by Windows,
		// so a library holding them would not project the same way on every
		// platform (spec 3.4). Renaming it silently would be worse: the user
		// would never learn their file is not called what they think.
		r.Err = fmt.Errorf("%w — rename it at the source and import again", err)
		return r
	}

	folder, err := tree.folderFor(path.Dir(c.rel), payload)
	if err != nil {
		r.Err = err
		return r
	}

	f, err := os.Open(c.abs)
	if err != nil {
		r.Err = err
		return r
	}
	defer f.Close()

	// The source file's own facts, read while the source still exists. Once
	// the content is in blobs/ this is gone: the stored copy's mtime is when
	// kist wrote it, so "capture it now or never" is a property of the fact
	// rather than a choice about where to put the code (spec 1.7).
	var mtime int64
	if info, err := f.Stat(); err == nil {
		mtime = info.ModTime().UnixNano()
	}

	put, err := l.blobs.Put(ctx, f)
	if err != nil {
		r.Err = err
		return r
	}
	r.Outcome = put.Outcome

	name, skip := resolveName(taken, folder, r.Name, string(put.Key))
	if skip {
		r.Status = FileSkipped
		return r
	}
	r.Name = name

	payload.Files = append(payload.Files, oplog.ImportedFile{
		FileID:     domain.NewFileID(),
		FolderID:   folder,
		Name:       name,
		BlobKey:    string(put.Key),
		Sha256:     put.Key.Hex(),
		Size:       put.Size,
		MTime:      mtime,
		CapturedAt: l.capturedFromFacts(put.Size, mtime),
	})
	r.Status = FileImported
	return r
}

// resolveName applies spec 3.7.1 ②: same name and same content is a skip, same
// name and different content is numbered.
//
// It also records the name it hands out, because the next file in this same
// batch has to see it — the database will not, until the transaction at the
// end.
func resolveName(
	taken map[domain.FolderID]map[string]string,
	folder domain.FolderID,
	name, key string,
) (string, bool) {
	if taken[folder] == nil {
		taken[folder] = map[string]string{}
	}
	inUse := taken[folder]

	candidate := name
	for n := 2; ; n++ {
		existing, clash := inUse[domain.Fold(candidate)]
		if !clash {
			inUse[domain.Fold(candidate)] = key
			return candidate, false
		}
		if existing == key {
			return candidate, true
		}
		candidate = domain.NumberedName(name, n)
	}
}

// importTree resolves a relative directory path to a folder id, reusing what
// is already there and minting ids for what is not.
//
// Every decision it makes happens here, while the op is being built — never
// inside Apply (invariant 3). By the time the payload is written, "which
// folder does this file go in" has exactly one answer recorded in it.
type importTree struct {
	box  domain.BoxID
	base domain.FolderID
	// byPath maps a slash path relative to base onto a folder id. "." is base.
	byPath map[string]domain.FolderID
}

func newImportTree(box domain.BoxID, folders []domain.Folder, into domain.FolderID) (*importTree, error) {
	byID := map[domain.FolderID]domain.Folder{}
	var root domain.FolderID
	for _, f := range folders {
		byID[f.ID] = f
		if f.ParentID == nil {
			root = f.ID
		}
	}
	base := into
	if base.IsZero() {
		base = root
	}
	if _, ok := byID[base]; !ok {
		return nil, fmt.Errorf("folder %s is not in box %s: %w", base, box, ErrFolderNotInBox)
	}

	t := &importTree{box: box, base: base, byPath: map[string]domain.FolderID{".": base}}
	// Index every existing descendant of base by its path relative to base, so
	// a re-run of the same import finds the folders the first run created.
	for id := range byID {
		if rel, ok := relPathOf(byID, base, id); ok {
			t.byPath[rel] = id
		}
	}
	return t, nil
}

// relPathOf walks from id up to base, returning the slash path between them.
// It reports false when id is not a descendant of base.
func relPathOf(byID map[domain.FolderID]domain.Folder, base, id domain.FolderID) (string, bool) {
	var parts []string
	for cur := id; ; {
		if cur == base {
			if len(parts) == 0 {
				return ".", true
			}
			// parts were collected leaf-first.
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
			return strings.Join(parts, "/"), true
		}
		f, ok := byID[cur]
		if !ok || f.ParentID == nil {
			return "", false
		}
		parts = append(parts, f.Name)
		cur = *f.ParentID
	}
}

// folderFor returns the folder for a slash path, appending any folders it has
// to create to the payload. Parents are appended before children, which is the
// order Apply relies on.
func (t *importTree) folderFor(rel string, payload *oplog.ImportFilesPayload) (domain.FolderID, error) {
	rel = path.Clean(rel)
	if id, ok := t.byPath[rel]; ok {
		return id, nil
	}
	parent, err := t.folderFor(path.Dir(rel), payload)
	if err != nil {
		return domain.FolderID{}, err
	}
	name := path.Base(rel)
	if err := domain.ValidateName(name); err != nil {
		return domain.FolderID{}, fmt.Errorf("directory %q: %w", name, err)
	}
	id := domain.NewFolderID()
	payload.Folders = append(payload.Folders, oplog.ImportedFolder{
		FolderID: id, ParentID: parent, Name: name,
	})
	t.byPath[rel] = id
	return id, nil
}

// capturedFromFacts resolves a capture time from what the import knows.
//
// At import there is no EXIF yet — extraction is a separate op — so this is
// only ever the modification time. That is a poor answer for a photograph and
// the only one there is for a scan, a contract or a voice memo, which is most
// of what an archive holds (spec 1.7).
//
// It runs here, while the op is being generated, because the resolution needs
// the library's timezone: configuration is not payload, and a decision that
// read configuration during a replay would break invariant 3. `kist extract`
// overwrites this the moment it finds something better.
func (l *Library) capturedFromFacts(size, mtime int64) domain.Instant {
	if mtime == 0 {
		return domain.Instant{}
	}
	facts := []extract.KV{
		{Key: "file:size", Value: strconv.FormatInt(size, 10), Type: extract.TypeInt},
		{Key: "file:mtime", Value: strconv.FormatInt(mtime, 10), Type: extract.TypeInt},
	}
	c := extract.ResolveCaptured(facts, l.zone())
	if !c.Known {
		return domain.Instant{}
	}
	return domain.NewInstant(c.UnixNano)
}
