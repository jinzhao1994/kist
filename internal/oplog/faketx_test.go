package oplog

import (
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

// fakeTx is an in-memory stand-in for the real transaction. Handlers are pure
// functions of (tx, payload), so testing them against maps rather than SQLite
// is not a shortcut — it is the shape the design promises.
type fakeTx struct {
	searches map[domain.SavedSearchID]domain.SavedSearch
	ts       time.Time
	boxes    map[domain.BoxID]domain.Box
	folders  map[domain.FolderID]domain.Folder
	tags     map[domain.TagID]domain.Tag
	files    map[domain.FileID]domain.File
	blobs    map[string]fakeBlob
	boxTags  map[[2]string]bool            // box|tag -> present
	fileTags map[[2]string]map[string]bool // file|tag -> the sources claiming it
	meta     map[[2]string]metaRow
	extracts map[[2]string]int
}

// metaRow mirrors a row of file_metadata.
type metaRow struct {
	kv        extract.KV
	source    string
	tombstone bool
}

// fakeBlob mirrors a row of the blobs index.
type fakeBlob struct {
	sha256 string
	size   int64
}

func newFakeTx(ts time.Time) *fakeTx {
	return &fakeTx{
		ts:       ts,
		boxes:    map[domain.BoxID]domain.Box{},
		folders:  map[domain.FolderID]domain.Folder{},
		tags:     map[domain.TagID]domain.Tag{},
		files:    map[domain.FileID]domain.File{},
		blobs:    map[string]fakeBlob{},
		boxTags:  map[[2]string]bool{},
		fileTags: map[[2]string]map[string]bool{},
		meta:     map[[2]string]metaRow{},
		extracts: map[[2]string]int{},
	}
}

func btKey(b domain.BoxID, t domain.TagID) [2]string { return [2]string{b.String(), t.String()} }

func ftKey(f domain.FileID, t domain.TagID) [2]string { return [2]string{f.String(), t.String()} }

func (f *fakeTx) TS() time.Time { return f.ts }

func (f *fakeTx) InsertBox(b domain.Box) error {
	if _, exists := f.boxes[b.ID]; exists {
		return fmt.Errorf("box %s already exists", b.ID)
	}
	b.CreatedAt, b.UpdatedAt = f.ts, f.ts
	f.boxes[b.ID] = b
	return nil
}

func (f *fakeTx) UpdateBox(b domain.Box) error {
	old, exists := f.boxes[b.ID]
	if !exists {
		return fmt.Errorf("no such box %s", b.ID)
	}
	b.CreatedAt = old.CreatedAt
	b.UpdatedAt = f.ts
	f.boxes[b.ID] = b
	return nil
}

func (f *fakeTx) DeleteBox(id domain.BoxID) error {
	delete(f.boxes, id)
	return nil
}

func (f *fakeTx) GetBox(id domain.BoxID) (domain.Box, bool, error) {
	b, ok := f.boxes[id]
	return b, ok, nil
}

func (f *fakeTx) CountBoxTags(id domain.BoxID) (int, error) {
	n := 0
	for k := range f.boxTags {
		if k[0] == id.String() {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) InsertFolder(fo domain.Folder) error {
	fo.CreatedAt, fo.UpdatedAt = f.ts, f.ts
	f.folders[fo.ID] = fo
	return nil
}

func (f *fakeTx) InsertTag(t domain.Tag) error {
	for _, existing := range f.tags {
		if existing.NameFold == t.NameFold {
			return fmt.Errorf("tag name %q is already taken", t.Name)
		}
	}
	f.tags[t.ID] = t
	return nil
}

func (f *fakeTx) UpdateTag(t domain.Tag) error {
	if _, ok := f.tags[t.ID]; !ok {
		return fmt.Errorf("no such tag %s", t.ID)
	}
	for id, existing := range f.tags {
		if id != t.ID && existing.NameFold == t.NameFold {
			return fmt.Errorf("tag name %q is already taken", t.Name)
		}
	}
	f.tags[t.ID] = t
	return nil
}

func (f *fakeTx) DeleteTag(id domain.TagID) error { delete(f.tags, id); return nil }

func (f *fakeTx) GetTag(id domain.TagID) (domain.Tag, bool, error) {
	t, ok := f.tags[id]
	return t, ok, nil
}

func (f *fakeTx) CountTagChildren(id domain.TagID) (int, error) {
	n := 0
	for _, t := range f.tags {
		if t.ParentID != nil && *t.ParentID == id {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) CountTagUsage(id domain.TagID) (int, error) {
	n := 0
	for k := range f.fileTags {
		if k[1] == id.String() {
			n++
		}
	}
	for k := range f.boxTags {
		if k[1] == id.String() {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) AddBoxTag(b domain.BoxID, t domain.TagID) error {
	f.boxTags[btKey(b, t)] = true
	return nil
}

func (f *fakeTx) RemoveBoxTag(b domain.BoxID, t domain.TagID) error {
	delete(f.boxTags, btKey(b, t))
	return nil
}

func (f *fakeTx) HasBoxTag(b domain.BoxID, t domain.TagID) (bool, error) {
	_, ok := f.boxTags[btKey(b, t)]
	return ok, nil
}

// CountBoxFiles walks down from the box's root, the way the real one does:
// a file names only its folder, so its box is whichever root it descends from.
func (f *fakeTx) CountBoxFiles(id domain.BoxID) (int, error) {
	box, ok := f.boxes[id]
	if !ok {
		return 0, nil
	}
	inBox := map[domain.FolderID]bool{box.RootFolderID: true}
	for grew := true; grew; {
		grew = false
		for _, fo := range f.folders {
			if fo.ParentID != nil && inBox[*fo.ParentID] && !inBox[fo.ID] {
				inBox[fo.ID] = true
				grew = true
			}
		}
	}
	n := 0
	for _, file := range f.files {
		if inBox[file.FolderID] {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) GetFolder(id domain.FolderID) (domain.Folder, bool, error) {
	fo, ok := f.folders[id]
	return fo, ok, nil
}

func (f *fakeTx) DeleteFolder(id domain.FolderID) error { delete(f.folders, id); return nil }

func (f *fakeTx) CountFolderFiles(id domain.FolderID) (int, error) {
	n := 0
	for _, file := range f.files {
		if file.FolderID == id {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) CountFolderChildren(id domain.FolderID) (int, error) {
	n := 0
	for _, fo := range f.folders {
		if fo.ParentID != nil && *fo.ParentID == id {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) InsertFile(file domain.File) error {
	if _, exists := f.files[file.ID]; exists {
		return fmt.Errorf("file %s already exists", file.ID)
	}
	if _, ok := f.blobs[file.BlobKey]; !ok {
		return fmt.Errorf("file %s references unknown blob %s", file.Name, file.BlobKey)
	}
	for _, existing := range f.files {
		if existing.FolderID == file.FolderID && existing.NameFold == file.NameFold {
			return fmt.Errorf("%q already exists in that folder", file.Name)
		}
	}
	file.CreatedAt, file.UpdatedAt = f.ts, f.ts
	f.files[file.ID] = file
	return nil
}

func (f *fakeTx) DeleteFile(id domain.FileID) error { delete(f.files, id); return nil }

func (f *fakeTx) GetFile(id domain.FileID) (domain.File, bool, error) {
	file, ok := f.files[id]
	return file, ok, nil
}

func (f *fakeTx) CountFileTags(id domain.FileID) (int, error) {
	n := 0
	for k := range f.fileTags {
		if k[0] == id.String() {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) UpsertBlob(key, sha256 string, size int64) error {
	f.blobs[key] = fakeBlob{sha256: sha256, size: size}
	return nil
}

func (f *fakeTx) AddFileTag(file domain.FileID, t domain.TagID, source string) error {
	k := ftKey(file, t)
	if f.fileTags[k] == nil {
		f.fileTags[k] = map[string]bool{}
	}
	f.fileTags[k][source] = true
	return nil
}

func (f *fakeTx) RemoveFileTag(file domain.FileID, t domain.TagID, source string) error {
	k := ftKey(file, t)
	delete(f.fileTags[k], source)
	if len(f.fileTags[k]) == 0 {
		delete(f.fileTags, k)
	}
	return nil
}

func (f *fakeTx) HasFileTag(file domain.FileID, t domain.TagID) (bool, error) {
	_, ok := f.fileTags[ftKey(file, t)]
	return ok, nil
}

// compile-time proof that the fake covers the whole interface
var _ Tx = (*fakeTx)(nil)

func (f *fakeTx) SetFileMetadata(file domain.FileID, kv extract.KV, source string) error {
	if _, ok := f.files[file]; !ok {
		return fmt.Errorf("no such file %s", file)
	}
	f.meta[[2]string{file.String(), kv.Key}] = metaRow{kv: kv, source: source}
	return nil
}

func (f *fakeTx) TombstoneFileMetadata(file domain.FileID, key string) error {
	if _, ok := f.files[file]; !ok {
		return fmt.Errorf("no such file %s", file)
	}
	f.meta[[2]string{file.String(), key}] = metaRow{source: extract.UserSource, tombstone: true}
	return nil
}

func (f *fakeTx) GetFileMetadata(file domain.FileID, key string) (extract.Row, bool, error) {
	r, ok := f.meta[[2]string{file.String(), key}]
	if !ok {
		return extract.Row{}, false, nil
	}
	return extract.Row{KV: r.kv, Source: r.source, Tombstone: r.tombstone}, true, nil
}

func (f *fakeTx) DeleteFileMetadata(file domain.FileID, key string) error {
	delete(f.meta, [2]string{file.String(), key})
	return nil
}

func (f *fakeTx) SetFileExtraction(file domain.FileID, extractor string, version int) error {
	f.extracts[[2]string{file.String(), extractor}] = version
	return nil
}

func (f *fakeTx) DeleteFileExtraction(file domain.FileID, extractor string) error {
	delete(f.extracts, [2]string{file.String(), extractor})
	return nil
}

func (f *fakeTx) SetCapturedAt(file domain.FileID, at domain.Instant) error {
	got, ok := f.files[file]
	if !ok {
		return fmt.Errorf("no such file %s", file)
	}
	got.CapturedAt = at
	f.files[file] = got
	return nil
}

// Composite archive handlers use real SQLite in library's integration tests.
func (*fakeTx) BoxContents(domain.BoxID) (domain.BoxContents, bool, error) {
	return domain.BoxContents{}, false, fmt.Errorf("box archive is not implemented by fakeTx")
}
func (*fakeTx) DeleteBoxContents(domain.BoxContents) error {
	return fmt.Errorf("box archive is not implemented by fakeTx")
}
func (*fakeTx) RestoreBoxContents(domain.BoxContents) error {
	return fmt.Errorf("box archive is not implemented by fakeTx")
}

func (f *fakeTx) GetSavedSearch(id domain.SavedSearchID) (domain.SavedSearch, bool, error) {
	s, ok := f.searches[id]
	return s, ok, nil
}
func (f *fakeTx) InsertSavedSearch(s domain.SavedSearch) error {
	if f.searches == nil {
		f.searches = map[domain.SavedSearchID]domain.SavedSearch{}
	}
	if _, ok := f.searches[s.ID]; ok {
		return fmt.Errorf("duplicate search")
	}
	f.searches[s.ID] = s
	return nil
}
func (f *fakeTx) UpdateSavedSearch(s domain.SavedSearch) error    { f.searches[s.ID] = s; return nil }
func (f *fakeTx) DeleteSavedSearch(id domain.SavedSearchID) error { delete(f.searches, id); return nil }

func (f *fakeTx) SavedSearchByName(name string) (domain.SavedSearch, bool, error) {
	for _, s := range f.searches {
		if domain.Fold(s.Name) == domain.Fold(name) {
			return s, true, nil
		}
	}
	return domain.SavedSearch{}, false, nil
}

func (*fakeTx) EntryContents(domain.BoxID, domain.ID, bool) (domain.EntryContents, bool, error) {
	panic("not implemented")
}
func (*fakeTx) DeleteEntryContents(domain.EntryContents) error  { panic("not implemented") }
func (*fakeTx) RestoreEntryContents(domain.EntryContents) error { panic("not implemented") }
func (*fakeTx) RenameEntry(domain.ID, bool, string) error       { panic("not implemented") }

func (*fakeTx) EntryNameAvailable(domain.FolderID, string, domain.ID) (bool, error) {
	panic("not implemented")
}
