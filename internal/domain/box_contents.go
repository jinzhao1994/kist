package domain

// BoxContents is a self-contained archive of a box, including empty folders
// and file annotations. Folders are ordered parents first, files by ID.
// Existing entity types retain their JSON field names as part of this format.
type BoxContents struct {
	Box     Box            `json:"box"`
	Folders []Folder       `json:"folders"`
	Files   []ArchivedFile `json:"files"`
	Tags    []TagID        `json:"tags"`
}

// ArchivedFile retains all authoritative file state; derived media is excluded.
type ArchivedFile struct {
	File        File               `json:"file"`
	SHA256      string             `json:"sha256"`
	Size        int64              `json:"size"`
	Tags        []ArchivedTag      `json:"tags"`
	Rejections  []TagID            `json:"rejections"`
	Metadata    []ArchivedMetadata `json:"metadata"`
	Extractions map[string]int     `json:"extractions"`
}

type ArchivedTag struct {
	Tag    TagID  `json:"tag"`
	Source string `json:"source"`
}

type ArchivedMetadata struct {
	Key    string  `json:"key"`
	Value  *string `json:"value"`
	Type   *string `json:"type"`
	Source string  `json:"source"`
}
