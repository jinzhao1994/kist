package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func syncCall(t *testing.T, s *Server, method, suffix, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, "http://localhost:8473"+Prefix+"/sync/"+suffix, strings.NewReader(string(raw)))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestSyncHTTPAuthorizationAndSubmission(t *testing.T) {
	s, l := testServer(t)
	for _, suffix := range []string{"capabilities", "head", "catalog", "submit"} {
		w := syncCall(t, s, "POST", suffix, "", struct{}{})
		if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "unauthorized") {
			t.Fatalf("unprotected %s: %d %s", suffix, w.Code, w.Body)
		}
	}
	credential, err := l.AuthorizeSync(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	w := syncCall(t, s, "GET", "capabilities", credential.Token, nil)
	var caps library.SyncCapabilities
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &caps) != nil || caps.Version != 1 || !caps.UndoReferences {
		t.Fatalf("capabilities=%d %s", w.Code, w.Body)
	}
	raw, _ := oplog.Marshal(oplog.CreateBoxPayload{BoxID: domain.NewBoxID(), RootFolderID: domain.NewFolderID(), BoxFields: oplog.BoxFields{Name: "offline HTTP box"}})
	p := library.SyncProposal{Version: 1, ID: "http-offline-create", OccurredAt: time.Date(2026, 10, 20, 1, 2, 3, 123456789, time.UTC), Action: string(oplog.ActionCreateBox), Origin: "user", Payload: raw}
	var receipt library.SyncResult
	for range 2 {
		w = syncCall(t, s, "POST", "submit", credential.Token, p)
		var response SyncSubmitResponse
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Error != nil || response.Rejected || response.Result == nil {
			t.Fatalf("submit=%d %s", w.Code, w.Body)
		}
		if receipt.OpID != 0 && receipt != *response.Result {
			t.Fatal("HTTP retry changed receipt")
		}
		receipt = *response.Result
	}
	if count, _ := l.OpCount(t.Context()); count != 1 {
		t.Fatalf("HTTP retry wrote %d events", count)
	}
	w = syncCall(t, s, "GET", "head", credential.Token, nil)
	var head library.SyncBoundary
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &head) != nil {
		t.Fatal(w.Body.String())
	}
	w = syncCall(t, s, "POST", "catalog", credential.Token, map[string]any{"boundary": head, "after": 0, "limit": 10, "max_bytes": 4096})
	var page library.SyncCatalogPage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || !page.Done || len(page.Lines) != 1 {
		t.Fatalf("catalog=%d %s", w.Code, w.Body)
	}
	line, err := journal.UnmarshalLine(page.Lines[0])
	if err != nil || line.Sync == nil || line.Sync.ID != p.ID || line.TS != journal.FormatTime(p.OccurredAt) {
		t.Fatalf("transport altered authoritative envelope: %+v %v", line, err)
	}
	p.OccurredAt = p.OccurredAt.Add(time.Second)
	w = syncCall(t, s, "POST", "submit", credential.Token, p)
	var rejected SyncSubmitResponse
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &rejected) != nil || !rejected.Rejected || rejected.Error == nil || rejected.Result != nil {
		t.Fatalf("conflicting identity not definitively rejected: %d %s", w.Code, w.Body)
	}
	if _, err := l.RevokeSync(t.Context(), credential.ID); err != nil {
		t.Fatal(err)
	}
	w = syncCall(t, s, "GET", "head", credential.Token, nil)
	if w.Code != 401 {
		t.Fatalf("revoked credential still works: %d %s", w.Code, w.Body)
	}
	if w := call(s, "GET", Prefix+"/boxes", "", ""); w.Code != 200 {
		t.Fatal("sync auth changed existing web API access")
	}
}
