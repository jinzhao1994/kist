package integrity

import (
	"github.com/jinzhao1994/kist/internal/projection"
)

// CheckProjection reports how far one box's directory has drifted from the
// database.
//
// Every finding here is a warning, never an error: boxes/ is a projection, not
// data (spec 3.2). `kist relink` rebuilds the whole thing from the database, so
// nothing found here can have lost anything.
func CheckProjection(s *projection.Syncer, dir string, want []projection.Entry) (Report, error) {
	var r Report
	r.Note("projected entries", len(want))

	d, err := s.DiffDir(dir, want)
	if err != nil {
		return r, err
	}
	replaced := make(map[string]bool, len(d.Repair))
	for _, p := range d.Repair {
		replaced[p.Path] = true
		r.Add(Finding{Kind: LinkWrongTarget, Detail: p.Path + ": " + p.Reason, Fix: "kist relink"})
	}
	for _, e := range d.Create {
		if replaced[e.Path] {
			continue
		}
		r.Add(Finding{Kind: LinkMissing, Detail: e.Path, Fix: "kist relink"})
	}
	for _, p := range d.Remove {
		if replaced[p] {
			continue
		}
		r.Add(Finding{Kind: LinkStray, Detail: p, Fix: "kist relink"})
	}

	return r, nil
}
