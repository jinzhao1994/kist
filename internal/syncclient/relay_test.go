package syncclient_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/syncclient"
)

func TestHTTPRelayCustodyAuthTopologyAndHandover(t *testing.T) {
	ctx := t.Context()
	root := filepath.Join(t.TempDir(), "root")
	if _, err := library.Create(ctx, root); err != nil {
		t.Fatal(err)
	}
	nas, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nas.Close()
	grant, err := nas.AuthorizeSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ns := httptest.NewUnstartedServer(nil)
	handler, err := api.New(nas, api.Options{AllowedHosts: []string{ns.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	var rootOffline atomic.Bool
	ns.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rootOffline.Load() {
			http.Error(w, "offline", 503)
			return
		}
		handler.ServeHTTP(w, r)
	})
	ns.Start()
	defer ns.Close()
	np, err := syncclient.New(ns.URL, grant.Token, syncclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	head, err := np.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	laptop, err := library.CreateClient(ctx, t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer laptop.Close()
	if err = laptop.VerifyAndSetConnection(ctx, np, library.ClientConnection{Server: ns.URL}, grant.Token, ""); err != nil {
		t.Fatal(err)
	}
	rootOffline.Store(false)
	if _, err = laptop.Synchronize(ctx, np); err != nil {
		t.Fatal(err)
	}
	lg, err := laptop.AuthorizeSyncWithNote(ctx, "phone")
	if err != nil {
		t.Fatal(err)
	}
	ls := httptest.NewUnstartedServer(nil)
	lh, err := api.NewClient(laptop, api.Options{AllowedHosts: []string{ls.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	var dropReply atomic.Bool
	ls.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/relay/submit") && dropReply.CompareAndSwap(true, false) {
			rr := httptest.NewRecorder()
			lh.ServeHTTP(rr, r)
			if rr.Code != 200 {
				t.Errorf("submit before dropped reply: %d", rr.Code)
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
			return
		}
		lh.ServeHTTP(w, r)
	})
	ls.Start()
	defer ls.Close()
	lp, err := syncclient.New(ls.URL, lg.Token, syncclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	caps, err := lp.Capabilities(ctx)
	if err != nil || !caps.Relay || len(caps.Ancestors) != 1 {
		t.Fatalf("relay capabilities: %+v %v", caps, err)
	}
	wrong, _ := syncclient.New(ls.URL, grant.Token, syncclient.Options{})
	if _, err = wrong.Capabilities(ctx); err == nil {
		t.Fatal("root credential accepted at relay")
	}
	if err = laptop.VerifyAndSetConnection(ctx, lp, library.ClientConnection{Server: ls.URL}, lg.Token, ""); !errors.Is(err, library.ErrSyncConflict) {
		t.Fatalf("self connection: %v", err)
	}
	phone, err := library.CreateClient(ctx, t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer phone.Close()
	if err = phone.VerifyAndSetConnection(ctx, lp, library.ClientConnection{Server: ls.URL}, lg.Token, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = phone.Synchronize(ctx, lp); err != nil {
		t.Fatal(err)
	}
	rootOffline.Store(true)
	box, folder := domain.NewBoxID(), domain.NewFolderID()
	raw, _ := oplog.Marshal(oplog.CreateBoxPayload{BoxID: box, RootFolderID: folder, BoxFields: oplog.BoxFields{Name: "trip"}})
	proposal := library.SyncProposal{Version: 1, ID: "phone-trip", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionCreateBox), Origin: "user", Payload: raw}
	if err = phone.Stage(ctx, proposal); err != nil {
		t.Fatal(err)
	}
	result, err := phone.SynchronizeAutomatic(ctx, lp)
	if err != nil || result.ManualRequired {
		t.Fatalf("automatic relay: %+v %v", result, err)
	}
	status, err := phone.Status(ctx)
	if err != nil || len(status.Pending) != 1 || !status.Pending[0].AwaitingRoot {
		t.Fatalf("custody status: %+v %v", status, err)
	}
	rootOffline.Store(false)
	if err = phone.VerifyAndSetConnection(ctx, np, library.ClientConnection{Server: ns.URL}, grant.Token, ""); !errors.Is(err, library.ErrSyncConflict) {
		t.Fatalf("unsafe handover: %v", err)
	}
	rootOffline.Store(true)
	content := bytes.Repeat([]byte("relay bytes"), 1000)
	stored, err := phone.PutOriginal(ctx, bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = oplog.Marshal(oplog.ImportFilesPayload{BoxID: box, Files: []oplog.ImportedFile{{FileID: domain.NewFileID(), FolderID: folder, Name: "trip.bin", BlobKey: string(stored.Key), Sha256: stored.Key.Hex(), Size: stored.Size}}})
	p := library.SyncProposal{Version: 1, ID: "phone-file", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionImportFiles), Origin: "user", Payload: raw, Requires: []string{proposal.ID}}
	if err = phone.Stage(ctx, p); err != nil {
		t.Fatal(err)
	}
	dropReply.Store(true)
	if _, err = phone.Synchronize(ctx, lp); err == nil {
		t.Fatal("lost custody reply succeeded")
	}
	if err = phone.Resolve(ctx, p.ID, nil, "discard"); !errors.Is(err, library.ErrSyncConflict) {
		t.Fatalf("discarded uncertain custody: %v", err)
	}
	if _, err = phone.Synchronize(ctx, lp); err != nil {
		t.Fatal(err)
	}
	reader, err := lp.OpenOriginal(ctx, string(stored.Key), stored.Size, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("relay bytes: %v", err)
	}
	rootOffline.Store(false)
	if _, err = laptop.Synchronize(ctx, np); err != nil {
		t.Fatal(err)
	}
	if _, err = phone.Synchronize(ctx, lp); err != nil {
		t.Fatal(err)
	}
	status, err = phone.Status(ctx)
	if err != nil || len(status.Pending) != 0 {
		t.Fatalf("final confirmation: %+v %v", status, err)
	}
	if err = phone.VerifyAndSetConnection(ctx, np, library.ClientConnection{Server: ns.URL}, grant.Token, ""); err != nil {
		t.Fatalf("finalized handover: %v", err)
	}
	if _, err = phone.Synchronize(ctx, np); err != nil {
		t.Fatal(err)
	}

	// An online descendant cannot become its ancestor's parent.
	if err = phone.VerifyAndSetConnection(ctx, lp, library.ClientConnection{Server: ls.URL}, lg.Token, ""); err != nil {
		t.Fatal(err)
	}
	pg, err := phone.AuthorizeSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ps := httptest.NewUnstartedServer(nil)
	ph, err := api.NewClient(phone, api.Options{AllowedHosts: []string{ps.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	ps.Config.Handler = ph
	ps.Start()
	defer ps.Close()
	pp, _ := syncclient.New(ps.URL, pg.Token, syncclient.Options{})
	if err = laptop.VerifyAndSetConnection(ctx, pp, library.ClientConnection{Server: ps.URL}, pg.Token, ""); !errors.Is(err, library.ErrSyncConflict) {
		t.Fatalf("descendant connection: %v", err)
	}
	stale, _ := lp.RelayHead(ctx)
	stale.Sequence++
	proposal.ID = "rejected-cas"
	_, err = lp.ReceiveRelayAt(ctx, head.LibraryID, proposal, stale)
	var rejected *library.SyncRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("CAS rejection not definitive: %v", err)
	}
	request, _ := http.NewRequestWithContext(ctx, "GET", ls.URL+"/api/v1/sync/capabilities", nil)
	request.Header.Set("Authorization", "Bearer "+lg.Token)
	request.Header.Set("X-Kist-Route", caps.NodeID.String())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 409 {
		t.Fatalf("loop trace: %d", response.StatusCode)
	}
	if _, err = laptop.RevokeSync(ctx, lg.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = lp.RelayHead(ctx); err == nil {
		t.Fatal("revoked credential accepted")
	}
}
