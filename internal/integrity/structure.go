package integrity

import (
	"github.com/jinzhao1994/kist/internal/store"
)

// CheckStructure looks for materialized-view rows that contradict each other.
//
// None of these can be produced by library: the schema's foreign keys and
// unique indexes forbid them. The check is for the day something arrives
// another way — a hand-edited database, a half-restored backup, a bug not yet
// written. It is spec 3.8's cheap approximation of `verify --deep`: it cannot
// find everything a replay would, but it does not have to replay.
func CheckStructure(q store.Queryer) (Report, error) {
	var r Report

	checks := []struct {
		kind Kind
		fn   func(store.Queryer) ([]store.Inconsistency, error)
	}{
		{FileOrphanFolder, store.OrphanFiles},
		{FileUnknownBlob, store.FilesWithUnknownBlob},
		{FolderUnreachable, store.UnreachableFolders},
		{BoxBadRoot, store.BoxesWithBadRoot},
		{FolderUnclaimedRoot, store.UnclaimedRootFolders},
	}
	for _, c := range checks {
		found, err := c.fn(q)
		if err != nil {
			return r, err
		}
		for _, f := range found {
			r.Add(Finding{Kind: c.kind, Detail: f.Detail, Fix: "kist rebuild"})
		}
	}
	return r, nil
}
