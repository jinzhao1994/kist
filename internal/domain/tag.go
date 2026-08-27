package domain

// Tag is a node in the single global tag tree. Names are unique across the
// whole tree, not per parent — the hierarchy is not a namespace (spec 1.6).
type Tag struct {
	ID       TagID
	ParentID *TagID
	Name     string
	NameFold string
}

// NewTag validates and folds.
func NewTag(id TagID, parent *TagID, name string) (Tag, error) {
	name = Normalize(name)
	if err := ValidateName(name); err != nil {
		return Tag{}, err
	}
	return Tag{ID: id, ParentID: parent, Name: name, NameFold: Fold(name)}, nil
}
