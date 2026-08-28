package wire

// The envelopes below are what a command actually prints. Each one is an
// object rather than a bare array: an array cannot grow a sibling field later
// without breaking every consumer, and the first one that needed a sibling —
// the cursor — was needed immediately.

// FileList is a page of files. Cursor is empty on the last page; pass it back
// to get the next one (spec 4.5).
type FileList struct {
	Files  []File `json:"files"`
	Cursor string `json:"cursor,omitempty"`
}

// BoxList is every box.
type BoxList struct {
	Boxes []Box `json:"boxes"`
}

// FolderList is what is directly inside one folder: subdirectories and files,
// kept apart rather than merged into one list of entries, because they are
// different things and every consumer immediately separates them again.
type FolderList struct {
	Box     Box      `json:"box"`
	Folder  Folder   `json:"folder"`
	Folders []Folder `json:"folders"`
	Files   []File   `json:"files"`
}

// TagTree is the whole tag tree, nested from the roots.
type TagTree struct {
	Tags []Tag `json:"tags"`
}

// OpList is a page of history.
type OpList struct {
	Ops []Op `json:"ops"`
}

// FileDetail is everything known about one file.
//
// CapturedFrom names which metadata row the capture time was resolved from,
// and CapturedZone how the timezone was established (spec 1.5.2). They are
// separate fields rather than a sentence because the consumer that wants to
// explain "why does this photo say 06:03 when the camera said 14:03" needs the
// parts, not the prose.
type FileDetail struct {
	File         File       `json:"file"`
	CapturedFrom string     `json:"captured_from,omitempty"`
	CapturedZone string     `json:"captured_zone,omitempty"`
	Extracted    []string   `json:"extracted"`
	Metadata     []Metadata `json:"metadata"`
}
