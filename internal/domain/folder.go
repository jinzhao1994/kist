package domain

import "time"

// Folder is a directory inside one box. Every box has exactly one root folder,
// whose ParentID is nil and whose Name is "" (spec 1.4); the partial unique
// index in the schema is what actually enforces "exactly one".
//
// M1b only ever creates the root. Sub-folders arrive with import in M1c.
type Folder struct {
	ID        FolderID
	BoxID     BoxID
	ParentID  *FolderID
	Name      string
	NameFold  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewRootFolder builds the nameless root folder of a box.
func NewRootFolder(id FolderID, box BoxID) Folder {
	return Folder{ID: id, BoxID: box, Name: "", NameFold: ""}
}
