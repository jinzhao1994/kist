package library

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func stageAdoptionOriginal(t *testing.T, c *Client) (blob.Key, domain.FileID) {
	t.Helper()
	create := syncBox(t, "adopt-box")
	if err := c.Stage(t.Context(), create); err != nil {
		t.Fatal(err)
	}
	box, _ := oplog.Payload[oplog.CreateBoxPayload](create.Payload)
	content, err := c.PutOriginal(t.Context(), strings.NewReader("offline photo"))
	if err != nil {
		t.Fatal(err)
	}
	file := oplog.ImportedFile{FileID: domain.NewFileID(), FolderID: box.RootFolderID, Name: "photo.txt", BlobKey: string(content.Key), Sha256: content.Key.Hex(), Size: content.Size}
	raw, _ := oplog.Marshal(oplog.ImportFilesPayload{BoxID: box.BoxID, Files: []oplog.ImportedFile{file}})
	if err := c.Stage(t.Context(), SyncProposal{Version: 1, ID: "adopt-file", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionImportFiles), Origin: "user", Payload: raw}); err != nil {
		t.Fatal(err)
	}
	return content.Key, file.FileID
}

func TestClientAdoptionPreservesPendingAndLiveReaders(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	key, _ := stageAdoptionOriginal(t, c)
	if err := c.AdoptConfirmedOriginals(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ := c.blobs.LocalPath(key)
	if _, err := os.Stat(source); err != nil {
		t.Fatal("pending original lost", err)
	}
	reader, err := c.OpenOriginal(ctx, string(key))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("active original moved", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "offline photo" {
		t.Fatal(string(data), err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	preparation, err := c.localPreparationLease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AdoptConfirmedOriginals(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("preparing original moved", err)
	}
	preparation()
	if err := c.AdoptConfirmedOriginals(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("confirmed bytes remain protected", err)
	}
	cached, err := c.OpenOriginal(ctx, string(key))
	if err != nil {
		t.Fatal(err)
	}
	data, err = io.ReadAll(cached)
	cached.Close()
	if err != nil || string(data) != "offline photo" {
		t.Fatal(string(data), err)
	}
	if err := c.AdoptConfirmedOriginals(ctx); err != nil {
		t.Fatal("repeat", err)
	}
	protected, err := c.ProtectedStorage(ctx)
	if err != nil || protected.OriginalsBytes != 0 {
		t.Fatal(protected, err)
	}
}

func TestClientAdoptionQuotaAndOfflinePins(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		t.Run(map[bool]string{false: "evict", true: "retain"}[pinned], func(t *testing.T) {
			nas := openTestLibrary(t)
			c := newSyncClient(t, nas)
			ctx := t.Context()
			key, _ := stageAdoptionOriginal(t, c)
			if pinned {
				if err := c.SetOfflineOriginals(ctx, "test", []string{string(key)}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := c.SetCacheLimits(ctx, time.Hour, 1); err != nil {
				t.Fatal(err)
			}
			if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
				t.Fatal(err)
			}
			reader, err := c.OpenOriginal(ctx, string(key))
			if pinned {
				if err != nil {
					t.Fatal(err)
				}
				reader.Close()
			} else if !errors.Is(err, ErrNotCached) {
				t.Fatal(err)
			}
			status, err := c.CacheStatus(ctx)
			if err != nil || status.OverLimit != pinned {
				t.Fatal(status, err)
			}
			if _, err := nas.blobs.Stat(ctx, key); err != nil {
				t.Fatal("NAS authority missing", err)
			}
		})
	}
}

func TestClientAdoptionRequiresConfirmedCatalog(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	key, _ := stageAdoptionOriginal(t, c)
	entries, err := c.queue.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	peer := &testSyncPeer{nas: nas}
	for _, entry := range entries {
		if err := c.uploadIntentOriginals(ctx, peer, entry.Proposal); err != nil {
			t.Fatal(err)
		}
		if _, err := nas.AcceptSync(ctx, entry.Proposal); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate losing every response: the NAS is durable, but local knowledge
	// has not crossed the confirmed catalog boundary.
	if err := c.AdoptConfirmedOriginals(ctx); err != nil {
		t.Fatal(err)
	}
	path, _ := c.blobs.LocalPath(key)
	if _, err := os.Stat(path); err != nil {
		t.Fatal("unconfirmed original moved", err)
	}
	if _, err := c.Synchronize(ctx, peer); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("confirmed original not migrated", err)
	}
}

func TestClientAdoptionRecoversBeforeReadyMarker(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	key, _ := stageAdoptionOriginal(t, c)
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	record, err := c.cacheRecord(string(key))
	if err != nil {
		t.Fatal(err)
	}
	// This is the durable state after rename but before writing Complete=true.
	record.Complete = false
	if err := c.saveCacheRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	if _, err := c.OpenOriginal(ctx, string(key)); !errors.Is(err, ErrNotCached) {
		t.Fatal("premature readiness", err)
	}
	result, err := c.RebuildCache(ctx)
	if err != nil || result.Verified != 1 {
		t.Fatal(result, err)
	}
	reader, err := c.OpenOriginal(ctx, string(key))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "offline photo" {
		t.Fatal(string(data), err)
	}
}
