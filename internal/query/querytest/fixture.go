package querytest

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// Builder assembles a Fixture.
//
// Ids are derived from names rather than generated, for two reasons. A case
// can then say want: []string{"a.txt", "b.txt"} instead of listing UUIDs, and
// a failure names something a person recognises. More importantly, the
// imported ordering is id order (spec 4.5.1) — with random ids the expected
// result of every SortImported case would change on every run.
//
// Derivation keeps ids increasing in the order things are added, so "imported"
// really does mean "the order this fixture declared them".
type Builder struct {
	seq      int
	boxes    map[string]domain.Box
	folders  map[string]domain.Folder
	files    map[string]domain.File
	tags     map[string]domain.Tag
	fileTags map[domain.FileID][]domain.TagID
	order    []string // file names, in declaration order
}

func NewBuilder() *Builder {
	return &Builder{
		boxes:    map[string]domain.Box{},
		folders:  map[string]domain.Folder{},
		files:    map[string]domain.File{},
		tags:     map[string]domain.Tag{},
		fileTags: map[domain.FileID][]domain.TagID{},
	}
}

// nextID mints an id that is stable for a name and increasing with the order
// names are handed out, which is what UUIDv7 gives in production.
func (b *Builder) nextID(name string) domain.ID {
	b.seq++
	sum := sha256.Sum256([]byte(name))
	var raw [16]byte
	copy(raw[:], sum[:])
	// Overwrite the timestamp field with the sequence number so that ordering
	// is declaration order, and set the version and variant bits so the value
	// is a well-formed UUIDv7 like every other id in the library.
	binary.BigEndian.PutUint64(raw[0:8], uint64(b.seq)<<16)
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	return domain.ID(raw)
}

// Box adds a box. start and end may be empty.
func (b *Builder) Box(name, start, end string) *Builder {
	box := domain.Box{
		ID:        domain.BoxID{ID: b.nextID("box:" + name)},
		Name:      name,
		NameFold:  domain.Fold(name),
		CreatedAt: fixtureTime,
		UpdatedAt: fixtureTime,
	}
	if start != "" {
		box.StartAt = mustDate(start)
	}
	if end != "" {
		box.EndAt = mustDate(end)
	}
	// Every box has a root folder (spec 1.4), and the box is what records the
	// link to it.
	root := domain.Folder{
		ID:        domain.FolderID{ID: b.nextID("folder:" + name + "/")},
		Name:      "",
		CreatedAt: fixtureTime,
		UpdatedAt: fixtureTime,
	}
	box.RootFolderID = root.ID
	b.boxes[name] = box
	b.folders[name+"/"] = root
	return b
}

// Folder adds a directory directly under a box's root.
func (b *Builder) Folder(box, name string) *Builder {
	root := b.folders[box+"/"]
	id := domain.FolderID{ID: b.nextID("folder:" + box + "/" + name)}
	parent := root.ID
	b.folders[box+"/"+name] = domain.Folder{
		ID:        id,
		ParentID:  &parent,
		Name:      name,
		NameFold:  domain.Fold(name),
		CreatedAt: fixtureTime,
		UpdatedAt: fixtureTime,
	}
	return b
}

// File adds a file. folder is "" for the box root; captured may be empty;
// tags name tags added with Tag.
func (b *Builder) File(box, folder, name, captured string, tags ...string) *Builder {
	f := b.folders[box+"/"+folder]
	file := domain.File{
		ID:        domain.FileID{ID: b.nextID("file:" + box + "/" + folder + "/" + name)},
		FolderID:  f.ID,
		Name:      name,
		NameFold:  domain.Fold(name),
		BlobKey:   "sha256:" + fmt.Sprintf("%064x", b.seq),
		CreatedAt: fixtureTime,
		UpdatedAt: fixtureTime,
	}
	if captured != "" {
		file.CapturedAt = mustInstant(captured)
	}
	b.files[name] = file
	b.order = append(b.order, name)
	for _, tag := range tags {
		b.fileTags[file.ID] = append(b.fileTags[file.ID], b.tags[tag].ID)
	}
	return b
}

// Tag adds a tag. parent may be "" for a root tag.
func (b *Builder) Tag(name, parent string) *Builder {
	tag := domain.Tag{
		ID:       domain.TagID{ID: b.nextID("tag:" + name)},
		Name:     name,
		NameFold: domain.Fold(name),
	}
	if parent != "" {
		p := b.tags[parent].ID
		tag.ParentID = &p
	}
	b.tags[name] = tag
	return b
}

// Fixture renders what has been built.
func (b *Builder) Fixture() Fixture {
	f := Fixture{FileTags: b.fileTags}
	for _, box := range b.boxes {
		f.Boxes = append(f.Boxes, box)
	}
	// Roots first, so a backend inserting in order never sees a dangling
	// parent.
	for _, d := range b.folders {
		if d.ParentID == nil {
			f.Folders = append(f.Folders, d)
		}
	}
	for _, d := range b.folders {
		if d.ParentID != nil {
			f.Folders = append(f.Folders, d)
		}
	}
	for _, t := range b.tags {
		if t.ParentID == nil {
			f.Tags = append(f.Tags, t)
		}
	}
	for _, t := range b.tags {
		if t.ParentID != nil {
			f.Tags = append(f.Tags, t)
		}
	}
	for _, name := range b.order {
		f.Files = append(f.Files, b.files[name])
	}
	return f
}

// FileID looks up a file a case declared, for building queries and assertions.
func (b *Builder) FileID(name string) domain.FileID { return b.files[name].ID }

// BoxID looks up a box.
func (b *Builder) BoxID(name string) domain.BoxID { return b.boxes[name].ID }

// FolderID looks up a folder. Use "" for a box's root.
func (b *Builder) FolderID(box, folder string) domain.FolderID {
	return b.folders[box+"/"+folder].ID
}

// TagID looks up a tag.
func (b *Builder) TagID(name string) domain.TagID { return b.tags[name].ID }

// fixtureTime is a fixed instant. Nothing in the contract depends on it, and a
// real clock would make failures differ between runs.
var fixtureTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// mustDate is for the two places that are still variable-precision dates: a
// box's declared start and end, and the DateRange a caller asks for. A file's
// capture time is not one of them any more (spec 1.5.2).
func mustDate(s string) domain.LocalDate {
	d, err := domain.ParseLocalDate(s)
	if err != nil {
		panic("querytest: bad fixture date " + s + ": " + err.Error())
	}
	return d
}

// mustInstant turns a fixture's human-readable date into the instant the
// column actually holds.
//
// Cases stay written as "2019-08-12" because that is what a reader of the
// suite needs to see; the conversion is pinned to UTC so a fixture means the
// same thing on every machine. Which zone it is does not matter to any case —
// they all assert relative order — but that it is *fixed* does (spec 4.7).
func mustInstant(s string) domain.Instant {
	d, err := domain.ParseLocalDate(s)
	if err != nil {
		panic("querytest: bad fixture date " + s + ": " + err.Error())
	}
	start, err := time.Parse("2006-01-02T15:04", d.Start())
	if err != nil {
		panic("querytest: bad fixture date " + s + ": " + err.Error())
	}
	return domain.NewInstant(start.UnixNano())
}

// names renders a result for a failure message.
func names(files []domain.File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name
	}
	return out
}

// requireNames asserts the exact result, in order.
func requireNames(t *testing.T, got []domain.File, want ...string) {
	t.Helper()
	gotNames := names(got)
	if len(gotNames) != len(want) {
		t.Fatalf("got %v, want %v", gotNames, want)
	}
	for i := range want {
		if gotNames[i] != want[i] {
			t.Fatalf("got %v, want %v", gotNames, want)
		}
	}
}

// requireSet asserts the result regardless of order, for cases about which
// rows match rather than how they are ordered.
func requireSet(t *testing.T, got []domain.File, want ...string) {
	t.Helper()
	have := map[string]bool{}
	for _, f := range got {
		have[f.Name] = true
	}
	if len(have) != len(want) {
		t.Fatalf("got %v, want %v", names(got), want)
	}
	for _, w := range want {
		if !have[w] {
			t.Fatalf("got %v, want %v", names(got), want)
		}
	}
}
