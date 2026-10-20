package domain

// EntryContents preserves one file or a non-root directory subtree for trash.
// Folders are ordered parents first. Ancestors exclude the deleted subtree.
type EntryContents struct {
	Filesets  []Fileset      `json:"filesets,omitempty"`
	BoxID     BoxID          `json:"box_id"`
	ID        ID             `json:"id"`
	Folder    bool           `json:"folder"`
	Parent    FolderID       `json:"parent"`
	Name      string         `json:"name"`
	Ancestors []FolderID     `json:"ancestors"`
	Folders   []Folder       `json:"folders"`
	Files     []ArchivedFile `json:"files"`
}
