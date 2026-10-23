package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientConnectionIdentityPermissionsAndCrossHandleUpdates(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	settings, err := c.settings()
	if err != nil {
		t.Fatal(err)
	}
	connection := ClientConnection{Server: "https://nas.example"}
	if err := c.SetConnection(t.Context(), settings.LibraryID, connection, "first-secret"); err != nil {
		t.Fatal(err)
	}
	other, err := OpenClient(t.Context(), c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := other.SetConnection(t.Context(), domain.NewID(), connection, "wrong-library-secret"); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("rebound client to another NAS: %v", err)
	}
	_, token, err := c.Connection(t.Context())
	if err != nil || token != "first-secret" {
		t.Fatalf("failed connection changed credential: %v", err)
	}
	if err := other.SetConnection(t.Context(), settings.LibraryID, connection, "replacement-secret"); err != nil {
		t.Fatal(err)
	}
	_, token, err = c.Connection(t.Context())
	if err != nil || token != "replacement-secret" {
		t.Fatalf("retained stale credential: %v", err)
	}
	info, err := os.Stat(filepath.Join(c.root, "kist.toml"))
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("connection permissions: %v %v", info, err)
	}
	p := syncBox(t, "status-pending")
	if err := c.Stage(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if err := c.recordSyncEvent(t.Context(), "pending_send", pendingEvent{Target: p.ID}); err != nil {
		t.Fatal(err)
	}
	status, err := other.Status(t.Context())
	if err != nil || len(status.Pending) != 1 || !status.Pending[0].AwaitingReceipt || status.Pending[0].ID != p.ID {
		t.Fatalf("status lost uncertainty: %+v %v", status, err)
	}
	raw, err := json.Marshal(status)
	if err != nil || strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "token") {
		t.Fatalf("status exposed credential: %v", err)
	}
	if err := c.Resolve(t.Context(), p.ID, nil, "discard"); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("discarded uncertain operation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(c.root, "kist.toml"), []byte("version = 99\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Connection(t.Context()); err == nil {
		t.Fatalf("accepted unknown connection version: %v", err)
	}
}
