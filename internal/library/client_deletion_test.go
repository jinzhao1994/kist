package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestClientDeletionConfirmationSurvivesAcceptanceAndRestores(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "before.txt", "original")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	if _, err := c.RenameEntry(ctx, box, f.ID.ID, false, "before.txt", "after.txt"); err != nil {
		t.Fatal(err)
	}
	preview, err := c.PreviewEntryDeletion(ctx, box, f.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	// Another box advances formal IDs without changing the confirmed scope.
	if _, err := nas.CreateBox(ctx, "other", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	after, err := c.PreviewEntryDeletion(ctx, box, f.ID.ID, false)
	if err != nil || preview.Revision != after.Revision {
		t.Fatalf("acceptance changed confirmation: %+v %+v %v", preview, after, err)
	}
	ref, err := c.DeleteEntry(ctx, box, f.ID.ID, false, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := nas.File(ctx, f.ID); err != nil || found {
		t.Fatalf("not deleted: %t %v", found, err)
	}
	if _, err := c.Undo(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	restored, found, err := nas.File(ctx, f.ID)
	if err != nil || !found || restored.Name != "after.txt" {
		t.Fatalf("restore=%+v %t %v", restored, found, err)
	}
}

func TestClientDeletionConfirmationRejectsSameBoxEdit(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "one.txt", "one")
	other := importOneFile(t, nas, box, "two.txt", "two")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	p, err := c.PreviewEntryDeletion(ctx, box, f.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.RenameEntry(ctx, box, other.ID.ID, false, "two.txt", "changed.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteEntry(ctx, box, f.ID.ID, false, p.Revision); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatalf("stale confirmation=%v", err)
	}
}

func TestClientBoxDeletionAndOfflineRestore(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "file.txt", "original")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	p, err := c.PreviewBoxDeletion(ctx, box)
	if err != nil || p.FileCount != 1 {
		t.Fatalf("preview=%+v %v", p, err)
	}
	ref, err := c.DeleteBox(ctx, box, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Undo(ctx, ref); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || result.Accepted != 2 {
		t.Fatalf("sync=%+v %v", result, err)
	}
	if _, found, err := nas.File(ctx, f.ID); err != nil || !found {
		t.Fatalf("restore=%t %v", found, err)
	}
}

func TestClientDeletionAfterPendingRenameSynchronizes(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "before.txt", "original")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	if _, err := c.RenameEntry(ctx, box, f.ID.ID, false, "before.txt", "after.txt"); err != nil {
		t.Fatal(err)
	}
	preview, err := c.PreviewEntryDeletion(ctx, box, f.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteEntry(ctx, box, f.ID.ID, false, preview.Revision); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || result.Accepted != 2 {
		t.Fatalf("local rename then delete must not conflict with its own acceptance: %+v %v", result, err)
	}
}

func TestClientDeletionRejectsNASChangeEvenWhenReverted(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "file.txt", "original")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	p, err := c.PreviewEntryDeletion(ctx, box, f.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := c.DeleteEntry(ctx, box, f.ID.ID, false, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := nas.RenameEntry(ctx, box, f.ID.ID, false, "file.txt", "other.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Undo(ctx, changed); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if !errors.Is(err, ErrSyncConflict) || result.BlockedID != ref.LocalID {
		t.Fatalf("ABA bypassed confirmation: %+v %v", result, err)
	}
	if _, found, err := nas.File(ctx, f.ID); err != nil || !found {
		t.Fatalf("concurrent file lost: %t %v", found, err)
	}
}

func TestClientDeletionNestedFilesetAndRestore(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	if _, err := nas.Import(ctx, writeTree(t, map[string]string{"doc/page/image.jpg": "image", "doc/page/video.mov": "video", "doc/other.jpg": "other"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	folders, err := nas.Folders(ctx, box)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]domain.FolderID{}
	for _, f := range folders {
		ids[f.Name] = f.ID
	}
	files := listAll(t, nas, box)
	fileIDs := map[string]domain.FileID{}
	for _, f := range files {
		fileIDs[f.Name] = f.ID
	}
	child := domain.Fileset{FolderID: ids["page"], Kind: "live_photo", Layout: "flat", Members: []domain.FilesetMember{{FileID: fileIDs["image.jpg"], Role: "image"}, {FileID: fileIDs["video.mov"], Role: "video"}}}
	parent := domain.Fileset{FolderID: ids["doc"], Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FolderID: child.FolderID, Role: "page"}, {FileID: fileIDs["other.jpg"], Role: "page"}}}
	for _, set := range []domain.Fileset{child, parent} {
		if _, err := nas.SetFileset(ctx, box, set.FolderID, nil, &set); err != nil {
			t.Fatal(err)
		}
	}
	c := newSyncClient(t, nas)
	p, err := c.PreviewEntryDeletion(ctx, box, child.FolderID.ID, true)
	if err != nil || p.FileCount != 2 {
		t.Fatalf("preview=%+v %v", p, err)
	}
	ref, err := c.DeleteEntry(ctx, box, child.FolderID.ID, true, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	sets, err := nas.Filesets(ctx)
	if err != nil || len(sets) != 1 || len(sets[0].Members) != 1 {
		t.Fatalf("sets=%+v %v", sets, err)
	}
	if _, err := c.Undo(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	sets, err = nas.Filesets(ctx)
	if err != nil || len(sets) != 2 {
		t.Fatalf("restore=%+v %v", sets, err)
	}
	if err := nas.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	sets, err = nas.Filesets(ctx)
	if err != nil || len(sets) != 2 {
		t.Fatalf("rebuild=%+v %v", sets, err)
	}
}
