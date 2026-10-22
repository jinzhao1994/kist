package library

import (
	"bytes"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientConflictDeletionReviewAndUndo(t *testing.T) {
	for _, kind := range []string{"box", "folder", "file", "fileset-member"} {
		t.Run(kind, func(t *testing.T) {
			ctx := t.Context()
			nas, box := openWithBox(t)
			if _, err := nas.Import(ctx, writeTree(t, map[string]string{"docs/one.txt": "one", "outside.txt": "outside"}), ImportOptions{Box: box}); err != nil {
				t.Fatal(err)
			}
			files := listAll(t, nas, box)
			var target domain.FileID
			for _, f := range files {
				if f.Name == "one.txt" {
					target = f.ID
				}
			}
			f, _, err := nas.File(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			folder := f.FolderID
			if kind == "fileset-member" {
				set := domain.Fileset{FolderID: folder, Kind: "document_pages", Layout: "directory", Members: []domain.FilesetMember{{FileID: target, Role: "page"}}}
				if _, err := nas.SetFileset(ctx, box, folder, nil, &set); err != nil {
					t.Fatal(err)
				}
			}
			c := newSyncClient(t, nas)
			var ref SyncReference
			if kind == "box" {
				p, err := c.PreviewBoxDeletion(ctx, box)
				if err != nil {
					t.Fatal(err)
				}
				ref, err = c.DeleteBox(ctx, box, p.Revision)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				id, isFolder := target.ID, false
				if kind == "folder" {
					id, isFolder = folder.ID, true
				}
				p, err := c.PreviewEntryDeletion(ctx, box, id, isFolder)
				if err != nil {
					t.Fatal(err)
				}
				ref, err = c.DeleteEntry(ctx, box, id, isFolder, p.Revision)
				if err != nil {
					t.Fatal(err)
				}
			}
			original, err := c.LocalHistoryOp(ctx, ref.LocalID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = nas.RenameEntry(ctx, box, target.ID, false, "one.txt", "changed.txt"); err != nil {
				t.Fatal(err)
			}
			if _, err = nas.Import(ctx, writeTree(t, map[string]string{"docs/new.txt": "new"}), ImportOptions{Box: box}); err != nil {
				t.Fatal(err)
			}
			if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err == nil {
				t.Fatal("expected deletion conflict")
			}
			draft, err := c.ConflictEdit(ctx, ref.LocalID)
			if err != nil || draft.Replacement == nil || draft.Deletion == nil || draft.Deletion.Current == nil {
				t.Fatal(draft, err)
			}
			scope := draft.Deletion
			extra := 0
			if kind == "box" || kind == "folder" {
				extra = 1
			}
			if scope.Current.FileCount != scope.Before.FileCount+extra {
				t.Fatal(scope)
			}
			if kind == "file" || kind == "fileset-member" {
				if scope.Before.Name != "one.txt" || scope.Current.Name != "changed.txt" {
					t.Fatal(scope)
				}
			}
			if draft.Replacement.Deletion == nil || draft.Replacement.Version != 2 || draft.Replacement.ID == ref.LocalID {
				t.Fatal(draft.Replacement)
			}
			if _, err = nas.UpdateBox(ctx, box, BoxFields{Name: "newer NAS title"}); err != nil {
				t.Fatal(err)
			}
			if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err == nil {
				t.Fatal("expected newer conflict")
			}
			if err = c.Resolve(ctx, ref.LocalID, draft.Replacement, "reviewed deletion"); err == nil {
				t.Fatal("stale deletion accepted")
			}
			draft, err = c.ConflictEdit(ctx, ref.LocalID)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err = c.Resolve(ctx, ref.LocalID, draft.Replacement, "reviewed deletion"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
				t.Fatal(err)
			}
			if _, found, err := nas.File(ctx, target); err != nil || found {
				t.Fatal("target not deleted", found, err)
			}
			old, err := c.LocalHistoryOp(ctx, ref.LocalID)
			if err != nil || old.State != "replaced" || !bytes.Equal(old.Proposal.Payload, original.Proposal.Payload) {
				t.Fatal(old, err)
			}
			if _, err = c.Undo(ctx, SyncReference{LocalID: draft.Replacement.ID}); err != nil {
				t.Fatal(err)
			}
			if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
				t.Fatal(err)
			}
			if err = nas.Rebuild(ctx); err != nil {
				t.Fatal(err)
			}
			restored, found, err := nas.File(ctx, target)
			if err != nil || !found || restored.Name != "changed.txt" {
				t.Fatal(restored, found, err)
			}
			if got := listAll(t, nas, box); len(got) != 3 {
				t.Fatalf("reviewed scope not restored: %+v", got)
			}
			if kind == "fileset-member" {
				sets, err := nas.Filesets(ctx)
				if err != nil || len(sets) != 1 || len(sets[0].Members) != 1 || sets[0].Members[0].FileID != target {
					t.Fatal(sets, err)
				}
			}
		})
	}
}

func TestClientConflictDeletionMissingTarget(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "one.txt", "one")
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
	current, err := nas.PreviewEntryDeletion(ctx, box, f.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = nas.DeleteEntry(ctx, box, f.ID.ID, false, current.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err == nil {
		t.Fatal("expected missing target conflict")
	}
	draft, err := c.ConflictEdit(ctx, ref.LocalID)
	if err != nil || draft.Deletion == nil || draft.Deletion.Current != nil || draft.Replacement != nil {
		t.Fatal(draft, err)
	}
}
