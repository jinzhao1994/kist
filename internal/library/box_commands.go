package library

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

// Command generation is shared by NAS writes and offline intents. IDs are
// chosen here before persistence; Apply never generates them during replay.
func createBoxPayload(name, description string, start, end domain.LocalDate) (oplog.CreateBoxPayload, error) {
	if err := domain.ValidateName(name); err != nil {
		return oplog.CreateBoxPayload{}, err
	}
	return oplog.CreateBoxPayload{BoxID: domain.NewBoxID(), RootFolderID: domain.NewFolderID(), BoxFields: oplog.BoxFields{Name: name, Description: description, StartAt: start, EndAt: end}}, nil
}

// Only changed fields are asserted. An independent change to another field
// therefore does not block merging or strictly undoing this command.
func updateBoxPayload(current domain.Box, fields BoxFields) oplog.UpdateBoxPayload {
	p := oplog.UpdateBoxPayload{BoxID: current.ID}
	if current.Name != fields.Name {
		p.Name = &oplog.TextChange{Before: current.Name, After: fields.Name}
	}
	if current.Description != fields.Description {
		p.Description = &oplog.TextChange{Before: current.Description, After: fields.Description}
	}
	if current.StartAt != fields.StartAt {
		p.StartAt = &oplog.DateChange{Before: current.StartAt, After: fields.StartAt}
	}
	if current.EndAt != fields.EndAt {
		p.EndAt = &oplog.DateChange{Before: current.EndAt, After: fields.EndAt}
	}
	return p
}
