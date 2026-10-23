package library

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
)

func TestLegacyClientMigrationPreservesIdentityConnectionAndQueue(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	root := c.root
	settings, err := c.settings()
	if err != nil {
		t.Fatal(err)
	}
	p := syncBox(t, "offline-work")
	if err = c.Stage(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "kist.toml")); err != nil {
		t.Fatal(err)
	}
	if err = writeClientJSON(filepath.Join(root, "client.json"), settings); err != nil {
		t.Fatal(err)
	}
	old := clientConnectionFile{Version: 1, LibraryID: settings.LibraryID, ClientConnection: ClientConnection{Server: "https://remote.example"}, Token: "private-token"}
	if err = writeClientJSON(filepath.Join(root, "connection.json"), old); err != nil {
		t.Fatal(err)
	}
	c, err = OpenClient(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	got, err := c.settings()
	if err != nil || got != settings {
		t.Fatalf("settings changed: %+v %v", got, err)
	}
	connection, token, err := c.Connection(t.Context())
	if err != nil || token != old.Token || connection != old.ClientConnection {
		t.Fatalf("connection lost: %v", err)
	}
	status, err := c.Status(t.Context())
	if err != nil || len(status.Pending) != 1 || status.Pending[0].ID != p.ID {
		t.Fatalf("pending lost: %v", err)
	}
	for _, name := range []string{"client.json", "connection.json"} {
		if _, err = os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("legacy file retained: %s", name)
		}
	}
	if wrong, err := Open(root); err == nil {
		wrong.Close()
		t.Fatal("client opened as authoritative library")
	}
	// Model a crash after publication but before legacy cleanup: a stale sidecar
	// must never restore old connection settings.
	old.Token = "stale-token"
	if err = writeClientJSON(filepath.Join(root, "connection.json"), old); err != nil {
		t.Fatal(err)
	}
	second, err := OpenClient(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	_, token, err = second.Connection(t.Context())
	if err != nil || token != "private-token" {
		t.Fatalf("stale sidecar won: %v", err)
	}
}

func TestLegacyLibraryCredentialsMigrateOnce(t *testing.T) {
	l := openTestLibrary(t)
	root := l.layout.Root
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Version = 1
	raw, err := config.Encode(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = config.WriteAtomic(l.layout.ConfigPath(), raw); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("secret"))
	grant := syncGrant{SyncCredential: SyncCredential{ID: "legacy", CreatedAt: l.now().UTC()}, Digest: hex.EncodeToString(sum[:])}
	if err = writeClientJSON(filepath.Join(root, "sync-access.json"), syncAccess{Version: 1, Grants: []syncGrant{grant}}); err != nil {
		t.Fatal(err)
	}
	other, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	list, err := other.SyncCredentials(t.Context())
	if err != nil || len(list) != 1 || list[0].ID != "legacy" {
		t.Fatalf("migration lost grants: %v", err)
	}
	if _, err = other.RevokeSync(t.Context(), "legacy"); err != nil {
		t.Fatal(err)
	}
	if err = writeClientJSON(filepath.Join(root, "sync-access.json"), syncAccess{Version: 1, Grants: []syncGrant{grant}}); err != nil {
		t.Fatal(err)
	}
	next, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	list, err = next.SyncCredentials(t.Context())
	if err != nil || len(list) != 0 {
		t.Fatalf("revoked grant resurrected: %v", err)
	}
}

func TestSettingsConcurrentEditorsAndExplicitCopy(t *testing.T) {
	l := openTestLibrary(t)
	settings, err := l.InstanceSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for _, color := range []string{"green", "orange"} {
		go func() {
			results <- l.UpdateSettings(t.Context(), settings.Revision, map[string]any{"appearance.primary_color": color})
		}()
	}
	a, b := <-results, <-results
	if !((a == nil && errors.Is(b, config.ErrChanged)) || (b == nil && errors.Is(a, config.ErrChanged))) {
		t.Fatalf("concurrent results: %v, %v", a, b)
	}
	settings, err = l.InstanceSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err = l.UpdateSettings(t.Context(), settings.Revision, map[string]any{"storage.link_mode": "copy"}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(l.layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.cfg.Storage.LinkMode != "copy" {
		t.Fatal("explicit copy was overridden")
	}
}

func TestLegacyClientRejectsUnknownOrTrailingConfiguration(t *testing.T) {
	for _, raw := range []string{`{"version":99}`, `{"version":1} {}`} {
		t.Run(raw, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "client.json")
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if err := migrateInstanceConfig(t.Context(), root, "client"); err == nil {
				t.Fatal("accepted invalid legacy configuration")
			}
			if _, err := os.Stat(filepath.Join(root, "kist.toml")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("published invalid migration")
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != raw {
				t.Fatal("legacy configuration modified")
			}
		})
	}
}
