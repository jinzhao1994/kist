package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestClientExtractionPersistsFactsSyncsAndUndoes(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	imported, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "text.jpg", "plain text")}, ClientImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := imported.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	file := files[0]
	prepared, err := c.PrepareExtraction(ctx, file.ID, "media")
	if err != nil {
		t.Fatal(err)
	}
	request := Request{ID: "client-extract-media", Digest: strings.Repeat("b", 64)}
	receipt, err := c.ExecuteRequest(ctx, request, prepared.Commit)
	if err != nil || receipt.LocalID == "" {
		t.Fatal(receipt, err)
	}
	if _, err := c.ExecuteRequest(ctx, request, func(context.Context) error { t.Fatal("repeated extraction"); return nil }); err != nil {
		t.Fatal(err)
	}
	info, err := c.MediaInfo(ctx, file.ID)
	if err != nil || info.Kind != "document" || !strings.HasPrefix(info.MIME, "text/plain") {
		t.Fatal(info, err)
	}
	noop, err := c.PrepareExtraction(ctx, file.ID, "media")
	if err != nil {
		t.Fatal(err)
	}
	nr, err := c.ExecuteRequest(ctx, Request{ID: "client-extract-noop", Digest: strings.Repeat("c", 64)}, noop.Commit)
	if err != nil || !nr.Noop {
		t.Fatal(nr, err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	accepted, found, err := nas.SyncStatus(ctx, receipt.LocalID)
	if err != nil || !found {
		t.Fatal(accepted, err)
	}
	row, found, err := store.GetOp(nas.db, accepted.OpID)
	if err != nil || !found || row.Origin != string(oplog.OriginExtractor) {
		t.Fatal(row, err)
	}
	if err := nas.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	info, err = nas.MediaInfo(ctx, file.ID)
	if err != nil || info.Kind != "document" {
		t.Fatal(info, err)
	}
	if _, err := c.Undo(ctx, SyncReference{LocalID: receipt.LocalID}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	info, err = nas.MediaInfo(ctx, file.ID)
	if err != nil || info.Kind != "" {
		t.Fatal(info, err)
	}
}

func TestClientExtractionRequiresCachedOriginal(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "photo.jpg", "original")
	c := newSyncClient(t, nas)
	if _, err := c.PrepareExtraction(t.Context(), file.ID, "media"); !errors.Is(err, ErrNotCached) {
		t.Fatal(err)
	}
}

func TestClientPreparedExtractionRejectsReplacedContent(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	first, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "file.txt", "first")}, ClientImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	prepared, err := c.PrepareExtraction(ctx, files[0].ID, "media")
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "file.txt", "replacement")}, ClientImportOptions{Box: box, ConflictMode: "overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replacement.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Commit(ctx); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("stale content: %v", err)
	}
	info, err := c.MediaInfo(ctx, files[0].ID)
	if err != nil || info.Kind != "" {
		t.Fatal(info, err)
	}
}

func TestClientExtractionPreservesSuppressionAndSkippedCaptureTime(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	imported, err := c.PrepareFiles(ctx, []ImportSource{clientImportSource(t, "photo.jpg", "text, not a photograph")}, ClientImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := imported.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	file := files[0]
	skipped, err := c.PrepareExtraction(ctx, file.ID, "exif")
	if err != nil {
		t.Fatal(err)
	}
	if err := skipped.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	after, found, err := c.File(ctx, file.ID)
	if err != nil || !found || after.CapturedAt != file.CapturedAt {
		t.Fatal(after, err)
	}
	firstMedia, err := c.PrepareExtraction(ctx, file.ID, "media")
	if err != nil {
		t.Fatal(err)
	}
	if err := firstMedia.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RemoveMetadata(ctx, file.ID, "media:kind"); err != nil {
		t.Fatal(err)
	}
	media, err := c.PrepareExtraction(ctx, file.ID, "media")
	if err != nil {
		t.Fatal(err)
	}
	if err := media.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	row := metaOf(t, nas, file.ID)["media:kind"]
	if !row.Tombstone {
		t.Fatal("extraction restored suppressed fact", row)
	}
}
