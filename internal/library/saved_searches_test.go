package library

import (
	"errors"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSavedSearchLifecycleReplayAndDrift(t *testing.T) {
	l, _ := openWithBox(t)
	ctx := t.Context()
	definition := domain.SearchDefinition{Version: 1, Entity: "files", Filters: domain.SearchFilters{Sort: "captured", TagNot: []string{domain.NewTagID().String()}}}
	s, err := l.CreateSavedSearch(ctx, "Contracts", definition)
	if err != nil {
		t.Fatal(err)
	}
	created, _, _ := l.ops.Last()
	if err := l.UpdateSavedSearch(ctx, s, "Agreements", definition); err != nil {
		t.Fatal(err)
	}
	updated, _, _ := l.ops.Last()
	if _, err := l.Undo(ctx, created.ID); !errors.Is(err, ErrUndoBlocked) {
		t.Fatalf("stale undo: %v", err)
	}
	if err := l.ViewAt(ctx, created.ID, func(v *HistoricalView) error {
		items, err := v.SavedSearches(ctx)
		if err == nil && (len(items) != 1 || items[0].Name != "Contracts") {
			t.Fatalf("historical searches: %v", items)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(ctx, updated.ID); err != nil {
		t.Fatal(err)
	}
	if err := l.DeleteSavedSearch(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	deleted, _, _ := l.ops.Last()
	if items, _ := l.SavedSearches(ctx); len(items) != 0 {
		t.Fatal(items)
	}
	if _, err := l.Undo(ctx, deleted.ID); err != nil {
		t.Fatal(err)
	}
	items, _ := l.SavedSearches(ctx)
	if !reflect.DeepEqual(items, []domain.SavedSearch{s}) {
		t.Fatalf("restored: %#v", items)
	}
	before, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Snapshot(l.db)
	if before != after {
		t.Fatal("rebuild changed saved search state")
	}
	root := t.TempDir()
	if _, err := Create(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(root, "ops"), os.DirFS(l.layout.Ops())); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	recoveredItems, err := recovered.SavedSearches(ctx)
	if err != nil || !reflect.DeepEqual(items, recoveredItems) {
		t.Fatalf("catch-up: %v %v", recoveredItems, err)
	}
	if _, err := l.db.Exec(`UPDATE saved_searches SET name='corrupt'`); err != nil {
		t.Fatal(err)
	}
	report, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) == 0 {
		t.Fatalf("drift not found: %v %v", report, err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	report, err = l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 {
		t.Fatalf("verify: %v %v", report, err)
	}
}

func TestSavedSearchRejectsDuplicateAndInvalidWithoutLogging(t *testing.T) {
	l, _ := openWithBox(t)
	ctx := t.Context()
	d := domain.SearchDefinition{Version: 1, Entity: "boxes", Filters: domain.SearchFilters{Sort: "updated"}}
	if _, err := l.CreateSavedSearch(ctx, "Trips", d); err != nil {
		t.Fatal(err)
	}
	before, _, _ := l.ops.Last()
	if _, err := l.CreateSavedSearch(ctx, "TRIPS", d); err == nil {
		t.Fatal("duplicate accepted")
	}
	d.Filters.TagAll = []string{"invalid"}
	if _, err := l.CreateSavedSearch(ctx, "Invalid", d); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	after, _, _ := l.ops.Last()
	if before.ID != after.ID {
		t.Fatal("rejected search appended an op")
	}
}
