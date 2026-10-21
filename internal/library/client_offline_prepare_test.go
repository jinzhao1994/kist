package library

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestClientPrepareOfflineResumesAndVerifiesReadiness(t *testing.T) {
	nas, box := openWithBox(t)
	content := "offline original content"
	f := importOneFile(t, nas, box, "offline.txt", content)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	if err := c.SetOfflineSelection(ctx, "trip", ClientOfflineSelection{Files: []domain.FileID{f.ID}}); err != nil {
		t.Fatal(err)
	}
	status, err := c.OfflineStatus(ctx, "trip")
	if err != nil || status.Ready || len(status.Missing) != 1 {
		t.Fatalf("initial status=%+v %v", status, err)
	}
	source := &cacheTestSource{nas: nas, content: content, cut: 7}
	status, err = c.PrepareOffline(ctx, "trip", source)
	if !errors.Is(err, io.ErrUnexpectedEOF) || status.Ready {
		t.Fatalf("partial status=%+v %v", status, err)
	}
	status, err = c.OfflineStatus(ctx, "trip")
	if err != nil || status.Ready {
		t.Fatalf("partial became ready=%+v %v", status, err)
	}
	status, err = c.PrepareOffline(ctx, "trip", source)
	if err != nil || !status.Ready || status.Available != 1 {
		t.Fatalf("completed status=%+v %v", status, err)
	}
	if len(source.offsets) != 2 || source.offsets[1] != 7 {
		t.Fatalf("resume=%v", source.offsets)
	}
	// All required bytes are verified locally; a subsequent prepare needs no NAS.
	status, err = c.PrepareOffline(ctx, "trip", nil)
	if err != nil || !status.Ready {
		t.Fatalf("offline recheck=%+v %v", status, err)
	}
}

func TestClientPrepareOfflineUsesUnsyncedOriginal(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	create := syncBox(t, "offline-local-box")
	if err := c.Stage(ctx, create); err != nil {
		t.Fatal(err)
	}
	box, err := oplog.Payload[oplog.CreateBoxPayload](create.Payload)
	if err != nil {
		t.Fatal(err)
	}
	content := "only exists on the laptop"
	stored, err := c.PutOriginal(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	f := oplog.ImportedFile{FileID: domain.NewFileID(), FolderID: box.RootFolderID, Name: "local.txt", BlobKey: string(stored.Key), Sha256: stored.Key.Hex(), Size: stored.Size}
	raw, err := oplog.Marshal(oplog.ImportFilesPayload{BoxID: box.BoxID, Files: []oplog.ImportedFile{f}})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Stage(ctx, SyncProposal{Version: 1, ID: "offline-local-file", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionImportFiles), Origin: "user", Payload: raw, Requires: []string{create.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetOfflineSelection(ctx, "trip", ClientOfflineSelection{Files: []domain.FileID{f.FileID}}); err != nil {
		t.Fatal(err)
	}
	status, err := c.PrepareOffline(ctx, "trip", nil)
	if err != nil || !status.Ready {
		t.Fatalf("local pending content=%+v %v", status, err)
	}
	usage, err := c.CacheStatus(ctx)
	if err != nil || usage.CachedBytes != 0 {
		t.Fatalf("duplicated protected bytes in cache=%+v %v", usage, err)
	}
	reader, err := c.OpenOriginal(ctx, f.BlobKey)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil || string(got) != content {
		t.Fatalf("local bytes=%q %v", got, err)
	}
}

type selectionChangingSource struct {
	*cacheTestSource
	change func()
}

func (s *selectionChangingSource) OpenOriginal(ctx context.Context, key string, size, offset int64) (io.ReadCloser, error) {
	if s.change != nil {
		change := s.change
		s.change = nil
		change()
	}
	return s.cacheTestSource.OpenOriginal(ctx, key, size, offset)
}

func TestClientPrepareOfflineDoesNotClaimChangedSelectionReady(t *testing.T) {
	nas, box := openWithBox(t)
	first := importOneFile(t, nas, box, "first.txt", "first content")
	second := importOneFile(t, nas, box, "second.txt", "second content")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	if err := c.SetOfflineSelection(ctx, "trip", ClientOfflineSelection{Files: []domain.FileID{first.ID}}); err != nil {
		t.Fatal(err)
	}
	source := &selectionChangingSource{cacheTestSource: &cacheTestSource{nas: nas, content: "first content"}, change: func() {
		if err := c.SetOfflineSelection(ctx, "trip", ClientOfflineSelection{Files: []domain.FileID{second.ID}}); err != nil {
			t.Fatal(err)
		}
	}}
	status, err := c.PrepareOffline(ctx, "trip", source)
	if err != nil || status.Ready || len(status.Missing) != 1 || status.Missing[0] != second.BlobKey {
		t.Fatalf("obsolete readiness=%+v %v", status, err)
	}
	selections, err := c.OfflineSelections(ctx)
	if err != nil || selections["trip"].Files[0] != second.ID {
		t.Fatalf("overwrote newer selection=%+v %v", selections, err)
	}
}
