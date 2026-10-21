package journal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Problem is one thing wrong with the log.
//
// journal reports its findings in its own vocabulary rather than
// integrity.Finding: integrity sits above library and this package sits below
// it, so the dependency would run the wrong way. library translates, the same
// way it translates between Line and store.OpRow.
type Problem struct {
	// Bucket is the file, relative to the log root.
	Bucket string
	// OpID is the op the problem is about, or 0 when the line was too broken
	// to name one.
	OpID int64
	// Detail is one sentence for a person.
	Detail string
}

func (p Problem) String() string {
	if p.OpID == 0 {
		return fmt.Sprintf("%s: %s", p.Bucket, p.Detail)
	}
	return fmt.Sprintf("%s: op %d: %s", p.Bucket, p.OpID, p.Detail)
}

// Check reads the whole log and reports everything wrong with it.
//
// Two checks, and they cover the failure modes between them (spec 3.1.1): the
// per-line checksum catches a flipped bit or a rewritten op and names which
// one, and id contiguity catches a deleted line or a whole missing bucket,
// which no per-line check could see.
//
// It reports rather than stops. A log with two bad days should say so in one
// run, not one bad day per run.
func (d *Dir) Check() ([]Problem, error) {
	buckets, err := d.buckets()
	if err != nil {
		return nil, err
	}

	var problems []Problem
	var want int64 = 1
	for _, bucket := range buckets {
		day := bucketDay(bucket)
		err := d.scanRaw(bucket, func(raw []byte) {
			line, err := UnmarshalLine(raw)
			if err != nil {
				problems = append(problems, Problem{
					Bucket: bucket,
					OpID:   peekID(raw),
					Detail: err.Error(),
				})
				// The id is not trustworthy, so contiguity cannot continue
				// from it. Skip one and keep going.
				want++
				return
			}
			if line.ID != want {
				problems = append(problems, Problem{
					Bucket: bucket,
					OpID:   line.ID,
					Detail: fmt.Sprintf("expected op %d here; the log skips from %d to %d", want, want-1, line.ID),
				})
			}
			ts := line.CommitTimestamp()
			if len(ts) >= 10 && ts[:10] > day {
				problems = append(problems, Problem{
					Bucket: bucket,
					OpID:   line.ID,
					Detail: fmt.Sprintf("timestamp %s is later than its bucket", ts),
				})
			}
			want = line.ID + 1
		})
		if err != nil {
			return problems, err
		}
	}
	return problems, nil
}

// bucketDay is the YYYY-MM-DD a bucket name carries.
func bucketDay(bucket string) string {
	name := filepath.Base(bucket)
	if len(name) < 10 {
		return ""
	}
	return name[:10]
}

// peekID digs the id out of a line whose checksum did not verify, so that a
// report can still name it. It is best effort by definition.
func peekID(raw []byte) int64 {
	var probe struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return 0
	}
	return probe.ID
}

// scanRaw calls fn with every non-empty line of a bucket, newline stripped.
func (d *Dir) scanRaw(bucket string, fn func([]byte)) error {
	f, err := os.Open(d.path(bucket))
	if err != nil {
		return fmt.Errorf("read op log: %w", err)
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		raw, err := r.ReadBytes('\n')
		if trimmed := bytes.TrimRight(raw, "\n"); len(trimmed) > 0 {
			fn(trimmed)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read op log %s: %w", bucket, err)
		}
	}
}

// RecoverTail repairs a torn final append, and reports whether it did.
//
// A crash between the write and the fsync can leave a partial line at the end
// of the newest bucket. That line was never applied — the write path appends
// before it applies anything (spec 2.2.2) — so discarding it loses nothing
// that was ever visible.
//
// A torn write and a rotted line are deliberately different cases. A file that
// does not end in a newline was never finished; a line that ends properly but
// fails its checksum was finished and then damaged, and truncating that would
// destroy a real op. Only the first is repaired here; the second is what Check
// reports.
//
// The removed bytes are moved into ops/tmp/ rather than dropped, the way
// blobs/tmp/ holds a partial blob: a repair that destroys evidence is a repair
// nobody can audit.
func (d *Dir) RecoverTail() (bool, error) {
	buckets, err := d.buckets()
	if err != nil {
		return false, err
	}
	if len(buckets) == 0 {
		return false, nil
	}
	bucket := buckets[len(buckets)-1]
	path := d.path(bucket)

	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return false, fmt.Errorf("open op log bucket: %w", err)
	}
	defer f.Close()

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return false, fmt.Errorf("read op log %s: %w", bucket, err)
	}
	if size == 0 {
		return false, nil
	}
	tail := make([]byte, 1)
	if _, err := f.ReadAt(tail, size-1); err != nil {
		return false, fmt.Errorf("read op log %s: %w", bucket, err)
	}
	if tail[0] == '\n' {
		return false, nil
	}

	keep, err := lastNewlineBefore(f, size)
	if err != nil {
		return false, err
	}
	torn := make([]byte, size-keep)
	if _, err := f.ReadAt(torn, keep); err != nil {
		return false, fmt.Errorf("read op log %s: %w", bucket, err)
	}
	if err := d.stashTorn(bucket, torn); err != nil {
		return false, err
	}
	if err := f.Truncate(keep); err != nil {
		return false, fmt.Errorf("truncate op log %s: %w", bucket, err)
	}
	if err := f.Sync(); err != nil {
		return false, fmt.Errorf("flush op log %s: %w", bucket, err)
	}
	return true, nil
}

// lastNewlineBefore returns the offset just past the last newline in the file,
// or 0 if there is none.
func lastNewlineBefore(f *os.File, size int64) (int64, error) {
	for window := int64(readChunk); ; window *= 2 {
		if window > size {
			window = size
		}
		buf := make([]byte, window)
		if _, err := f.ReadAt(buf, size-window); err != nil {
			return 0, fmt.Errorf("read op log: %w", err)
		}
		if i := bytes.LastIndexByte(buf, '\n'); i >= 0 {
			return size - window + int64(i) + 1, nil
		}
		if window == size {
			return 0, nil
		}
	}
}

// stashTorn parks the bytes RecoverTail is about to remove.
func (d *Dir) stashTorn(bucket string, torn []byte) error {
	dir := filepath.Join(d.root, tmpDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create op log tmp: %w", err)
	}
	name := fmt.Sprintf("torn-%s-%s", time.Now().UTC().Format("20060102T150405Z"), filepath.Base(bucket))
	if err := os.WriteFile(filepath.Join(dir, name), torn, 0o644); err != nil {
		return fmt.Errorf("stash torn op log tail: %w", err)
	}
	return syncDir(dir)
}
