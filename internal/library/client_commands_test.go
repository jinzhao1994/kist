package library

import (
	"context"
	"errors"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/query"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientBoxCommandsMergeIndependentNASFieldAndRetry(t *testing.T) {
	nas, id := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	original, found, err := nas.Box(ctx, id)
	if err != nil || !found {
		t.Fatal(err)
	}
	request := Request{ID: "client-update-box", Digest: strings.Repeat("a", 64)}
	result, err := c.ExecuteRequest(ctx, request, func(scoped context.Context) error {
		updated, err := c.UpdateBox(scoped, id, BoxFields{Name: "offline rename", Description: original.Description, StartAt: original.StartAt, EndAt: original.EndAt})
		if err == nil && updated.Name != "offline rename" {
			t.Fatal("local edit invisible")
		}
		return err
	})
	if err != nil || result.LocalID == "" {
		t.Fatalf("local command=%+v %v", result, err)
	}
	if _, err := nas.UpdateBox(ctx, id, BoxFields{Name: original.Name, Description: "independent NAS description", StartAt: original.StartAt, EndAt: original.EndAt}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	merged, _, err := nas.Box(ctx, id)
	if err != nil || merged.Name != "offline rename" || merged.Description != "independent NAS description" {
		t.Fatalf("lost independent change=%+v %v", merged, err)
	}
	retry, err := c.ExecuteRequest(ctx, request, func(context.Context) error { t.Fatal("replayed command"); return nil })
	if err != nil || retry != result {
		t.Fatalf("retry=%+v %v", retry, err)
	}
}

func TestClientCreateBoxAndNoopCommand(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	var created domain.Box
	result, err := c.ExecuteRequest(ctx, Request{ID: "client-create-box", Digest: strings.Repeat("b", 64)}, func(scoped context.Context) error {
		var err error
		created, err = c.CreateBox(scoped, "Travel", "notes", domain.LocalDate{}, domain.LocalDate{})
		return err
	})
	if err != nil || created.ID.IsZero() || result.LocalID == "" {
		t.Fatalf("create=%+v %+v %v", created, result, err)
	}
	noop, err := c.ExecuteRequest(ctx, Request{ID: "client-update-noop", Digest: strings.Repeat("c", 64)}, func(scoped context.Context) error {
		_, err := c.UpdateBox(scoped, created.ID, BoxFields{Name: created.Name, Description: created.Description, StartAt: created.StartAt, EndAt: created.EndAt})
		return err
	})
	if err != nil || !noop.Noop {
		t.Fatalf("noop=%+v %v", noop, err)
	}
	if _, err := c.UpdateBox(ctx, created.ID, BoxFields{Name: "Travel changed", Description: created.Description}); err != nil {
		t.Fatal(err)
	}
	synced, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || synced.Accepted != 2 {
		t.Fatalf("sync=%+v %v", synced, err)
	}
	got, found, err := nas.Box(ctx, created.ID)
	if err != nil || !found || got.Name != "Travel changed" {
		t.Fatalf("NAS box=%+v %v", got, err)
	}
}

func TestClientFilesetCommandRejectsStaleAndConcurrentConfiguration(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	if _, err := nas.Import(ctx, writeTree(t, map[string]string{"pages/one.jpg": "one", "pages/two.jpg": "two"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	folders, err := nas.Folders(ctx, box)
	if err != nil {
		t.Fatal(err)
	}
	var folder domain.FolderID
	for _, f := range folders {
		if f.Name == "pages" {
			folder = f.ID
		}
	}
	files, _, err := nas.Files(ctx, query.Query{})
	if err != nil {
		t.Fatal(err)
	}
	before := domain.Fileset{FolderID: folder, Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FileID: files[0].ID, Role: "page"}, {FileID: files[1].ID, Role: "page"}}}
	c := newSyncClient(t, nas)
	request := Request{ID: "client-create-fileset", Digest: strings.Repeat("d", 64)}
	result, err := c.ExecuteRequest(ctx, request, func(scoped context.Context) error {
		ref, err := c.SetFileset(scoped, box, folder, nil, &before)
		if err == nil && (ref.LocalID == "" || ref.OpID != 0) {
			t.Fatal("invalid local reference")
		}
		return err
	})
	if err != nil || result.LocalID == "" {
		t.Fatalf("create=%+v %v", result, err)
	}
	if _, err := c.SetFileset(ctx, box, folder, nil, &before); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatalf("stale config=%v", err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	local := before
	local.Members = []domain.FilesetMember{before.Members[1], before.Members[0]}
	ref, err := c.SetFileset(ctx, box, folder, &before, &local)
	if err != nil {
		t.Fatal(err)
	}
	remote := before
	remote.Layout = "directory"
	if _, err := nas.SetFileset(ctx, box, folder, &before, &remote); err != nil {
		t.Fatal(err)
	}
	synced, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if !errors.Is(err, ErrSyncConflict) || synced.BlockedID != ref.LocalID {
		t.Fatalf("configuration conflict=%+v %v", synced, err)
	}
	got, err := nas.Filesets(ctx)
	if err != nil || len(got) != 1 || !domain.EqualFileset(&got[0], &remote) {
		t.Fatalf("overwrote NAS config=%+v %v", got, err)
	}
}

func TestClientFolderCommandPreservesOwnershipAndDuplicateChecks(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	other, err := nas.CreateBox(ctx, "Other box", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	ref, err := c.CreateFolder(ctx, box, domain.FolderID{}, "Pages")
	if err != nil || ref.LocalID == "" || ref.OpID != 0 {
		t.Fatalf("folder=%+v %v", ref, err)
	}
	if _, err := c.CreateFolder(ctx, box, domain.FolderID{}, "pages"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("duplicate folder=%v", err)
	}
	if _, err := c.CreateFolder(ctx, box, other.RootFolderID, "Wrong box"); !errors.Is(err, ErrFolderNotInBox) {
		t.Fatalf("cross-box folder=%v", err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || result.Accepted != 1 {
		t.Fatalf("sync=%+v %v", result, err)
	}
	folders, err := nas.Folders(ctx, box)
	if err != nil || len(folders) != 2 {
		t.Fatalf("folders=%+v %v", folders, err)
	}
}
