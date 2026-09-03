// Package wire is the JSON shape of a kist library.
//
// It exists so that `kist find --json` and the HTTP API return the same thing,
// defined once (spec 2.8). Both cmd and api import it; it imports nothing but
// domain, which internal/arch's test enforces — the same shape as oplog and
// query, and for the same reason: a contract that lives inside the layer that
// produces it is out of reach of the second consumer.
//
// Two rules the types below follow.
//
// Folded columns never appear. NameFold is how spec 4.7 gets case-insensitive
// matching without asking the database for a collation; it is a storage
// decision, and a client that saw it would be tempted to match on it.
//
// Ids are strings, never numbers. They are UUIDv7 (spec 1.2) and a JSON number
// could not hold one anyway, but the rule also covers op ids, which are int64:
// those stay numbers, because they are a version number people compare and
// order (spec 3.12).
package wire

import (
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// Box is one activity, trip or batch of paperwork.
type Box struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Variable-precision, as the user gave it: "2019", "2019-08",
	// "2019-08-12" (spec 1.3). Empty when not set, which is a real state.
	StartAt   string    `json:"start_at,omitempty"`
	EndAt     string    `json:"end_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewBox(b domain.Box) Box {
	return Box{
		ID:          b.ID.String(),
		Name:        b.Name,
		Description: b.Description,
		StartAt:     b.StartAt.String(),
		EndAt:       b.EndAt.String(),
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}

func NewBoxes(boxes []domain.Box) []Box {
	out := make([]Box, len(boxes))
	for i, b := range boxes {
		out[i] = NewBox(b)
	}
	return out
}

// File is one stored thing.
//
// CapturedAt is an instant, not a variable-precision date (spec 1.5.2), so it
// is rendered as RFC 3339 with an offset: the value is a real point in time
// and the zone is a display decision, which is exactly what the offset says.
// It is null when nothing could establish one.
//
// Path is the path inside the box, because a bare name is not enough to go
// find the thing under boxes/. It is omitted where the caller did not resolve
// it — resolving it costs a walk up the folder chain per file.
type File struct {
	ID         string     `json:"id"`
	BoxID      string     `json:"box_id"`
	FolderID   string     `json:"folder_id"`
	Name       string     `json:"name"`
	Path       string     `json:"path,omitempty"`
	BlobKey    string     `json:"blob_key"`
	CapturedAt *time.Time `json:"captured_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// NewFile renders a file. loc is the library timezone; path may be empty.
func NewFile(f domain.File, path string, loc *time.Location) File {
	out := File{
		ID:        f.ID.String(),
		BoxID:     f.BoxID.String(),
		FolderID:  f.FolderID.String(),
		Name:      f.Name,
		Path:      path,
		BlobKey:   f.BlobKey,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	}
	if t, ok := f.CapturedAt.Time(loc); ok {
		out.CapturedAt = &t
	}
	return out
}

// Folder is a directory inside a box. ParentID is null for a box's root.
type Folder struct {
	ID        string    `json:"id"`
	BoxID     string    `json:"box_id"`
	ParentID  *string   `json:"parent_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewFolder(f domain.Folder) Folder {
	out := Folder{
		ID:        f.ID.String(),
		BoxID:     f.BoxID.String(),
		Name:      f.Name,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	}
	if f.ParentID != nil {
		s := f.ParentID.String()
		out.ParentID = &s
	}
	return out
}

// Tag is one node of the tag tree.
//
// Children nest rather than being flattened with a parent_id, because the one
// thing that asks for the whole tree wants to render it as a tree. ParentID is
// still there so a single tag out of context knows where it sits.
//
// Count is the subtree total — the tag itself plus everything under it — which
// is what a selector shows (spec 1.6). It is a pointer because computing it
// costs a GROUP BY over the largest table in the library, so most places do
// not: null means "not counted", which zero would have claimed to mean "empty".
type Tag struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name"`
	Count    *int    `json:"count"`
	Children []Tag   `json:"children,omitempty"`
}

// NewTag renders a tag without a count.
func NewTag(t domain.Tag) Tag {
	out := Tag{ID: t.ID.String(), Name: t.Name}
	if t.ParentID != nil {
		s := t.ParentID.String()
		out.ParentID = &s
	}
	return out
}

// NewTagCounted renders a tag with its subtree total.
func NewTagCounted(t domain.Tag, count int) Tag {
	out := NewTag(t)
	out.Count = &count
	return out
}

// Op is one entry in the history.
//
// Payload is absent from listings on purpose: spec 2.2.1 keeps it out of the
// list query because it is unbounded and nothing may query into it
// (invariant 4). Reading one op can carry it.
type Op struct {
	ID     int64     `json:"id"`
	TS     time.Time `json:"ts"`
	Action string    `json:"action"`
	Source string    `json:"source"`
	// RevertsID is the op this one undoes; UndoneBy is the op that undid this
	// one. Both are here because a history view needs both directions, and
	// reverts_id being an indexed column rather than payload is what makes the
	// second one answerable at all (invariant 4).
	RevertsID *int64 `json:"reverts_id"`
	UndoneBy  *int64 `json:"undone_by"`
}

// Metadata is one extracted fact about a file.
//
// Value is always a string, with Type naming how to read it (spec 1.7). A
// decimal string survives int64 exactly, which a JSON number does not.
type Metadata struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Type   string `json:"type"`
	Source string `json:"source"`
}

// MetadataRemoval is the result of `kist file metadata rm`.
//
// Removed is false when the key was already suppressed: nothing changed and no
// op was written, which a script has to be able to tell apart from a removal.
type MetadataRemoval struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	Removed bool   `json:"removed"`
}
