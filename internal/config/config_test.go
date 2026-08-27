package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/blob"
)

// writeConfig writes a kist.toml and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	want := Default(blob.LinkReflink)

	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Version != want.Version {
		t.Errorf("Version = %d, want %d", got.Version, want.Version)
	}
	if got.Library.ID != want.Library.ID {
		t.Errorf("Library.ID = %v, want %v", got.Library.ID, want.Library.ID)
	}
	if got.Storage.LinkMode != blob.LinkReflink {
		t.Errorf("Storage.LinkMode = %q, want %q", got.Storage.LinkMode, blob.LinkReflink)
	}
}

func TestDefaultGeneratesLibraryID(t *testing.T) {
	// spec 3.12: op ids become (instance_id, seq) once devices sync. Minting
	// the instance id now is free; backfilling it later is a migration plus a
	// guess about which library is which.
	a := Default(blob.LinkCopy)
	b := Default(blob.LinkCopy)

	if a.Library.ID.IsZero() {
		t.Error("Default did not generate a library id")
	}
	if a.Library.ID == b.Library.ID {
		t.Error("two libraries got the same id")
	}
}

func TestSaveIsHumanReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := Save(path, Default(blob.LinkHardlink)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[library]", "[storage]", "link_mode", "hardlink"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("config file does not contain %q:\n%s", want, raw)
		}
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	// A typo that gets silently dropped leaves the user believing a setting
	// took effect.
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := os.WriteFile(path, []byte("version = 1\n[storage]\nlink_moed = \"copy\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load accepted an unknown key")
	}
}

func TestLoadRejectsFutureVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := os.WriteFile(path, []byte("version = 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load accepted a config from the future")
	}
	if !strings.Contains(err.Error(), "newer") {
		t.Errorf("error = %q, want it to explain that a newer kist wrote this", err)
	}
}

func TestLoadRejectsUnknownLinkMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := os.WriteFile(path, []byte("version = 1\n[storage]\nlink_mode = \"teleport\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load accepted an unknown link mode")
	}
}

// Remotes are hand-written into kist.toml — there is no `kist remote add` —
// so the file is the interface and it has to be checked rather than trusted.
func TestLoadReadsRemotes(t *testing.T) {
	path := writeConfig(t, `
version = 1

[library]
id = "01234567-89ab-7def-8123-456789abcdef"

[storage]
link_mode = "hardlink"

[[remote]]
name = "外置硬盘"
path = "/Volumes/Archive/kist/blobs"

[[remote]]
name = "nas"
path = "/mnt/nas/kist/blobs"
`)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Remotes) != 2 {
		t.Fatalf("read %d remotes, want 2", len(c.Remotes))
	}
	if c.Remotes[0].Name != "外置硬盘" || c.Remotes[0].Path != "/Volumes/Archive/kist/blobs" {
		t.Errorf("first remote = %+v", c.Remotes[0])
	}
}

func TestLoadRejectsBadRemotes(t *testing.T) {
	cases := map[string]string{
		"no name":     "[[remote]]\npath = \"/a\"\n",
		"no path":     "[[remote]]\nname = \"a\"\n",
		"a duplicate": "[[remote]]\nname = \"a\"\npath = \"/a\"\n\n[[remote]]\nname = \"a\"\npath = \"/b\"\n",
	}
	for what, body := range cases {
		path := writeConfig(t, "version = 1\n\n"+body)
		if _, err := Load(path); err == nil {
			t.Errorf("a config with %s was accepted", what)
		}
	}
}

// Round-tripping matters because kist rewrites the file when it re-probes the
// link mode, and losing somebody's remotes that way would be silent.
func TestSaveKeepsRemotes(t *testing.T) {
	c := Default(blob.LinkHardlink)
	c.Remotes = []Remote{{Name: "backup", Path: "/backup/blobs"}}

	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Remotes) != 1 || back.Remotes[0].Name != "backup" {
		t.Errorf("remotes did not survive a save/load: %+v", back.Remotes)
	}
}
