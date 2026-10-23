package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/syncclient"
)

func TestAutomaticWorkerPullsAndWakesAfterLocalWrite(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	head, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client, err := library.CreateClient(ctx, t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	grant, err := nas.AuthorizeSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewUnstartedServer(nil)
	handler, err := New(nas, Options{AllowedHosts: []string{remote.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	remote.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/submit-conditional") {
			entered <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		handler.ServeHTTP(w, r)
	})
	remote.Start()
	defer remote.Close()
	if err := client.SetConnection(ctx, head.LibraryID, library.ClientConnection{Server: remote.URL}, grant.Token); err != nil {
		t.Fatal(err)
	}
	peer, err := syncclient.New(remote.URL, grant.Token, syncclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Synchronize(ctx, peer); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.CreateBox(ctx, "remote", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	server, err := NewClient(client, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { server.RunAutoSync(runCtx); close(done) }()
	defer func() { cancel(); <-done }()
	wait := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("automatic synchronization timed out")
	}
	wait(func() bool {
		catalog, _ := client.CatalogStatus(ctx)
		current, _ := nas.SyncHead(ctx)
		return catalog.Confirmed == current
	})
	w := call(server, "POST", Prefix+"/boxes", "automatic-create-box", `{"name":"local"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("local mutation did not wake worker")
	}
	status := decodeBody[clientConnectionStatus](t, server, "/client/connection-status")
	if !status.Syncing || status.PendingUpload != 1 {
		t.Fatal(status)
	}
	once.Do(func() { close(release) })
	wait(func() bool {
		status, _ := client.Status(ctx)
		current, _ := nas.SyncHead(ctx)
		return len(status.Pending) == 0 && status.Catalog.Confirmed == current
	})
}
