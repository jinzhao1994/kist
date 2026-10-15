package upload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamingPrefixAndImmutableSeal(t *testing.T) {
	s := newStore(t, t.TempDir(), 100)
	m := manifest([]byte("abcdefgh"))
	m.Protocol = 2
	m.Files[0].SHA256 = ""
	state := create(t, s, "streaming", m)
	if err := appendBlock(s, t.Context(), state.ID, 0, []byte("abcd")); err != nil {
		t.Fatal(err)
	}
	s = newStore(t, s.path, 100)
	if err := s.With(t.Context(), state.ID, func(v *Session) error {
		checkKind(t, v.MatchPrefix(t.Context(), 0, 4, digest([]byte("xxxx"))), "file_integrity")
		checkKind(t, v.Seal([]string{digest([]byte("abcdefgh"))}), "incomplete")
		return v.MatchPrefix(t.Context(), 0, 4, digest([]byte("abcd")))
	}); err != nil {
		t.Fatal(err)
	}
	if err := appendBlock(s, t.Context(), state.ID, 4, []byte("efgh")); err != nil {
		t.Fatal(err)
	}
	if err := s.With(t.Context(), state.ID, func(v *Session) error {
		if err := v.Seal([]string{digest([]byte("abcdefgh"))}); err != nil {
			return err
		}
		checkKind(t, v.Seal([]string{digest([]byte("other"))}), "file_integrity")
		_, err := v.Verify(t.Context())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := progress(t, newStore(t, s.path, 100), state.ID).Digests; len(got) != 1 || got[0] != digest([]byte("abcdefgh")) {
		t.Fatal(got)
	}
	// Recovery compares real disk data, rather than a cached chunk digest.
	if err := os.WriteFile(filepath.Join(s.path, state.ID, "0"), []byte("xxxxefgh"), 0600); err != nil {
		t.Fatal(err)
	}
	err := s.With(context.Background(), state.ID, func(v *Session) error { return v.MatchPrefix(t.Context(), 0, 8, digest([]byte("abcdefgh"))) })
	checkKind(t, err, "file_integrity")
}
