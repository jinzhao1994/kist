package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncEnvelopeVersionsAndTampering(t *testing.T) {
	l := line(1, day1, `{}`)
	l.Sync = &Sync{ID: "pending-operation-1", Digest: strings.Repeat("b", 64), AcceptedAt: day2}
	raw, err := MarshalLine(l)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalLine(raw)
	if err != nil || got.V != 3 || got.TS != day1 || got.CommitTimestamp() != day2 {
		t.Fatalf("line=%+v err=%v", got, err)
	}
	again, _ := json.Marshal(got)
	if !bytes.Equal(raw, again) {
		t.Fatal("sync round trip changed bytes")
	}
	for _, pair := range [][2]string{{"pending-operation-1", "pending-operation-2"}, {day2, day1}, {strings.Repeat("b", 64), strings.Repeat("c", 64)}} {
		changed := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if _, err := UnmarshalLine(changed); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("tampering accepted: %v", err)
		}
	}
	for _, v := range []int{1, 2} {
		got.V = v
		got.Sum, _ = checksum(got)
		old, _ := json.Marshal(got)
		if _, err := UnmarshalLine(old); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("v%d accepted sync metadata: %v", v, err)
		}
	}
	l.Sync.AcceptedAt = "not-a-time"
	if _, err := MarshalLine(l); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("bad acceptance time: %v", err)
	}
}

func TestSyncBucketsUseAcceptanceNotOfflineClock(t *testing.T) {
	d := newDir(t)
	first := line(1, day1, `{}`)
	if err := d.Append(first); err != nil {
		t.Fatal(err)
	}
	second := line(2, "2030-01-01T00:00:00.000000000Z", `{}`)
	second.Sync = &Sync{ID: "pending-operation-2", Digest: strings.Repeat("b", 64), AcceptedAt: day2}
	if err := d.Append(second); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(d.Root(), "2026/09/2026-09-07_ops.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	problems, err := d.Check()
	if err != nil || len(problems) != 0 {
		t.Fatalf("check=%v err=%v", problems, err)
	}
}
