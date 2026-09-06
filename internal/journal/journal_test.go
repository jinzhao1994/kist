package journal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newDir(t *testing.T) *Dir {
	t.Helper()
	d, err := NewDir(t.TempDir())
	if err != nil {
		t.Fatalf("NewDir: %v", err)
	}
	return d
}

func line(id int64, ts string, payload string) Line {
	return Line{
		ID:      id,
		TS:      ts,
		Action:  "import_files",
		Origin:  "user",
		Payload: json.RawMessage(payload),
	}
}

const day1 = "2026-09-06T12:00:00.123456789Z"
const day2 = "2026-09-07T00:00:00.000000000Z"

// TestRoundTripIsByteStable is what the checksum rests on.
//
// checksum hashes a re-encoding of the line rather than its raw bytes, which
// is only sound while decoding and re-encoding reproduces the input exactly.
// If that ever stops being true, every line already on disk fails its
// checksum — so it is pinned here rather than left as a comment.
func TestRoundTripIsByteStable(t *testing.T) {
	payloads := []string{
		`{"box_id":"018f-…","files":[{"name":"护照.jpg"}]}`,
		`{}`,
		`{"text":"a<b>c&d"}`,
		`{"nested":{"a":[1,2,3],"b":null},"n":-315619200}`,
	}
	for _, p := range payloads {
		reverts := int64(7)
		l := line(41, day1, p)
		l.RevertsID = &reverts
		raw, err := MarshalLine(l)
		if err != nil {
			t.Fatalf("MarshalLine: %v", err)
		}
		got, err := UnmarshalLine(raw)
		if err != nil {
			t.Fatalf("UnmarshalLine: %v", err)
		}
		again, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, again) {
			t.Errorf("round trip changed the bytes.\n in: %s\nout: %s", raw, again)
		}
	}
}

func TestChecksumCatchesAFlippedByte(t *testing.T) {
	raw, err := MarshalLine(line(1, day1, `{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Replace(raw, []byte(`"a":1`), []byte(`"a":2`), 1)
	if bytes.Equal(raw, corrupt) {
		t.Fatal("test did not actually change anything")
	}
	if _, err := UnmarshalLine(corrupt); err == nil {
		t.Fatal("a rewritten payload passed its checksum")
	}
}

func TestUnknownVersionIsNamed(t *testing.T) {
	raw, err := MarshalLine(line(1, day1, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	var l Line
	if err := json.Unmarshal(raw, &l); err != nil {
		t.Fatal(err)
	}
	l.V = Version + 1
	future, err := json.Marshal(l) // keep its own sum; version is checked first
	if err != nil {
		t.Fatal(err)
	}
	_, err = UnmarshalLine(future)
	if err == nil || !strings.Contains(err.Error(), "newer version") {
		t.Fatalf("want an unsupported-version error, got %v", err)
	}
}

func TestBucketPath(t *testing.T) {
	got, err := bucketFor(day1)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026/09/2026-09-06_ops.jsonl"; got != want {
		t.Errorf("bucketFor = %q, want %q", got, want)
	}
}

// TestBucketsNeverMoveBackwards is the NTP case.
//
// Verified to fail: drop the clamp in bucketToWrite and the second op lands in
// the 09-06 file, after which reading the buckets in name order reads op 2
// before op 1.
func TestBucketsNeverMoveBackwards(t *testing.T) {
	d := newDir(t)
	if err := d.Append(line(1, day2, `{}`)); err != nil {
		t.Fatal(err)
	}
	// The clock is corrected backwards by a day.
	if err := d.Append(line(2, day1, `{}`)); err != nil {
		t.Fatal(err)
	}

	buckets, err := d.buckets()
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0] != "2026/09/2026-09-07_ops.jsonl" {
		t.Fatalf("buckets = %v, want only the 09-07 one", buckets)
	}

	var ids []int64
	if err := d.Iter(func(l Line) error { ids = append(ids, l.ID); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Errorf("ids = %v, want [1 2] — name order must be id order", ids)
	}
}

func TestAppendAndIter(t *testing.T) {
	d := newDir(t)
	const n = 1000
	for i := int64(1); i <= n; i++ {
		ts := day1
		if i > n/2 {
			ts = day2
		}
		if err := d.Append(line(i, ts, fmt.Sprintf(`{"i":%d}`, i))); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	var seen int64
	err := d.Iter(func(l Line) error {
		seen++
		if l.ID != seen {
			return fmt.Errorf("got op %d at position %d", l.ID, seen)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != n {
		t.Errorf("read %d lines, want %d", seen, n)
	}
}

func TestLastOnEmptyLog(t *testing.T) {
	d := newDir(t)
	_, ok, err := d.Last()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("an empty log reported a last op")
	}
}

// TestLastReadsPastAHugeLine exercises the growing backwards window. An import
// payload of a megabyte is ordinary (spec 2.2.1), and a fixed 64 KiB read
// would miss the newline in front of it.
func TestLastReadsPastAHugeLine(t *testing.T) {
	d := newDir(t)
	if err := d.Append(line(1, day1, `{"small":true}`)); err != nil {
		t.Fatal(err)
	}
	big := `{"blob":"` + strings.Repeat("x", 300<<10) + `"}`
	if err := d.Append(line(2, day1, big)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := d.Last()
	if err != nil || !ok {
		t.Fatalf("Last: %v ok=%v", err, ok)
	}
	if got.ID != 2 {
		t.Errorf("Last = op %d, want 2", got.ID)
	}
}

func TestCheckIsCleanOnAGoodLog(t *testing.T) {
	d := newDir(t)
	for i := int64(1); i <= 5; i++ {
		if err := d.Append(line(i, day1, `{}`)); err != nil {
			t.Fatal(err)
		}
	}
	problems, err := d.Check()
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("clean log reported %v", problems)
	}
}

func TestCheckCatchesTheFourWaysItBreaks(t *testing.T) {
	bucket1 := "2026/09/2026-09-06_ops.jsonl"
	bucket2 := "2026/09/2026-09-07_ops.jsonl"

	cases := []struct {
		name   string
		break_ func(t *testing.T, d *Dir)
		want   string
	}{
		{
			name: "bit rot",
			break_: func(t *testing.T, d *Dir) {
				p := d.path(bucket1)
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				b = bytes.Replace(b, []byte(`"i":2`), []byte(`"i":9`), 1)
				if err := os.WriteFile(p, b, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "checksum",
		},
		{
			name: "a deleted line",
			break_: func(t *testing.T, d *Dir) {
				p := d.path(bucket1)
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				lines := bytes.SplitAfter(b, []byte("\n"))
				kept := append(append([]byte{}, lines[0]...), lines[2]...)
				if err := os.WriteFile(p, kept, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "skips",
		},
		{
			name: "a missing bucket",
			break_: func(t *testing.T, d *Dir) {
				if err := os.Remove(d.path(bucket1)); err != nil {
					t.Fatal(err)
				}
			},
			want: "skips",
		},
		{
			name: "a rewritten op",
			break_: func(t *testing.T, d *Dir) {
				p := d.path(bucket2)
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				b = bytes.Replace(b, []byte(`"action":"import_files"`), []byte(`"action":"delete_files"`), 1)
				if err := os.WriteFile(p, b, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "checksum",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDir(t)
			for i := int64(1); i <= 3; i++ {
				if err := d.Append(line(i, day1, fmt.Sprintf(`{"i":%d}`, i))); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Append(line(4, day2, `{"i":4}`)); err != nil {
				t.Fatal(err)
			}

			tc.break_(t, d)

			problems, err := d.Check()
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if len(problems) == 0 {
				t.Fatalf("%s went unreported", tc.name)
			}
			joined := fmt.Sprint(problems)
			if !strings.Contains(joined, tc.want) {
				t.Errorf("problems = %v, want one mentioning %q", problems, tc.want)
			}
		})
	}
}

// TestRecoverTailRemovesATornWriteAndKeepsTheBytes is the crash-during-append
// case. Nothing applied that line, so dropping it is safe — but it is moved
// aside rather than deleted.
func TestRecoverTailRemovesATornWrite(t *testing.T) {
	d := newDir(t)
	for i := int64(1); i <= 2; i++ {
		if err := d.Append(line(i, day1, `{}`)); err != nil {
			t.Fatal(err)
		}
	}
	p := d.path("2026/09/2026-09-06_ops.jsonl")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"v":1,"id":3,"ts":"2026-09`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	recovered, err := d.RecoverTail()
	if err != nil {
		t.Fatalf("RecoverTail: %v", err)
	}
	if !recovered {
		t.Fatal("a torn tail was not recovered")
	}

	problems, err := d.Check()
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("log still reports %v", problems)
	}

	stashed, err := filepath.Glob(filepath.Join(d.Root(), tmpDir, "torn-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stashed) != 1 {
		t.Fatalf("torn bytes not stashed: %v", stashed)
	}
	b, err := os.ReadFile(stashed[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"id":3`) {
		t.Errorf("stashed bytes are not the torn line: %q", b)
	}
}

// TestRecoverTailLeavesRotAlone is the other half. A line that ends properly
// but fails its checksum was written completely and then damaged; truncating
// it would destroy a real op.
func TestRecoverTailLeavesRotAlone(t *testing.T) {
	d := newDir(t)
	for i := int64(1); i <= 2; i++ {
		if err := d.Append(line(i, day1, fmt.Sprintf(`{"i":%d}`, i))); err != nil {
			t.Fatal(err)
		}
	}
	p := d.path("2026/09/2026-09-06_ops.jsonl")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	before := len(b)
	b = bytes.Replace(b, []byte(`"i":2`), []byte(`"i":9`), 1)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}

	recovered, err := d.RecoverTail()
	if err != nil {
		t.Fatalf("RecoverTail: %v", err)
	}
	if recovered {
		t.Error("rot was mistaken for a torn write and truncated")
	}
	after, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != before {
		t.Errorf("file shrank from %d to %d bytes", before, len(after))
	}
}

func TestAppendIsDurableAcrossReopen(t *testing.T) {
	root := t.TempDir()
	d, err := NewDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Append(line(1, day1, `{"a":1}`)); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenDir(root)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := reopened.Last()
	if err != nil || !ok {
		t.Fatalf("Last: %v ok=%v", err, ok)
	}
	if got.ID != 1 || got.Action != "import_files" {
		t.Errorf("read back %+v", got)
	}
	if _, err := got.Time(); err != nil {
		t.Errorf("Time: %v", err)
	}
	if want := time.Date(2026, 9, 6, 12, 0, 0, 123456789, time.UTC); func() bool {
		ts, _ := got.Time()
		return !ts.Equal(want)
	}() {
		t.Errorf("timestamp did not survive the round trip")
	}
}
