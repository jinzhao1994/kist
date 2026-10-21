package journal

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestAppendVerifiedPreservesLegacyEnvelope(t *testing.T) {
	d, err := NewDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	line := Line{V: 1, ID: 1, TS: FormatTime(time.Now()), Action: "create_box", Origin: "user", Payload: json.RawMessage(`{"old":"facts"}`)}
	line.Sum, err = checksum(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AppendVerified(line); err != nil {
		t.Fatal(err)
	}
	got, found, err := d.Last()
	if err != nil || !found || got.V != 1 || got.Sum != line.Sum {
		t.Fatalf("legacy event rewritten: %+v %v", got, err)
	}
	line.Payload = json.RawMessage(`{"old":"changed"}`)
	if err := d.AppendVerified(line); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("unchecked replica appended: %v", err)
	}
}
