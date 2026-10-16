package library

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// UploadConflicts is a preview of the exact destinations a batch may replace.
type UploadConflicts struct {
	Token        string   `json:"token"`
	Paths        []string `json:"paths"`
	CanOverwrite bool     `json:"can_overwrite"`
	entries      map[string]domain.EntryContents
}

// PreviewUpload binds confirmation to existing file state and absent names.
func (l *Library) PreviewUpload(ctx context.Context, box domain.BoxID, into domain.FolderID, paths []string) (UploadConflicts, error) {
	result := UploadConflicts{Paths: []string{}, CanOverwrite: true, entries: map[string]domain.EntryContents{}}
	paths = append([]string(nil), paths...)
	sort.Strings(paths)
	snapshots := map[string]any{}
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		folders, err := store.FoldersOfBox(tx.Queryer(), box)
		if err != nil {
			return err
		}
		tree, err := newImportTree(box, folders, into)
		if err != nil {
			return err
		}
		files, err := store.FilesOfBox(tx.Queryer(), box)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, rel := range paths {
			if rel == "" || strings.Contains(rel, "\\") || strings.HasPrefix(rel, "/") || path.Clean(rel) != rel {
				return ErrInvalidRequest
			}
			for _, part := range strings.Split(rel, "/") {
				if err := domain.ValidateName(part); err != nil {
					return err
				}
			}
			if seen[domain.Fold(rel)] {
				return ErrInvalidRequest
			}
			seen[domain.Fold(rel)] = true
			// Refuse a file used as an intermediate directory; neither policy deletes subtrees.
			for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
				if at, ok := tree.byPath[path.Dir(parent)]; ok {
					for _, f := range files {
						if f.FolderID == at && f.NameFold == domain.Fold(path.Base(parent)) {
							return ErrUploadTargetChanged
						}
					}
				}
			}
			folder, exists := tree.byPath[path.Dir(rel)]
			snapshots[rel] = folder
			if !exists {
				continue
			}
			for _, f := range files {
				if f.FolderID == folder && f.NameFold == domain.Fold(path.Base(rel)) {
					entry, found, err := tx.EntryContents(box, f.ID.ID, false)
					if err != nil {
						return err
					}
					if !found {
						return ErrNoSuchFile
					}
					snapshots[rel] = entry
					result.entries[rel] = entry
					result.Paths = append(result.Paths, rel)
				}
			}
			for _, f := range folders {
				if f.ParentID != nil && *f.ParentID == folder && domain.Fold(f.Name) == domain.Fold(path.Base(rel)) {
					snapshots[rel] = f.ID
					result.Paths = append(result.Paths, rel)
					result.CanOverwrite = false
				}
			}
		}
		for rel := range seen {
			for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
				if seen[parent] {
					return ErrInvalidRequest
				}
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	raw, err := json.Marshal(snapshots)
	if err != nil {
		return result, err
	}
	result.Token = fmt.Sprintf("%x", sha256.Sum256(raw))
	return result, nil
}

func (l *Library) replacementAfter(before domain.EntryContents, file oplog.ImportedFile) domain.EntryContents {
	after := before
	archived := before.Files[0]
	archived.File.BlobKey = file.BlobKey
	archived.File.CapturedAt = file.CapturedAt
	archived.SHA256 = file.Sha256
	archived.Size = file.Size
	archived.Extractions = map[string]int{}
	archived.Tags = nil
	for _, tag := range before.Files[0].Tags {
		if tag.Source == extract.UserSource {
			archived.Tags = append(archived.Tags, tag)
		}
	}
	archived.Metadata = nil
	userKeys := map[string]bool{}
	for _, m := range before.Files[0].Metadata {
		if m.Source == extract.UserSource {
			archived.Metadata = append(archived.Metadata, m)
			userKeys[m.Key] = true
		}
	}
	kind := string(extract.TypeInt)
	size := strconv.FormatInt(file.Size, 10)
	if !userKeys["file:size"] {
		archived.Metadata = append(archived.Metadata, domain.ArchivedMetadata{Key: "file:size", Value: &size, Type: &kind, Source: extract.ImportSource})
	}
	if file.MTime != 0 && !userKeys["file:mtime"] {
		mtime := strconv.FormatInt(file.MTime, 10)
		archived.Metadata = append(archived.Metadata, domain.ArchivedMetadata{Key: "file:mtime", Value: &mtime, Type: &kind, Source: extract.ImportSource})
	}
	sort.Slice(archived.Metadata, func(i, j int) bool { return archived.Metadata[i].Key < archived.Metadata[j].Key })
	facts := []extract.KV{}
	for _, m := range archived.Metadata {
		if m.Value != nil && m.Type != nil {
			facts = append(facts, extract.KV{Key: m.Key, Value: *m.Value, Type: extract.Type(*m.Type)})
		}
	}
	archived.File.CapturedAt = l.resolveCaptured(facts)
	after.Files = []domain.ArchivedFile{archived}
	return after
}
