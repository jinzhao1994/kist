package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Append writes one line and flushes it to disk.
//
// This is step 1 of a write (spec 2.2.2), and the fsync is the whole point:
// the op must be durable before anything applies it, so that a crash can only
// leave the database behind the log and never ahead of it.
//
// The caller holds the library lock. Append does not take one — it does not
// know about kist.lock, and a second lock here would be a second answer to a
// question spec 2.7 already answered.
func (d *Dir) Append(l Line) error {
	b, err := MarshalLine(l)
	if err != nil {
		return err
	}
	bucket, err := d.bucketToWrite(l.TS)
	if err != nil {
		return err
	}

	path := d.path(bucket)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create op log bucket: %w", err)
	}
	_, statErr := os.Stat(path)
	isNew := os.IsNotExist(statErr)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open op log bucket: %w", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("append op %d: %w", l.ID, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("flush op %d: %w", l.ID, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close op log bucket: %w", err)
	}
	if isNew {
		return d.syncNewBucket(dir)
	}
	return nil
}

// bucketToWrite is the day's bucket, or the last one already written if that
// is later.
//
// The clock can move backwards — NTP corrects it, and spec 2.2 already says
// ts is not an ordering key for that reason. Without this clamp an op written
// after a correction lands in an earlier file, and reading the buckets in name
// order stops reading the ops in id order. Everything else in this package
// assumes it does.
func (d *Dir) bucketToWrite(ts string) (string, error) {
	bucket, err := bucketFor(ts)
	if err != nil {
		return "", err
	}
	buckets, err := d.buckets()
	if err != nil {
		return "", err
	}
	if n := len(buckets); n > 0 && strings.Compare(buckets[n-1], bucket) > 0 {
		return buckets[n-1], nil
	}
	return bucket, nil
}

// syncNewBucket flushes the directory entries a new bucket file needed, from
// the file's own directory up to the log root.
func (d *Dir) syncNewBucket(dir string) error {
	for {
		if err := syncDir(dir); err != nil {
			return err
		}
		if dir == d.root {
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}
