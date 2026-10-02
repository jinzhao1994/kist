package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRequestMetadataIsChecksummed(t *testing.T) {
	l := line(1, day1, `{}`)
	l.Request = &Request{ID: "request-001", Digest: strings.Repeat("a", 64)}
	raw, err := MarshalLine(l)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalLine(raw)
	if err != nil || got.V != 2 || got.Request == nil || *got.Request != *l.Request {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	changed := bytes.Replace(raw, []byte("request-001"), []byte("request-002"), 1)
	if _, err := UnmarshalLine(changed); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("changed request accepted: %v", err)
	}
	l.V = 1
	l.Sum, err = checksum(l)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UnmarshalLine(raw); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("v1 accepted v2 metadata: %v", err)
	}
}

func TestRequestValidation(t *testing.T) {
	for _, req := range []Request{
		{ID: "short", Digest: strings.Repeat("a", 64)},
		{ID: "request /bad", Digest: strings.Repeat("a", 64)},
		{ID: strings.Repeat("a", 129), Digest: strings.Repeat("a", 64)},
		{ID: "request-valid", Digest: strings.Repeat("x", 64)},
		{ID: "request-valid", Digest: "abc"},
	} {
		l := line(1, day1, `{}`)
		l.Request = &req
		if _, err := MarshalLine(l); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("accepted %+v: %v", req, err)
		}
	}
}
