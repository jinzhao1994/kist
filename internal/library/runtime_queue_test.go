package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
)

func TestRuntimeQueueMigratesOnceAndSurvivesArchiveRebuild(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	q := l.OCRQueue()
	j, err := q.resolve(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	j.State = "failed"
	j.Error = "synthetic failure"
	j.RequestID = uuid.NewString()
	legacy := ocrQueueState{Paused: true, Jobs: []OCRJob{j}}
	b, _ := json.Marshal(legacy)
	if err = os.MkdirAll(filepath.Dir(q.path()), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(q.path(), b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := q.load()
	if err != nil || len(s.Jobs) != 1 || !s.Paused {
		t.Fatal("migration lost state", err)
	}
	if err = l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, err := q.FileStatus(t.Context(), id); err != nil || got.Error != j.Error {
		t.Fatal("archive rebuild lost runtime state", err)
	}
	s.Jobs = nil
	if err = q.save(s); err != nil {
		t.Fatal(err)
	}
	other, err := Open(l.layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	got, err := other.OCRQueue().load()
	if err != nil || len(got.Jobs) != 0 {
		t.Fatal("old JSON revived removed task", err)
	}
	if after, err := os.ReadFile(q.path()); err != nil || string(after) != string(b) {
		t.Fatal("legacy backup changed", err)
	}
}

func TestRuntimeMissingDatabaseDoesNotReviveLegacyState(t *testing.T) {
	l := openTestLibrary(t)
	if _, err := l.OCRQueue().Status(t.Context()); err != nil {
		t.Fatal(err)
	}
	root := l.layout.Root
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "runtime.db"), filepath.Join(root, "runtime.db.backup")); err != nil {
		t.Fatal(err)
	}
	other, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err = other.OCRQueue().Status(t.Context()); err == nil {
		t.Fatal("silently created empty runtime state")
	}
	if _, err = os.Stat(filepath.Join(root, "runtime.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("recreated missing state", err)
	}
}
func TestRuntimeBindingAndOldBinaryGuard(t *testing.T) {
	l := openTestLibrary(t)
	if _, err := l.runtimeStore(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(l.layout.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "runtime_storage = 1") {
		t.Fatal("missing downgrade guard")
	}
	// A transplanted runtime database must not apply another instance's queues.
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	if err = config.Update(l.layout.ConfigPath(), "", map[string]any{"library.id": domain.NewID().String()}); err != nil {
		t.Fatal(err)
	}
	other, err := Open(l.layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err = other.runtimeStore(); err == nil {
		t.Fatal("accepted another instance's runtime state")
	}
}
