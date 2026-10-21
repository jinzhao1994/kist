package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/jinzhao1994/kist/internal/store"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

func clientImportSource(t *testing.T, name, content string) ImportSource {
	t.Helper()
	path := filepath.Join(t.TempDir(), "staged")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return ImportSource{Path: path, RelativePath: name}
}

func TestClientImportPersistsOriginalSyncsAndUndoes(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	box, err := c.CreateBox(ctx, "Travel", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	source := clientImportSource(t, "day/photo.txt", "offline original")
	prepared, err := c.PrepareFiles(ctx, []ImportSource{source}, ClientImportOptions{Box: box.ID})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Result().Files[0].Status != FileImported {
		t.Fatal("not prepared")
	}
	request := Request{ID: "offline-upload-request", Digest: strings.Repeat("a", 64)}
	receipt, err := c.ExecuteRequest(ctx, request, func(scoped context.Context) error { _, err := prepared.Commit(scoped); return err })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExecuteRequest(ctx, request, func(context.Context) error { t.Fatal("repeated import"); return nil }); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%+v %v", files, err)
	}
	if _, err := c.SetCacheLimits(ctx, time.Hour, 1); err != nil {
		t.Fatal(err)
	}
	_, reader, err := c.OpenContent(ctx, files[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(data) != "offline original" {
		t.Fatalf("local content=%q %v", data, err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	got, found, err := nas.File(ctx, files[0].ID)
	if err != nil || !found || got.BlobKey != files[0].BlobKey {
		t.Fatalf("NAS file=%+v %v %v", got, found, err)
	}
	if _, err := c.Undo(ctx, SyncReference{LocalID: receipt.LocalID}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := nas.File(ctx, files[0].ID); err != nil || found {
		t.Fatalf("undo=%v %v", found, err)
	}
}

func TestClientPreparedImportRejectsChangedDestinationAndBadHash(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	source := clientImportSource(t, "file.txt", "bytes")
	prepared, err := c.PrepareFiles(ctx, []ImportSource{source}, ClientImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	before, _, _ := c.Box(ctx, box)
	if _, err := c.UpdateBox(ctx, box, BoxFields{Name: "Changed", Description: before.Description, StartAt: before.StartAt, EndAt: before.EndAt}); err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Commit(ctx); !errors.Is(err, ErrUploadTargetChanged) {
		t.Fatalf("changed target=%v", err)
	}
	source.Expected = &ImportIntegrity{SHA256: strings.Repeat("0", 64), Size: 5}
	if _, err := c.PrepareFiles(ctx, []ImportSource{source}, ClientImportOptions{Box: box}); !errors.Is(err, ErrImportIntegrity) {
		t.Fatalf("bad hash=%v", err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 0 {
		t.Fatalf("failed preparation visible=%+v %v", files, err)
	}
}

func TestClientImportOverwriteKeepsIdentityAndSyncs(t *testing.T) {
	nas, box := openWithBox(t)
	old := importOneFile(t, nas, box, "file.txt", "old")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	prepared, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "file.txt", "new bytes")}, ClientImportOptions{Box: box, ConflictMode: "overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := prepared.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	file, found, err := c.File(ctx, old.ID)
	if err != nil || !found || file.BlobKey == old.BlobKey {
		t.Fatalf("replacement=%+v %v %v", file, found, err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || !result.Complete {
		t.Fatalf("sync=%+v %v", result, err)
	}
	if _, err := c.Undo(ctx, imported.Reference); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	restored, found, err := nas.File(ctx, old.ID)
	if err != nil || !found || restored.BlobKey != old.BlobKey {
		t.Fatalf("restored=%+v %v %v", restored, found, err)
	}
}

func TestClientImportOverwriteAfterPendingRenameConverges(t *testing.T) {
	nas, box := openWithBox(t)
	old := importOneFile(t, nas, box, "old.txt", "old bytes")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	if _, err := c.RenameEntry(ctx, box, old.ID.ID, false, old.Name, "new.txt"); err != nil {
		t.Fatal(err)
	}
	prepared, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "new.txt", "replacement")}, ClientImportOptions{Box: box, ConflictMode: "overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || !result.Complete {
		t.Fatalf("own rename caused conflict: %+v %v", result, err)
	}
}

func TestClientImportGroupAndStrictBatch(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	first, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "first.txt", "first")}, ClientImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	root, err := first.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	group := root.Reference
	next, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "second.txt", "second")}, ClientImportOptions{Box: box, Group: &group})
	if err != nil {
		t.Fatal(err)
	}
	group.LocalID = "mutated caller copy"
	member, err := next.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || !result.Complete {
		t.Fatalf("group sync=%+v %v", result, err)
	}
	_, rootID, _, found, err := store.SyncOp(nas.db, root.Reference.LocalID)
	if err != nil || !found {
		t.Fatalf("root mapping=%v %v", found, err)
	}
	_, memberID, _, found, err := store.SyncOp(nas.db, member.Reference.LocalID)
	if err != nil || !found {
		t.Fatalf("member mapping=%v %v", found, err)
	}
	entry, err := nas.HistoryOp(ctx, memberID)
	if err != nil || entry.RootOpID == nil || *entry.RootOpID != rootID {
		t.Fatalf("mapped group=%+v %v", entry, err)
	}
	good := clientImportSource(t, "strict-good.txt", "good")
	digest := sha256.Sum256([]byte("good"))
	good.Expected = &ImportIntegrity{SHA256: hex.EncodeToString(digest[:]), Size: 4}
	bad := clientImportSource(t, "strict-bad.txt", "bad")
	bad.Expected = &ImportIntegrity{SHA256: strings.Repeat("0", 64), Size: 3}
	if _, err := c.PrepareFiles(ctx, []ImportSource{good, bad}, ClientImportOptions{Box: box}); !errors.Is(err, ErrImportIntegrity) {
		t.Fatalf("strict batch=%v", err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 2 {
		t.Fatalf("partial strict batch visible=%+v %v", files, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := c.PrepareFiles(cancelled, []ImportSource{good}, ClientImportOptions{Box: box}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled prepare=%v", err)
	}
}

func TestClientImportOverwriteDoesNotLoseConcurrentNASTags(t *testing.T) {
	nas, box := openWithBox(t)
	old := importOneFile(t, nas, box, "file.txt", "old")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	prepared, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "file.txt", "replacement")}, ClientImportOptions{Box: box, ConflictMode: "overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := prepared.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := nas.CreateTag(ctx, "NAS annotation", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nas.TagFiles(ctx, tag.ID, []domain.FileID{old.ID}); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil && !errors.Is(err, ErrSyncConflict) {
		t.Fatal(err)
	}
	status, statusErr := c.Status(ctx)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if result.Complete || status.BlockedID != imported.Reference.LocalID {
		t.Fatalf("lost concurrent annotation=%+v", result)
	}
	file, found, err := nas.File(ctx, old.ID)
	if err != nil || !found || file.BlobKey != old.BlobKey {
		t.Fatalf("NAS overwritten=%+v %v %v", file, found, err)
	}
	tags, err := nas.FileTags(ctx, old.ID)
	if err != nil || len(tags) != 1 || tags[0] != tag.ID {
		t.Fatalf("tags lost=%+v %v", tags, err)
	}
}
