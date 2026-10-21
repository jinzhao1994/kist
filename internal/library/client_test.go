package library

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestClientPartialCatalogResumeAndLocalPersistence(t *testing.T) {
	nas, box := openWithBox(t)
	if _, err := nas.CreateBox(t.Context(), "second", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "client")
	c, err := CreateClient(t.Context(), root, ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { c.Close() }()
	if _, err := Open(root); !errors.Is(err, ErrNotALibrary) {
		t.Fatalf("client opened as authoritative NAS: %v", err)
	}
	first, err := nas.SyncCatalog(t.Context(), head, 0, 1, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ReceiveCatalogPage(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.Box(t.Context(), box); err != nil || found {
		t.Fatalf("partial catalog exposed as complete view: %v %v", found, err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c, err = OpenClient(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	status, err := c.CatalogStatus(t.Context())
	if err != nil || status.Complete || status.Downloaded != 1 || status.Transfer == nil || *status.Transfer != head {
		t.Fatalf("resume status=%+v err=%v", status, err)
	}
	if err := c.ReceiveCatalogPage(t.Context(), first); err != nil {
		t.Fatalf("retry first page: %v", err)
	}
	last, err := nas.SyncCatalog(t.Context(), head, status.Downloaded, 1, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ReceiveCatalogPage(t.Context(), last); err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.Box(t.Context(), box); err != nil || !found {
		t.Fatalf("completed catalog unavailable: %v %v", found, err)
	}
	local := syncBox(t, "client-local-box")
	if err := c.Stage(t.Context(), local); err != nil {
		t.Fatal(err)
	}
	if err := c.Stage(t.Context(), local); err != nil {
		t.Fatalf("stage retry: %v", err)
	}
	created, _ := oplog.Payload[oplog.CreateBoxPayload](local.Payload)
	if _, found, err := c.Box(t.Context(), created.BoxID); err != nil || !found {
		t.Fatalf("local edit unavailable: %v %v", found, err)
	}
	c.Close()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(filepath.Join(root, "client.db") + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	c, err = OpenClient(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.Box(t.Context(), created.BoxID); err != nil || !found {
		t.Fatalf("lost local edit after deleting derived database: %v %v", found, err)
	}
	if _, found, err := c.Box(t.Context(), box); err != nil || !found {
		t.Fatalf("lost confirmed catalog after deleting derived database: %v %v", found, err)
	}
}

func TestClientNewOriginalMustBeDurableBeforePendingIntent(t *testing.T) {
	nas, box := openWithBox(t)
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	c, err := CreateClient(t.Context(), t.TempDir(), ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	page, err := nas.SyncCatalog(t.Context(), head, 0, 256, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ReceiveCatalogPage(t.Context(), page); err != nil {
		t.Fatal(err)
	}
	b, _, _ := c.Box(t.Context(), box)
	content := "new travel photo"
	hash := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(hash[:])
	f := oplog.ImportedFile{FileID: domain.NewFileID(), FolderID: b.RootFolderID, Name: "travel.jpg", BlobKey: "sha256:" + digest, Sha256: digest, Size: int64(len(content))}
	raw, _ := oplog.Marshal(oplog.ImportFilesPayload{BoxID: box, Files: []oplog.ImportedFile{f}})
	p := SyncProposal{Version: 1, ID: "client-local-photo", OccurredAt: time.Now().UTC(), Origin: "user", Action: string(oplog.ActionImportFiles), Payload: raw}
	if err := c.Stage(t.Context(), p); err == nil {
		t.Fatal("staged new content without durable original")
	}
	entries, err := c.queue.snapshot(t.Context())
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed import persisted intent: %+v %v", entries, err)
	}
	if _, err := c.PutOriginal(t.Context(), strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if err := c.Stage(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	undo := SyncProposal{Version: 1, ID: "client-local-photo-undo", OccurredAt: time.Now().UTC(), Origin: "user", Undo: &SyncReference{LocalID: p.ID}}
	if err := c.Stage(t.Context(), undo); err != nil {
		t.Fatal(err)
	}
	if _, found, err := (&Library{db: c.db}).File(t.Context(), f.FileID); err != nil || found {
		t.Fatalf("client undo failed: %v %v", found, err)
	}
}

func TestClientCanReferenceUncachedNASBlob(t *testing.T) {
	nas, box := openWithBox(t)
	content := "already durable on NAS"
	remote := importOneFile(t, nas, box, "remote.txt", content)
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	c, err := CreateClient(t.Context(), t.TempDir(), ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	page, err := nas.SyncCatalog(t.Context(), head, 0, 256, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ReceiveCatalogPage(t.Context(), page); err != nil {
		t.Fatal(err)
	}
	f := oplog.ImportedFile{FileID: domain.NewFileID(), FolderID: remote.FolderID, Name: "copy.txt", BlobKey: remote.BlobKey, Sha256: strings.TrimPrefix(remote.BlobKey, "sha256:"), Size: int64(len(content)) + 1}
	p := SyncProposal{Version: 1, ID: "copy-uncached-nas-content", OccurredAt: time.Now().UTC(), Origin: "user", Action: string(oplog.ActionImportFiles)}
	p.Payload, _ = oplog.Marshal(oplog.ImportFilesPayload{BoxID: box, Files: []oplog.ImportedFile{f}})
	if err := c.Stage(t.Context(), p); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("changed known NAS content facts: %v", err)
	}
	f.Size--
	p.Payload, _ = oplog.Marshal(oplog.ImportFilesPayload{BoxID: box, Files: []oplog.ImportedFile{f}})
	if err := c.Stage(t.Context(), p); err != nil {
		t.Fatalf("uncached NAS original treated as missing: %v", err)
	}
	if _, found, err := (&Library{db: c.db}).File(t.Context(), f.FileID); err != nil || !found {
		t.Fatalf("metadata-only copy unavailable: %v %v", found, err)
	}
}
