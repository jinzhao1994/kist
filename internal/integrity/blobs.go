package integrity

import (
	"context"
	"fmt"
	"strings"

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

	var damaged []damage
	for _, b := range known {
		key, err := blob.ParseKey(b.Key)
		if err != nil {
			r.Add(Finding{Kind: BlobKeyMalformed, Detail: b.Key, Fix: "kist rebuild"})
			continue
		}
		info, err := d.Stat(ctx, key)
		if err != nil {
			damaged = append(damaged, damage{kind: BlobMissing, key: b.Key})
			continue
		}
		if info.Size != b.Size {
			damaged = append(damaged, damage{kind: BlobWrongSize, key: b.Key,
				note: fmt.Sprintf("%d bytes on disk, %d in the database", info.Size, b.Size)})
		}
	}
	if err := describeDamage(q, &r, damaged); err != nil {
		return r, err
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

// damage is one blob that needs repairing, before it has been turned into a
// sentence about the files that use it.
type damage struct {
	kind Kind
	key  string
	note string
}

// describeDamage turns damaged blob keys into sentences about files.
//
// One query for the whole batch rather than one per key: damage is normally
// rare, but a half-restored backup can make it universal, and that is exactly
// the moment a report must not take a minute to print.
func describeDamage(q store.Queryer, r *Report, damaged []damage) error {
	if len(damaged) == 0 {
		return nil
	}
	keys := make([]string, len(damaged))
	for i, d := range damaged {
		keys[i] = d.key
	}
	users, err := store.FilesUsingBlobs(q, keys)
	if err != nil {
		return err
	}
	for _, d := range damaged {
		r.Add(Finding{Kind: d.kind, Detail: describe(d.key, d.note, users[d.key]), Fix: "kist repair"})
	}
	return nil
}

// describe writes one damaged blob as a line a person can act on: the files
// that need it first, the key second, because the key is only useful once you
// have decided to do something about it.
func describe(key, note string, files []string) string {
	var sb strings.Builder
	switch {
	case len(files) == 0:
		// No file references it, so there is nothing to name. This is content
		// left behind by an undone import — still worth reporting, since the
		// database has a row for it.
		sb.WriteString("(no file uses this content) ")
	case len(files) <= 3:
		sb.WriteString(strings.Join(files, ", ") + " ")
	default:
		sb.WriteString(fmt.Sprintf("%s and %d more ", strings.Join(files[:3], ", "), len(files)-3))
	}
	sb.WriteString("— " + shortKey(key))
	if note != "" {
		sb.WriteString(", " + note)
	}
	return sb.String()
}

// shortKey trims a digest to something that still identifies it in a log but
// does not take a whole terminal line.
func shortKey(key string) string {
	if i := strings.IndexByte(key, ':'); i >= 0 && len(key) > i+13 {
		return key[:i+13] + "…"
	}
	return key
}
