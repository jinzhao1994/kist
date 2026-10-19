package domain

import "time"

// Folder is a directory. Every box has exactly one root folder, whose ParentID
// is nil and whose Name is "" (spec 1.4).
//
// It does not know its box. A folder's box is whichever box's root it descends
// from, and that is the only place the answer is written down — a BoxID here
// would be a second copy able to disagree with the first (spec 1.4).
//
// The root has no name and no parent; every other folder has both.
type Folder struct {
	// TotalSize is populated by display queries only, never stored in journal payloads.
	TotalSize int64 `json:"-"`
	ID        FolderID
	ParentID  *FolderID
	Name      string
	NameFold  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewRootFolder builds a nameless root folder. The box that owns it records
// the link, in boxes.root_folder_id.
func NewRootFolder(id FolderID) Folder {
	return Folder{ID: id, Name: "", NameFold: ""}
}

// NewFolder builds a named folder under a parent.
//
// It cannot produce a root: the root is the folder with no name and no parent.
// Keeping the two constructors apart is what makes an accidental second root
// impossible rather than merely unlikely.
func NewFolder(id FolderID, parent FolderID, name string) (Folder, error) {
	name = Normalize(name)
	if err := ValidateName(name); err != nil {
		return Folder{}, err
	}
	p := parent
	return Folder{
		ID:       id,
		ParentID: &p,
		Name:     name,
		NameFold: Fold(name),
	}, nil
}
