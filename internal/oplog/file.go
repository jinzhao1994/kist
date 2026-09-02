package oplog

import (
	"fmt"
	"strconv"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

func init() {
	Register(ActionImportFiles, importFiles{})
	Register(ActionDeleteFiles, deleteFiles{})
}

// ImportedFolder is a folder this op creates.
//
// Parents come before children in the slice; library guarantees that ordering
// when it builds the payload, so Apply can insert them in order without
// sorting — and therefore without making a decision (invariant 3).
type ImportedFolder struct {
	FolderID domain.FolderID `json:"folder_id"`
	ParentID domain.FolderID `json:"parent_id"`
	Name     string          `json:"name"`
}

// ImportedFile is one file this op adds.
//
// Sha256 and Size are here so that Apply can maintain the blobs index without
// touching the filesystem. That keeps blobs rebuildable from the log as well
// as from a directory scan, and it means Apply never has to read anything it
// was not handed (invariant 3).
type ImportedFile struct {
	FileID   domain.FileID   `json:"file_id"`
	FolderID domain.FolderID `json:"folder_id"`
	Name     string          `json:"name"`
	BlobKey  string          `json:"blob_key"`
	Sha256   string          `json:"sha256"`
	Size     int64           `json:"size"`
	// MTime is the source file's modification time, in unix nanoseconds.
	//
	// It is captured here because it cannot be captured anywhere else: once
	// the content is in blobs/, the stored copy's mtime is when it was
	// written. For a PDF, a scan or a voice memo it is often the only time
	// signal there is (spec 1.7), and it is what captured_at falls back to.
	MTime int64 `json:"mtime,omitempty"`
	// CapturedAt is the kernel's conclusion at import time (spec 1.5.2).
	//
	// It is resolved when the op is generated, not inside Apply: the
	// resolution needs the library's timezone, which is configuration rather
	// than payload, and a decision that reads configuration during a replay
	// would break invariant 3. Extraction refines it later.
	CapturedAt domain.Instant `json:"captured_at,omitempty"`
}

// ImportFilesPayload is one import: one box, the folders it had to create, and
// the files that actually made it in.
//
// Files that failed are absent — they never enter the payload at all, which is
// what lets a partly failed import still be one atomic op (spec 3.7.1 ③). The
// same is true of an interrupted one: it commits the files it finished
// (spec 3.7.1 ①).
type ImportFilesPayload struct {
	BoxID   domain.BoxID     `json:"box_id"`
	Folders []ImportedFolder `json:"folders"`
	Files   []ImportedFile   `json:"files"`
}

// DeleteFilesPayload is the same shape, because deleting must record enough to
// put everything back — including the folder ids, since a re-created folder
// with a fresh id would orphan the projection and every later import that
// targeted it.
type DeleteFilesPayload = ImportFilesPayload

type importFiles struct{}

func (importFiles) Apply(tx Tx, payload []byte) error {
	p, err := Payload[ImportFilesPayload](payload)
	if err != nil {
		return err
	}
	// Blobs first: files reference them, and the foreign key is on.
	for _, f := range p.Files {
		if err := tx.UpsertBlob(f.BlobKey, f.Sha256, f.Size); err != nil {
			return err
		}
	}
	// Folders in payload order, which library built parents-first.
	for _, d := range p.Folders {
		folder, err := domain.NewFolder(d.FolderID, p.BoxID, d.ParentID, d.Name)
		if err != nil {
			return err
		}
		if err := tx.InsertFolder(folder); err != nil {
			return err
		}
	}
	for _, f := range p.Files {
		file, err := domain.NewFile(f.FileID, p.BoxID, f.FolderID, f.Name, f.BlobKey)
		if err != nil {
			return err
		}
		file.CapturedAt = f.CapturedAt
		if err := tx.InsertFile(file); err != nil {
			return err
		}
		if err := applyFileFacts(tx, f); err != nil {
			return err
		}
	}
	return nil
}

// applyFileFacts records what the file itself said about itself.
//
// Under extract.ImportSource rather than an extractor id, because nothing can
// re-derive these: a later run would read the blob's mtime, which is when kist
// wrote it. Marking them as some extractor's would invite exactly that re-run.
func applyFileFacts(tx Tx, f ImportedFile) error {
	facts := []extract.KV{
		{Key: "file:size", Value: strconv.FormatInt(f.Size, 10), Type: extract.TypeInt},
	}
	if f.MTime != 0 {
		facts = append(facts, extract.KV{
			Key: "file:mtime", Value: strconv.FormatInt(f.MTime, 10), Type: extract.TypeInt,
		})
	}
	for _, kv := range facts {
		if err := tx.SetFileMetadata(f.FileID, kv, extract.ImportSource); err != nil {
			return err
		}
	}
	return nil
}

func (importFiles) Inverse(payload []byte) (Action, []byte, error) {
	return ActionDeleteFiles, payload, nil
}

func (importFiles) Targets(payload []byte) (Targets, error) { return fileTargets(payload) }

type deleteFiles struct{}

// Apply removes what an import added, refusing if anything has moved on.
//
// The checks are assertions, not decisions — they either let the op through
// unchanged or fail it (invariant 3). They are also the whole of strict undo
// for imports (spec 2.5): a half-completed undo of a 5000-file import is the
// worst possible outcome in an archive, so kist would rather name the obstacle
// and stop.
func (deleteFiles) Apply(tx Tx, payload []byte) error {
	p, err := Payload[DeleteFilesPayload](payload)
	if err != nil {
		return err
	}

	for _, f := range p.Files {
		got, ok, err := tx.GetFile(f.FileID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %q is already gone: %w", f.Name, ErrStaleBefore)
		}
		if got.Name != f.Name || got.FolderID != f.FolderID || got.BlobKey != f.BlobKey {
			return fmt.Errorf("%q has changed since it was imported: %w", f.Name, ErrStaleBefore)
		}
		n, err := tx.CountFileTags(f.FileID)
		if err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%q has been tagged since it was imported; remove its tags first: %w",
				f.Name, ErrStaleBefore)
		}
		if err := tx.DeleteFile(f.FileID); err != nil {
			return err
		}
	}

	// Children before parents, since library wrote the folders parents-first.
	for i := len(p.Folders) - 1; i >= 0; i-- {
		d := p.Folders[i]
		if _, ok, err := tx.GetFolder(d.FolderID); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("folder %q is already gone: %w", d.Name, ErrStaleBefore)
		}
		files, err := tx.CountFolderFiles(d.FolderID)
		if err != nil {
			return err
		}
		subs, err := tx.CountFolderChildren(d.FolderID)
		if err != nil {
			return err
		}
		if files > 0 || subs > 0 {
			return fmt.Errorf("folder %q is not empty; something was added to it after this import: %w",
				d.Name, ErrStaleBefore)
		}
		if err := tx.DeleteFolder(d.FolderID); err != nil {
			return err
		}
	}
	return nil
}

func (deleteFiles) Inverse(payload []byte) (Action, []byte, error) {
	return ActionImportFiles, payload, nil
}

func (deleteFiles) Targets(payload []byte) (Targets, error) { return fileTargets(payload) }

// fileTargets is shared by both directions: the same payload names the same
// box and the same files whichever way it is being applied.
func fileTargets(payload []byte) (Targets, error) {
	p, err := Payload[ImportFilesPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	t := Targets{Boxes: []domain.BoxID{p.BoxID}}
	for _, f := range p.Files {
		t.Files = append(t.Files, f.FileID)
		t.Blobs = append(t.Blobs, f.BlobKey)
	}
	return t, nil
}
