package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestClientBoxTagConcurrentSameChangeConflicts(t *testing.T) {
	for _, remove := range []bool{false, true} {
		name := "add"
		if remove {
			name = "remove"
		}
		t.Run(name, func(t *testing.T) {
			nas, box := openWithBox(t)
			ctx := t.Context()
			tag, err := nas.CreateTag(ctx, "Travel", nil)
			if err != nil {
				t.Fatal(err)
			}
			ids := []domain.BoxID{box}
			if remove {
				if _, err := nas.TagBoxes(ctx, tag.ID, ids); err != nil {
					t.Fatal(err)
				}
			}
			c := newSyncClient(t, nas)
			local, remote := c.TagBoxes, nas.TagBoxes
			if remove {
				local, remote = c.UntagBoxes, nas.UntagBoxes
			}
			if _, err := local(ctx, tag.ID, ids); err != nil {
				t.Fatal(err)
			}
			pending, err := c.queue.snapshot(ctx)
			if err != nil || len(pending) != 1 {
				t.Fatal(pending, err)
			}
			if _, err := remote(ctx, tag.ID, ids); err != nil {
				t.Fatal(err)
			}
			before, err := nas.OpCount(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := nas.AcceptSync(ctx, pending[0].Proposal); !errors.Is(err, ErrSyncConflict) {
				t.Fatalf("same change accepted as an undoable modification: %v", err)
			}
			if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); !errors.Is(err, ErrSyncConflict) {
				t.Fatalf("overlay did not preserve conflict: %v", err)
			}
			if after, err := nas.OpCount(ctx); err != nil || after != before {
				t.Fatal("conflict appended a formal op", after, before, err)
			}
		})
	}
}

func TestFileTagRemovalChecksExactSourceAndPreservesOtherClaims(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	file := importOneFile(t, nas, box, "tagged.txt", "content")
	tag, err := nas.CreateTag(ctx, "Travel", nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := []domain.FileID{file.ID}
	if _, err := nas.TagFiles(ctx, tag.ID, ids); err != nil {
		t.Fatal(err)
	}
	other, err := oplog.Marshal(oplog.TagFilesPayload{TagID: tag.ID, FileIDs: ids, BoxIDs: []domain.BoxID{box}, Source: "camera"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nas.write(ctx, oplog.ActionTagFiles, other, oplog.OriginExtractor, writeOptions{}); err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	if _, err := c.UntagFiles(ctx, tag.ID, ids); err != nil {
		t.Fatal(err)
	}
	pending, err := c.queue.snapshot(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatal(pending, err)
	}
	receipt, err := nas.AcceptSync(ctx, pending[0].Proposal)
	if err != nil {
		t.Fatal(err)
	}
	check := func(user bool) {
		t.Helper()
		for source, want := range map[string]bool{extract.UserSource: user, "camera": true} {
			var got bool
			if err := nas.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM file_tag_sources WHERE file_id=? AND tag_id=? AND source=?)`, file.ID, tag.ID, source).Scan(&got); err != nil || got != want {
				t.Fatal(source, got, want, err)
			}
		}
	}
	check(false)
	stale := pending[0].Proposal
	stale.ID = "another-removal-of-absent-user-claim"
	if _, err := nas.AcceptSync(ctx, stale); !errors.Is(err, ErrSyncConflict) {
		t.Fatal("other source masked the missing user claim", err)
	}
	if _, err := nas.Undo(ctx, receipt.OpID); err != nil {
		t.Fatal(err)
	}
	check(true)
	before := snapshotOf(t, nas)
	if err := nas.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, nas) != before {
		t.Fatal("rebuild changed source claims")
	}
}

func TestSyncedBoxTagRetryAndStrictUndo(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	tag, err := nas.CreateTag(ctx, "Travel", nil)
	if err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	ids := []domain.BoxID{box}
	if _, err := c.TagBoxes(ctx, tag.ID, ids); err != nil {
		t.Fatal(err)
	}
	pending, err := c.queue.snapshot(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatal(pending, err)
	}
	proposal := pending[0].Proposal
	receipt, err := nas.AcceptSync(ctx, proposal)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := nas.AcceptSync(ctx, proposal); err != nil || again != receipt {
		t.Fatal("retry rechecked an already accepted before state", again, err)
	}
	if _, err := nas.UntagBoxes(ctx, tag.ID, ids); err != nil {
		t.Fatal(err)
	}
	removed, err := nas.OpCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Undo(ctx, receipt.OpID); !errors.Is(err, ErrUndoBlocked) {
		t.Fatal("undo accepted a changed association", err)
	}
	if _, err := nas.Undo(ctx, removed); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Undo(ctx, receipt.OpID); err != nil {
		t.Fatal(err)
	}
	before := snapshotOf(t, nas)
	if err := nas.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, nas) != before {
		t.Fatal("rebuild changed tag history")
	}
}

func TestClientFileTagConcurrentSameChangeConflicts(t *testing.T) {
	for _, remove := range []bool{false, true} {
		name := "add"
		if remove {
			name = "remove"
		}
		t.Run(name, func(t *testing.T) {
			nas, box := openWithBox(t)
			file := importOneFile(t, nas, box, "tagged.txt", "content")
			ctx := t.Context()
			tag, err := nas.CreateTag(ctx, "Travel", nil)
			if err != nil {
				t.Fatal(err)
			}
			ids := []domain.FileID{file.ID}
			if remove {
				if _, err := nas.TagFiles(ctx, tag.ID, ids); err != nil {
					t.Fatal(err)
				}
			}
			c := newSyncClient(t, nas)
			local, remote := c.TagFiles, nas.TagFiles
			if remove {
				local, remote = c.UntagFiles, nas.UntagFiles
			}
			if _, err := local(ctx, tag.ID, ids); err != nil {
				t.Fatal(err)
			}
			pending, err := c.queue.snapshot(ctx)
			if err != nil || len(pending) != 1 {
				t.Fatal(pending, err)
			}
			if _, err := remote(ctx, tag.ID, ids); err != nil {
				t.Fatal(err)
			}
			before, err := nas.OpCount(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := nas.AcceptSync(ctx, pending[0].Proposal); !errors.Is(err, ErrSyncConflict) {
				t.Fatalf("same change accepted as an undoable modification: %v", err)
			}
			if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); !errors.Is(err, ErrSyncConflict) {
				t.Fatalf("overlay did not preserve conflict: %v", err)
			}
			if after, err := nas.OpCount(ctx); err != nil || after != before {
				t.Fatal("conflict appended a formal op", after, before, err)
			}
		})
	}
}
