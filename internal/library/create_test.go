package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestCreateBuildsTheWholeLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	res, err := Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, p := range []string{
		res.Layout.Blobs(),
		filepath.Join(res.Layout.Blobs(), "tmp"),
		res.Layout.Boxes(),
		res.Layout.Derived(),
	} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("missing directory %s: %v", p, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", p)
		}
	}
	for _, p := range []string{res.Layout.DB(), res.Layout.ConfigPath(), res.Layout.Notice()} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing file %s: %v", p, err)
		}
	}
}

func TestCreateMigratesTheDatabase(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	res, err := Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	db, err := store.Open(res.Layout.DB())
	if err != nil {
		t.Fatalf("open the created database: %v", err)
	}
	defer db.Close()

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='ops'`).Scan(&n); err != nil {
		t.Fatalf("query schema: %v", err)
	}
	if n != 1 {
		t.Error("the ops table is missing; migrations did not run")
	}
}

func TestCreateRecordsTheProbe(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	res, err := Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if res.Probe.LinkMode == "" {
		t.Error("Create did not probe the link mode")
	}
	loaded, err := config.Load(res.Layout.ConfigPath())
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	if loaded.Storage.LinkMode != res.Probe.LinkMode {
		t.Errorf("config records %q but the probe found %q", loaded.Storage.LinkMode, res.Probe.LinkMode)
	}
}

func TestNoticeExplainsWhatNotToTouch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	res, err := Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	raw, err := os.ReadFile(res.Layout.Notice())
	if err != nil {
		t.Fatal(err)
	}
	// It has to name what is authoritative and what is regenerable, or it is
	// just decoration (spec 3.2).
	for _, want := range []string{"blobs/", "kist.db", "boxes/", "kist relink"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("DO-NOT-EDIT.txt does not mention %q", want)
		}
	}
}

func TestCreateRefusesAnExistingLibrary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := Create(context.Background(), root)
	if !errors.Is(err, ErrAlreadyLibrary) {
		t.Errorf("second Create returned %v, want ErrAlreadyLibrary", err)
	}
}

func TestCreateRefusesANonEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "holiday.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Create(context.Background(), root)
	if !errors.Is(err, ErrNotEmpty) {
		t.Errorf("Create into an occupied directory returned %v, want ErrNotEmpty", err)
	}
}

func TestCreateAcceptsAnEmptyExistingDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(context.Background(), root); err != nil {
		t.Errorf("Create into an empty existing directory: %v", err)
	}
}

func TestOpenLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatalf("Create: %v", err)
	}

	l, err := OpenLayout(root)
	if err != nil {
		t.Fatalf("OpenLayout: %v", err)
	}
	if l.Root == "" {
		t.Error("OpenLayout returned an empty root")
	}

	if _, err := OpenLayout(t.TempDir()); err == nil {
		t.Error("OpenLayout accepted a directory that is not a library")
	}
}
