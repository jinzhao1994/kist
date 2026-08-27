package domain

import "time"

// Folder is a directory inside one box. Every box has exactly one root folder,
// whose ParentID is nil and whose Name is "" (spec 1.4); the partial unique
// index in the schema is what actually enforces "exactly one".
//
// The root has no name and no parent; every other folder has both.
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

// NewFolder builds a named folder under a parent.
//
// It cannot produce a root: the root is the folder with no name and no parent,
// and the schema's partial unique index allows exactly one per box (spec 1.4).
// Keeping the two constructors apart is what makes "one root" impossible to
// break by accident rather than merely unlikely.
func NewFolder(id FolderID, box BoxID, parent FolderID, name string) (Folder, error) {
	name = Normalize(name)
	if err := ValidateName(name); err != nil {
		return Folder{}, err
	}
	p := parent
	return Folder{
		ID:       id,
		BoxID:    box,
		ParentID: &p,
		Name:     name,
		NameFold: Fold(name),
	}, nil
}
