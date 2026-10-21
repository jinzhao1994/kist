package syncclient_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/syncclient"
)

func TestHTTPClientResumesDurableBlocksAndLostCommit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nas")
	if _, err := library.Create(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, "kist.toml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Library.Timezone = "UTC"
	if err := config.Save(filepath.Join(root, "kist.toml"), cfg); err != nil {
		t.Fatal(err)
	}
	nas, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nas.Close()
	credential, err := nas.AuthorizeSync(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	apiServer, err := api.New(nas, api.Options{AllowedHosts: []string{server.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	var dropSubmit, dropBlock, dropDownload atomic.Bool
	var mu sync.Mutex
	var offsets []string
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		block := r.Method == "PUT" && strings.Contains(r.URL.Path, "/original-uploads/")
		if block {
			mu.Lock()
			offsets = append(offsets, r.URL.Query().Get("offset"))
			mu.Unlock()
		}
		partialDownload := strings.HasSuffix(r.URL.Path, "/content") && dropDownload.CompareAndSwap(true, false)
		drop := partialDownload || block && dropBlock.CompareAndSwap(true, false) || strings.HasSuffix(r.URL.Path, "/submit") && dropSubmit.CompareAndSwap(true, false)
		if !drop {
			apiServer.ServeHTTP(w, r)
			return
		}
		// Finish the real write, then lose the TCP response entirely.
		recorder := httptest.NewRecorder()
		apiServer.ServeHTTP(recorder, r)
		if recorder.Code != 200 {
			t.Errorf("fault injection intercepted a failure: %d %s", recorder.Code, recorder.Body)
		}
		if partialDownload {
			for key, values := range recorder.Header() {
				w.Header()[key] = values
			}
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes()[:7])
			w.(http.Flusher).Flush()
		}
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = connection.Close()
	})
	server.Start()
	defer server.Close()
	peer, err := syncclient.New(server.URL, credential.Token, syncclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	head, err := peer.Head(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	clientRoot := t.TempDir()
	client, err := library.CreateClient(t.Context(), clientRoot, library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { client.Close() }()
	if _, err := client.Synchronize(t.Context(), peer); err != nil {
		t.Fatal(err)
	}
	boxID, folderID := domain.NewBoxID(), domain.NewFolderID()
	raw, _ := oplog.Marshal(oplog.CreateBoxPayload{BoxID: boxID, RootFolderID: folderID, BoxFields: oplog.BoxFields{Name: "travel"}})
	proposal := library.SyncProposal{Version: 1, ID: "http-travel-box", OccurredAt: time.Now().UTC().Add(-24 * time.Hour), Action: string(oplog.ActionCreateBox), Origin: "user", Payload: raw}
	if err := client.Stage(t.Context(), proposal); err != nil {
		t.Fatal(err)
	}
	independent, err := nas.CreateBox(t.Context(), "independent NAS edit", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	dropSubmit.Store(true)
	if _, err := client.Synchronize(t.Context(), peer); err == nil {
		t.Fatal("lost reply unexpectedly succeeded")
	}
	if err := client.Resolve(t.Context(), proposal.ID, nil, "discard"); !errors.Is(err, library.ErrSyncConflict) {
		t.Fatalf("discarded uncertain submission: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	client, err = library.OpenClient(t.Context(), clientRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := client.Synchronize(t.Context(), peer); err != nil || !result.Complete {
		t.Fatalf("recover commit: %+v %v", result, err)
	}
	if _, found, err := client.Box(t.Context(), independent.ID); err != nil || !found {
		t.Fatalf("lost independent NAS edit: %v", err)
	}
	content := bytes.Repeat([]byte("trip"), (9<<20)/4)
	stored, err := client.PutOriginal(t.Context(), bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = oplog.Marshal(oplog.ImportFilesPayload{BoxID: boxID, Files: []oplog.ImportedFile{{FileID: domain.NewFileID(), FolderID: folderID, Name: "trip.bin", BlobKey: string(stored.Key), Sha256: stored.Key.Hex(), Size: stored.Size}}})
	imported := library.SyncProposal{Version: 1, ID: "http-travel-original", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionImportFiles), Origin: "user", Payload: raw, Requires: []string{proposal.ID}}
	if err := client.Stage(t.Context(), imported); err != nil {
		t.Fatal(err)
	}
	dropBlock.Store(true)
	if _, err := client.Synchronize(t.Context(), peer); err == nil {
		t.Fatal("lost block reply unexpectedly succeeded")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	client, err = library.OpenClient(t.Context(), clientRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := client.Synchronize(t.Context(), peer); err != nil || !result.Complete {
		t.Fatalf("resume upload: %+v %v", result, err)
	}
	mu.Lock()
	gotOffsets := strings.Join(offsets, ",")
	mu.Unlock()
	if gotOffsets != "0,8388608" {
		t.Fatalf("retransmitted acknowledged content: %s", gotOffsets)
	}
	reader, err := nas.OpenSyncOriginal(t.Context(), string(stored.Key))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(content, actual) {
		t.Fatalf("uploaded content mismatch: %v", err)
	}
	download, err := peer.OpenOriginal(t.Context(), string(stored.Key), stored.Size, 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	tail, err := io.ReadAll(download)
	download.Close()
	if err != nil || !bytes.Equal(tail, content[8<<20:]) {
		t.Fatalf("resumed download mismatch: %v", err)
	}
	dropDownload.Store(true)
	if err := client.CacheOriginal(t.Context(), string(stored.Key), peer); err == nil {
		t.Fatal("interrupted cache download unexpectedly completed")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	client, err = library.OpenClient(t.Context(), clientRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CacheOriginal(t.Context(), string(stored.Key), peer); err != nil {
		t.Fatalf("cache resume: %v", err)
	}
	cached, err := client.OpenCachedOriginal(t.Context(), string(stored.Key))
	if err != nil {
		t.Fatal(err)
	}
	cachedBytes, err := io.ReadAll(cached)
	cached.Close()
	if err != nil || !bytes.Equal(cachedBytes, content) {
		t.Fatalf("resumed cache differs from NAS: %v", err)
	}
	if result, err := client.Synchronize(t.Context(), peer); err != nil || !result.Complete || result.Accepted != 0 {
		t.Fatalf("repeat sync: %+v %v", result, err)
	}
	if count, _ := nas.OpCount(t.Context()); count != 3 {
		t.Fatalf("duplicate or missing archive operations: %d", count)
	}
	if _, err := nas.RevokeSync(t.Context(), credential.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Capabilities(t.Context()); err == nil {
		t.Fatal("revoked credential remained usable")
	}
}

func TestHTTPSubmissionRequiresExplicitRejection(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
		rejected   bool
	}{
		{"explicit", `{"rejected":true,"error":{"code":"sync_conflict","class":"conflict","message":"stale"}}`, 200, true},
		{"uncertain", `{"rejected":false,"error":{"code":"internal","class":"internal","message":"unknown"}}`, 200, false},
		{"proxy", `{"rejected":true}`, 503, false},
		{"truncated", `{"rejected":true,"error":`, 200, false},
		{"inconsistent", `{"rejected":true,"result":{"op_id":1}}`, 200, false},
		{"empty-error", `{"rejected":true,"error":{}}`, 200, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			peer, err := syncclient.New(server.URL, "credential", syncclient.Options{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = peer.Submit(t.Context(), library.SyncProposal{})
			var rejected *library.SyncRejectedError
			if err == nil || errors.As(err, &rejected) != test.rejected {
				t.Fatalf("incorrect commit certainty: %v", err)
			}
		})
	}
}

func TestHTTPClientRejectsRedirectAndKeepsFailuresUncertain(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(true) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	peer, err := syncclient.New(redirect.URL, "secret", syncclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = peer.Submit(t.Context(), library.SyncProposal{})
	var rejected *library.SyncRejectedError
	if err == nil || errors.As(err, &rejected) || leaked.Load() {
		t.Fatalf("redirect leaked or was treated as rejection: %v", err)
	}
	if _, err := syncclient.New("http://nas.example", "secret", syncclient.Options{}); err == nil {
		t.Fatal("silently enabled cleartext NAS authorization")
	}
	if _, err := syncclient.New("http://nas.example", "secret", syncclient.Options{AllowHTTP: true}); err != nil {
		t.Fatal(err)
	}
}
