package library

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
)

func TestRuntimeMediaMigrationRetainsBackupAndDispatches(t *testing.T) {
	l, id, _ := mediaFixture(t)
	m := l.Media()
	j, err := m.resolve(t.Context(), id, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	j.State = "queued"
	j.Generation = "legacy-generation"
	j.UpdatedAt = time.Now().UTC()
	path, _, err := m.paths(j)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = writeClientJSON(path, j); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := m.claim(t.Context())
	if err != nil || !ok || got.Generation != j.Generation {
		t.Fatal(got, ok, err)
	}
	if after, err := os.ReadFile(path); err != nil || string(before) != string(after) {
		t.Fatal("legacy job was rewritten", err)
	}
	other, err := Open(l.layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	got, err = other.Media().Status(t.Context(), id, derive.Thumbnail)
	if err != nil || got.State != "running" {
		t.Fatal("reloaded stale JSON", got, err)
	}
}

func TestRuntimeCacheMigrationAtomicAndPreservesPins(t *testing.T) {
	c, err := CreateClient(t.Context(), t.TempDir(), ClientSettings{LibraryID: domain.NewID()})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	content := []byte("synthetic offline content")
	key := fmt.Sprintf("sha256:%x", sha256.Sum256(content))
	root := filepath.Join(c.root, "cache", "objects")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	record := cachedOriginal{Version: 1, Key: key, Size: int64(len(content)), Offset: int64(len(content)), Complete: true, AccessedAt: time.Now().UTC()}
	path := c.cachePath(key, ".json")
	if err := writeClientJSON(path, record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.cachePath(key, ".data"), content, 0600); err != nil {
		t.Fatal(err)
	}
	pinPath := filepath.Join(c.root, "cache-pins.json")
	if err := os.WriteFile(pinPath, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CacheStatus(t.Context()); err == nil {
		t.Fatal("damaged legacy state was silently ignored")
	}
	db, err := c.runtimeStore()
	if err != nil {
		t.Fatal(err)
	}
	var imported int
	if err = db.DB.QueryRow(`SELECT count(*) FROM migrations WHERE source='client-cache'`).Scan(&imported); err != nil || imported != 0 {
		t.Fatal("failed migration marked complete", err)
	}
	pins := cachePins{Version: 1, Owners: map[string][]string{"trip": {key}}}
	if err = writeClientJSON(pinPath, pins); err != nil {
		t.Fatal(err)
	}
	usage, err := c.CacheStatus(t.Context())
	if err != nil || usage.PinnedBytes != int64(len(content)) {
		t.Fatal(usage, err)
	}
	if err = c.SetOfflineOriginals(t.Context(), "trip", nil); err != nil {
		t.Fatal(err)
	}
	old := cachePins{}
	raw, err := os.ReadFile(pinPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &old); err != nil || len(old.Owners) != 1 {
		t.Fatal("backup was altered", err)
	}
	other, err := OpenClient(t.Context(), c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	usage, err = other.CacheStatus(t.Context())
	if err != nil || usage.PinnedBytes != 0 || usage.CachedBytes != int64(len(content)) {
		t.Fatal("old pin revived", usage, err)
	}
}
