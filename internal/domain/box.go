package domain

import "time"

// Box is the base unit: one trip, one hospital visit, one batch of paperwork.
//
// CreatedAt and UpdatedAt are machine time in UTC and are always taken from
// ops.ts, never from time.Now() — otherwise a rebuild would stamp the whole
// library with the moment it was rebuilt (spec 2.3.1). StartAt and EndAt are
// human time with no timezone at all (spec 1.3).
type Box struct {
	ID          BoxID
	Name        string
	NameFold    string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartAt     LocalDate
	EndAt       LocalDate
}

// NewBox validates and folds. Timestamps are left zero: they belong to the op
// that creates the box, and only the write path knows that op's ts.
func NewBox(id BoxID, name, description string) (Box, error) {
	name, description = Normalize(name), Normalize(description)
	if err := ValidateName(name); err != nil {
		return Box{}, err
	}
	return Box{
		ID:          id,
		Name:        name,
		NameFold:    Fold(name),
		Description: description,
	}, nil
}
