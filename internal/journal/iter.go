package journal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// readChunk is how much Last reads at a time when walking backwards. A single
// line can be far larger — a 5000-file import's payload is about a megabyte
// (spec 2.2.1) — so the walk grows its window instead of assuming a bound.
const readChunk = 64 << 10

// Iter calls fn for every line in the log, in id order.
//
// Buckets are read in name order and lines within a bucket in file order,
// which is id order because Append never moves a bucket backwards.
//
// One line at a time, never a slice of all of them: reading the whole log into
// memory would hold every payload at once, and after a hundred thousand
// imported files that is tens of megabytes for no reason.
func (d *Dir) Iter(fn func(Line) error) error {
	buckets, err := d.buckets()
	if err != nil {
		return err
	}
	for _, bucket := range buckets {
		if err := d.iterBucket(bucket, fn); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dir) iterBucket(bucket string, fn func(Line) error) error {
	f, err := os.Open(d.path(bucket))
	if err != nil {
		return fmt.Errorf("read op log: %w", err)
	}
	defer f.Close()

	// bufio.Reader rather than bufio.Scanner: Scanner caps a token at 64 KiB
	// and reports a line longer than that as an error, which an import payload
	// reaches routinely.
	r := bufio.NewReader(f)
	for {
		raw, err := r.ReadBytes('\n')
		if len(raw) > 0 {
			trimmed := bytes.TrimRight(raw, "\n")
			if len(trimmed) > 0 {
				line, lerr := UnmarshalLine(trimmed)
				if lerr != nil {
					return fmt.Errorf("%s: %w", bucket, lerr)
				}
				if ferr := fn(line); ferr != nil {
					return ferr
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read op log %s: %w", bucket, err)
		}
	}
}

// Last returns the newest line, or false if the log is empty.
//
// It reads the end of the last bucket rather than iterating, because the write
// path calls it on every write to pick the next id (spec 2.2.2): a scan would
// make each write cost the whole history.
func (d *Dir) Last() (Line, bool, error) {
	buckets, err := d.buckets()
	if err != nil {
		return Line{}, false, err
	}
	for i := len(buckets) - 1; i >= 0; i-- {
		raw, ok, err := lastLineOf(d.path(buckets[i]))
		if err != nil {
			return Line{}, false, err
		}
		if !ok {
			continue // an empty bucket; nothing writes one, but do not trip on it
		}
		line, err := UnmarshalLine(raw)
		if err != nil {
			return Line{}, false, fmt.Errorf("%s: %w", buckets[i], err)
		}
		return line, true, nil
	}
	return Line{}, false, nil
}

// lastLineOf returns the file's final complete line, without its newline.
func lastLineOf(path string) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, fmt.Errorf("read op log: %w", err)
	}
	defer f.Close()

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, false, fmt.Errorf("read op log %s: %w", path, err)
	}
	if size == 0 {
		return nil, false, nil
	}

	// Walk backwards in growing windows until the window holds a newline
	// before the trailing one, or until it covers the file.
	for window := int64(readChunk); ; window *= 2 {
		if window > size {
			window = size
		}
		buf := make([]byte, window)
		if _, err := f.ReadAt(buf, size-window); err != nil {
			return nil, false, fmt.Errorf("read op log %s: %w", path, err)
		}
		body := bytes.TrimRight(buf, "\n")
		if i := bytes.LastIndexByte(body, '\n'); i >= 0 {
			return body[i+1:], true, nil
		}
		if window == size {
			if len(body) == 0 {
				return nil, false, nil
			}
			return body, true, nil
		}
	}
}
