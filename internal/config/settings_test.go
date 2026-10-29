package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/blob"
)

func TestSparseDefaultsAndResetPreserveComments(t *testing.T) {
	path := writeConfig(t, "# My archive\nversion = 2\n[library]\ntimezone = 'Asia/Shanghai' # fixed interpretation\n\n[appearance]\nprimary_color = 'green' # favorite\n")
	raw, _ := os.ReadFile(path)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Listen != "127.0.0.1:8473" || c.Client.CacheMaxBytes != 10<<30 {
		t.Fatal("missing defaults")
	}
	if err = Update(path, Revision(raw), map[string]any{"appearance.primary_color": "pink", "server.max_upload_files": 123}); err != nil {
		t.Fatal(err)
	}
	next, _ := os.ReadFile(path)
	for _, comment := range []string{"# My archive", "# fixed interpretation", "# favorite"} {
		if !strings.Contains(string(next), comment) {
			t.Fatalf("lost comment %s", comment)
		}
	}
	if err = Update(path, Revision(raw), map[string]any{"appearance.primary_color": "gray"}); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale save=%v", err)
	}
	if err = Update(path, Revision(next), map[string]any{"appearance.primary_color": nil}); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil || c.Appearance.PrimaryColor != "blue" || c.Server.MaxUploadFiles != 123 {
		t.Fatalf("reset=%+v %v", c, err)
	}
	final, _ := os.ReadFile(path)
	if strings.Contains(string(final), "primary_color") {
		t.Fatal("reset persisted today's default")
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("configuration exposes credentials")
	}
}

func TestConfigRejectsInvalidUpdatesWithoutChangingFile(t *testing.T) {
	path := writeConfig(t, "version = 2\n")
	for key, value := range map[string]any{"appearance.primary_color": "invalid", "server.listen": "0.0.0.0:8473", "client.cache_max_age": "-1h", "server.max_upload_files": 0, "unknown": true, "geocoding.request_interval_seconds": -1} {
		raw, _ := os.ReadFile(path)
		if err := Update(path, Revision(raw), map[string]any{key: value}); err == nil {
			t.Fatalf("accepted %s", key)
		}
		after, _ := os.ReadFile(path)
		if string(raw) != string(after) {
			t.Fatalf("failed edit changed %s", key)
		}
	}
}

func TestNewConfigDoesNotExpandDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	cfg := Default(blob.LinkCopy)
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, key := range []string{"cache_max_age", "max_upload_bytes", "primary_color", "token"} {
		if strings.Contains(string(raw), key) {
			t.Fatalf("serialized default %s", key)
		}
	}
	got, err := Load(path)
	if err != nil || got.Library.ID != cfg.Library.ID || got.Library.Timezone != cfg.Library.Timezone {
		t.Fatalf("lost pinned facts: %v", err)
	}
}

func TestSparseTimezoneIsPortableAndSyntaxErrorsHideSecrets(t *testing.T) {
	cfg, err := Decode([]byte("version = 2\n"))
	if err != nil || cfg.Library.Timezone != "UTC" {
		t.Fatalf("default timezone: %v %v", cfg, err)
	}
	_, err = Decode([]byte("[client]\ntoken = secret-that-must-not-be-logged\n"))
	if err == nil || strings.Contains(err.Error(), "secret-that") {
		t.Fatalf("unsafe parse error: %v", err)
	}
	for _, listen := range []string{"127.0.0.1:wat", "127.0.0.1:65536", "127.0.0.1:-1"} {
		cfg.Server.Listen = listen
		if cfg.Validate() == nil {
			t.Fatalf("accepted %s", listen)
		}
	}
}

func TestSettingsByteSizes(t *testing.T) {
	for _, key := range []string{"server.max_upload_bytes", "client.cache_max_bytes"} {
		for input, want := range map[string]int64{"32 GB": 32000000000, "32G": 32000000000, "32 GiB": 32 << 30, "1.5 MB": 1500000, " 512 mib ": 512 << 20} {
			changes, err := NormalizeChanges("client", map[string]any{key: input})
			if err != nil || changes[key] != want {
				t.Fatalf("%s %q: %v, %v", key, input, changes, err)
			}
		}
		for _, input := range []string{"", "oops", "32 GB junk", "-1 GB", "0 GB", "999999999 PB"} {
			if _, err := NormalizeChanges("client", map[string]any{key: input}); err == nil {
				t.Fatalf("accepted %s=%q", key, input)
			}
		}
	}
	path := writeConfig(t, "version = 2\n")
	raw, _ := os.ReadFile(path)
	changes, err := NormalizeChanges("library", map[string]any{"server.max_upload_bytes": "32G"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Update(path, Revision(raw), changes); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.Server.MaxUploadBytes != 32000000000 {
		t.Fatalf("saved size: %+v %v", cfg, err)
	}
	if _, err := NormalizeChanges("library", map[string]any{"server.max_upload_files": "32G"}); err == nil {
		t.Fatal("file count accepted byte units")
	}
}

func TestLibraryLanguageDefaultsAndValidation(t *testing.T) {
	path := writeConfig(t, "version = 2\n")
	c, err := Load(path)
	if err != nil || c.Library.Language != "zh-CN" {
		t.Fatal(c.Library.Language, err)
	}
	for _, value := range []string{"fr", "en-GB", "zh-Hant"} {
		if err = Update(path, "", map[string]any{"library.language": value}); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"", "not a language", strings.Repeat("x", 65)} {
		if err = Update(path, "", map[string]any{"library.language": value}); err == nil {
			t.Fatal("invalid language accepted")
		}
	}
}

func TestNewArchiveLocationDefaultPreservesExistingChoices(t *testing.T) {
	c := Default(blob.LinkCopy)
	if !c.Geocoding.Automatic {
		t.Fatal("new archive location lookup is disabled")
	}
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || !loaded.Geocoding.Automatic {
		t.Fatal("new archive lost its location default", err)
	}
	for _, raw := range []string{"version = 2\n", "version = 2\n[geocoding]\nautomatic = false\n"} {
		old, err := Decode([]byte(raw))
		if err != nil || old.Geocoding.Automatic {
			t.Fatal("existing archive was implicitly enabled", err)
		}
	}
}
