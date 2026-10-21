package library

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/blob"
)

func TestSyncOriginalVerifiedPlacementAndRetry(t *testing.T) {
	l := openTestLibrary(t)
	content := "offline original"
	key := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
	status, err := l.SyncOriginal(t.Context(), key)
	if err != nil || status.Present {
		t.Fatalf("missing original: %+v %v", status, err)
	}
	if err := l.StoreSyncOriginal(t.Context(), key, int64(len(content)), strings.NewReader("wrong")); !errors.Is(err, blob.ErrIntegrityMismatch) {
		t.Fatalf("accepted invalid original: %v", err)
	}
	for range 2 {
		if err := l.StoreSyncOriginal(t.Context(), key, int64(len(content)), strings.NewReader(content)); err != nil {
			t.Fatal(err)
		}
	}
	status, err = l.SyncOriginal(t.Context(), key)
	if err != nil || !status.Present || status.Size != int64(len(content)) {
		t.Fatalf("stored original: %+v %v", status, err)
	}
	f, err := l.OpenSyncOriginal(t.Context(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Seek(8, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(f)
	if err != nil || string(raw) != content[8:] {
		t.Fatalf("range=%q err=%v", raw, err)
	}
	if count, _ := l.OpCount(t.Context()); count != 0 {
		t.Fatal("blob transfer created an archive operation")
	}
}
