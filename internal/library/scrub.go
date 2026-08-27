package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
)

// ScrubProgress is reported after each blob, so a command that will run for
// hours can say something while it does.
type ScrubProgress struct {
	Done  int
	Total int
	Bytes int64
	Key   string
}

// Scrub re-hashes every blob and reports the ones whose content no longer
// matches their name.
//
// This is the check verify deliberately cannot do: bit rot leaves the size
// alone, so nothing short of reading every byte finds it (spec 3.8). On a real
// archive that is hours, which is why it takes a progress callback and why it
// stops cleanly when its context is cancelled.
//
// Cancelling is not an error. A scrub stopped after four hours still checked
// four hours' worth of blobs, and throwing that away because somebody pressed
// Ctrl-C is the mistake import used to make (spec 3.7.1). But an unfinished
// run says so in the report: "nothing wrong" and "nothing wrong so far" are
// different sentences, and this command's whole value is that the first one
// can be believed.
//
// Nothing is written. In particular a corrupt blob is not removed: it may be
// the only copy there is, and damaged content still beats none.
func (l *Library) Scrub(ctx context.Context, progress func(ScrubProgress)) (integrity.Report, error) {
	var r integrity.Report

	known, err := store.AllBlobs(l.db)
	if err != nil {
		return r, err
	}

	var damaged []string
	notes := map[string]integrity.Kind{}

	for i, b := range known {
		if ctx.Err() != nil {
			r.Add(integrity.Finding{Kind: integrity.ScrubIncomplete,
				Detail: fmt.Sprintf("stopped after %d of %d blobs", i, len(known))})
			break
		}

		key, err := blob.ParseKey(b.Key)
		if err != nil {
			r.Add(integrity.Finding{Kind: integrity.BlobKeyMalformed, Detail: b.Key, Fix: "kist rebuild"})
			continue
		}

		hex, size, err := l.blobs.Rehash(ctx, key)
		if err != nil {
			if ctx.Err() != nil {
				r.Add(integrity.Finding{Kind: integrity.ScrubIncomplete,
					Detail: fmt.Sprintf("stopped after %d of %d blobs", i, len(known))})
				break
			}
			// Unreadable is a finding, not a stop: the next blob may be fine,
			// and the user needs the whole list.
			damaged = append(damaged, b.Key)
			notes[b.Key] = integrity.BlobMissing
			continue
		}

		r.Note("blobs", 1)
		switch {
		case hex != key.Hex():
			damaged = append(damaged, b.Key)
			notes[b.Key] = integrity.BlobCorrupt
		case size != b.Size:
			damaged = append(damaged, b.Key)
			notes[b.Key] = integrity.BlobWrongSize
		}

		if progress != nil {
			progress(ScrubProgress{Done: i + 1, Total: len(known), Bytes: size, Key: b.Key})
		}
	}

	if err := integrity.DescribeDamagedBlobs(l.db, &r, damaged, notes); err != nil {
		return r, err
	}
	return r, nil
}
