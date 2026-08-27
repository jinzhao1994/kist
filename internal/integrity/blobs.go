package integrity

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/store"
)

// CheckBlobs confirms that every row in the blobs table has content on disk of
// the right size, and reports content on disk that no row knows about.
//
// It stats and does not read: that is the line between verify and scrub
// (spec 3.8). A blob of the right size whose bytes have rotted is invisible
// here by design, and is exactly what scrub is for.
func CheckBlobs(ctx context.Context, q store.Queryer, d *blob.Dir) (Report, error) {
	var r Report

	known, err := store.AllBlobs(q)
	if err != nil {
		return r, err
	}
	r.Note("blobs", len(known))

	for _, b := range known {
		key, err := blob.ParseKey(b.Key)
		if err != nil {
			r.Add(Finding{Kind: BlobKeyMalformed, Detail: b.Key, Fix: "kist rebuild"})
			continue
		}
		info, err := d.Stat(ctx, key)
		if err != nil {
			r.Add(Finding{Kind: BlobMissing, Detail: b.Key, Fix: "kist repair"})
			continue
		}
		if info.Size != b.Size {
			r.Add(Finding{
				Kind:   BlobWrongSize,
				Detail: fmt.Sprintf("%s: %d bytes on disk, %d in the database", b.Key, info.Size, b.Size),
				Fix:    "kist repair",
			})
		}
	}

	onDisk := make(map[blob.Key]bool, len(known))
	if err := d.Walk(func(k blob.Key, _ int64) error {
		onDisk[k] = true
		return nil
	}); err != nil {
		return r, err
	}
	for _, b := range known {
		delete(onDisk, blob.Key(b.Key))
	}
	for k := range onDisk {
		// Not a problem: v0.1 collects nothing, so content written by a
		// transaction that then rolled back simply stays (spec 3.9). Saying so
		// is still better than a directory that silently grows.
		r.Add(Finding{Kind: BlobOrphan, Detail: string(k)})
	}
	return r, nil
}
