package library

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientSavedSearchLifecycleAndRequestRetry(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	definition := domain.SearchDefinition{Version: 2, Entity: "files", Filters: domain.SearchFilters{Sort: "name", IncludeMembers: true, Keywords: []string{"passport"}}}
	var search domain.SavedSearch
	request := Request{ID: "create-offline-search", Digest: strings.Repeat("a", 64)}
	result, err := c.ExecuteRequest(ctx, request, func(ctx context.Context) error {
		var err error
		search, err = c.CreateSavedSearch(ctx, "Documents", definition)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	retry, err := c.ExecuteRequest(ctx, request, func(context.Context) error { t.Fatal("request repeated"); return nil })
	if err != nil || retry != result {
		t.Fatalf("retry=%+v %v", retry, err)
	}
	if err := c.UpdateSavedSearch(ctx, search, search.Name, search.Definition); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateSavedSearch(ctx, search, "Travel documents", definition); err != nil {
		t.Fatal(err)
	}
	updated, err := c.SavedSearches(ctx)
	if err != nil || len(updated) != 1 || updated[0].Name != "Travel documents" {
		t.Fatalf("local=%+v %v", updated, err)
	}
	deleted, err := c.ExecuteRequest(ctx, Request{ID: "delete-offline-search", Digest: strings.Repeat("b", 64)}, func(ctx context.Context) error { return c.DeleteSavedSearch(ctx, search.ID) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Undo(ctx, SyncReference{LocalID: deleted.LocalID}); err != nil {
		t.Fatal(err)
	}
	synced, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || synced.Accepted != 4 {
		t.Fatalf("sync=%+v %v", synced, err)
	}
	got, err := nas.SavedSearches(ctx)
	if err != nil || !reflect.DeepEqual(got, updated) {
		t.Fatalf("NAS=%+v %v", got, err)
	}
	if err := nas.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = nas.SavedSearches(ctx)
	if err != nil || !reflect.DeepEqual(got, updated) {
		t.Fatalf("rebuild=%+v %v", got, err)
	}
}

func TestClientSavedSearchConcurrentNASChangeConflicts(t *testing.T) {
	nas := openTestLibrary(t)
	ctx := t.Context()
	definition := domain.SearchDefinition{Version: 1, Entity: "files", Filters: domain.SearchFilters{Sort: "captured"}}
	search, err := nas.CreateSavedSearch(ctx, "Original", definition)
	if err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	if err := c.UpdateSavedSearch(ctx, search, "Laptop", definition); err != nil {
		t.Fatal(err)
	}
	if err := nas.UpdateSavedSearch(ctx, search, "NAS", definition); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("concurrent edit=%v", err)
	}
	got, err := nas.SavedSearches(ctx)
	if err != nil || len(got) != 1 || got[0].Name != "NAS" {
		t.Fatalf("NAS overwritten: %+v %v", got, err)
	}
}

func TestClientSavedSearchReadsAcrossHandlesAndWithinRequest(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	second, err := OpenClient(ctx, c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	definition := domain.SearchDefinition{Version: 1, Entity: "files", Filters: domain.SearchFilters{Sort: "captured"}}
	var saved domain.SavedSearch
	var escaped context.Context
	_, err = second.ExecuteRequest(ctx, Request{ID: "search-read-after-write", Digest: strings.Repeat("c", 64)}, func(scoped context.Context) error {
		escaped = scoped
		var err error
		saved, err = second.CreateSavedSearch(scoped, "Recent", definition)
		if err != nil {
			return err
		}
		got, err := second.SavedSearches(scoped)
		if err == nil && (len(got) != 1 || got[0].ID != saved.ID) {
			t.Fatalf("scoped read=%+v", got)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.SavedSearches(ctx)
	if err != nil || len(got) != 1 || got[0].ID != saved.ID {
		t.Fatalf("other handle read=%+v %v", got, err)
	}
	if _, err := second.SavedSearches(escaped); !errors.Is(err, ErrRequestScope) {
		t.Fatalf("expired scope read=%v", err)
	}
}
