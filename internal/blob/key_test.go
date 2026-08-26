package blob

import (
	"path/filepath"
	"strings"
	"testing"
)

const testHex = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func TestKeyRoundTrip(t *testing.T) {
	tests := []struct {
		suffix int
		want   string
	}{
		{0, "sha256:" + testHex},
		{1, "sha256:" + testHex + ".1"},
		{42, "sha256:" + testHex + ".42"},
	}
	for _, tt := range tests {
		k := NewKey(AlgoSHA256, testHex, tt.suffix)
		if string(k) != tt.want {
			t.Errorf("NewKey(suffix=%d) = %q, want %q", tt.suffix, k, tt.want)
		}

		parsed, err := ParseKey(string(k))
		if err != nil {
			t.Fatalf("ParseKey(%q): %v", k, err)
		}
		if parsed.Algo() != AlgoSHA256 {
			t.Errorf("Algo() = %q, want %q", parsed.Algo(), AlgoSHA256)
		}
		if parsed.Hex() != testHex {
			t.Errorf("Hex() = %q, want %q", parsed.Hex(), testHex)
		}
		if parsed.Suffix() != tt.suffix {
			t.Errorf("Suffix() = %d, want %d", parsed.Suffix(), tt.suffix)
		}
	}
}

func TestParseKeyRejects(t *testing.T) {
	for _, s := range []string{
		"",
		testHex,                       // no algorithm prefix
		"sha256:",                     // no hash
		"sha256:" + testHex[:63],      // too short
		"sha256:" + testHex + "0",     // too long
		"sha256:ABCDEF" + testHex[6:], // uppercase hex
		"sha256:zzzz" + testHex[4:],   // not hex
		"md5:" + testHex,              // unknown algorithm
		"sha256:" + testHex + ".",     // empty suffix
		"sha256:" + testHex + ".0",    // suffix 0 must be written as no suffix
		"sha256:" + testHex + ".-1",   // negative
		"sha256:" + testHex + ".x",    // non-numeric suffix
		"sha256/" + testHex,           // path form is not a key
	} {
		if _, err := ParseKey(s); err == nil {
			t.Errorf("ParseKey(%q) = nil error, want a rejection", s)
		}
	}
}

func TestRelPathShards(t *testing.T) {
	k := NewKey(AlgoSHA256, testHex, 0)
	want := filepath.Join("sha256", "ab", "cd", testHex)
	if got := k.RelPath(); got != want {
		t.Errorf("RelPath() = %q, want %q", got, want)
	}

	withSuffix := NewKey(AlgoSHA256, testHex, 1)
	if got, want := withSuffix.RelPath(), want+".1"; got != want {
		t.Errorf("RelPath() with suffix = %q, want %q", got, want)
	}
}

func TestKeyIsNotAPath(t *testing.T) {
	// The ':' is a free guardrail: code that tries to build a path by
	// concatenating a key fails loudly on Windows instead of quietly writing
	// a broken file (spec 3.3).
	k := NewKey(AlgoSHA256, testHex, 0)
	if !strings.Contains(string(k), ":") {
		t.Error("key must contain ':' so it cannot be mistaken for a path")
	}
	if strings.Contains(k.RelPath(), ":") {
		t.Error("RelPath must not contain ':'")
	}
}
