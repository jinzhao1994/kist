package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func TestHistoryAPIAndIdempotentUndo(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "original", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.UpdateBox(t.Context(), box.ID, library.BoxFields{Name: "renamed"}); err != nil {
		t.Fatal(err)
	}
	listing := call(s, "GET", Prefix+"/ops?limit=1&box="+box.ID.String(), "", "")
	var page historyList
	if err := json.Unmarshal(listing.Body.Bytes(), &page); err != nil || listing.Code != 200 || len(page.Ops) != 1 || page.Before != 2 {
		t.Fatalf("%d %s %v", listing.Code, listing.Body, err)
	}
	if strings.Contains(listing.Body.String(), "payload") {
		t.Fatal("history list exposed payload")
	}
	detail := call(s, "GET", Prefix+"/ops/2", "", "")
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), "renamed") {
		t.Fatalf("%d %s", detail.Code, detail.Body)
	}
	blocked := call(s, "POST", Prefix+"/ops/1/undo", "undo-blocked-1", `{}`)
	if blocked.Code != 409 || !strings.Contains(blocked.Body.String(), `"blocking_op":2`) {
		t.Fatalf("%d %s", blocked.Code, blocked.Body)
	}
	first := call(s, "POST", Prefix+"/ops/2/undo", "undo-update-2", `{}`)
	retry := call(s, "POST", Prefix+"/ops/2/undo", "undo-update-2", `{}`)
	if first.Code != 200 || retry.Code != 200 || first.Body.String() != retry.Body.String() {
		t.Fatalf("first %d %s retry %d %s", first.Code, first.Body, retry.Code, retry.Body)
	}
	duplicate := call(s, "POST", Prefix+"/ops/2/undo", "undo-other-2", `{}`)
	if duplicate.Code != 409 || !strings.Contains(duplicate.Body.String(), `"reverted_by":3`) {
		t.Fatalf("%d %s", duplicate.Code, duplicate.Body)
	}
	older := call(s, "GET", Prefix+fmt.Sprintf("/ops?before=%d&limit=1", page.Before), "", "")
	if older.Code != 200 || !strings.Contains(older.Body.String(), `"id":1`) {
		t.Fatalf("%d %s", older.Code, older.Body)
	}
	undone := call(s, "GET", Prefix+"/ops/2", "", "")
	if !strings.Contains(undone.Body.String(), `"undone_by":3`) {
		t.Fatal(undone.Body)
	}
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	replay := call(s, "POST", Prefix+"/ops/2/undo", "undo-update-2", `{}`)
	if replay.Code != 200 || replay.Body.String() != first.Body.String() {
		t.Fatalf("rebuild retry: %d %s", replay.Code, replay.Body)
	}
}
func TestHistoryAPIRejectsInvalidRequests(t *testing.T) {
	s, _ := testServer(t)
	for _, path := range []string{"/ops?limit=1001", "/ops?before=-1", "/ops?box=bad", "/ops?file=bad", "/ops?action=bad", "/ops/0"} {
		r := call(s, "GET", Prefix+path, "", "")
		if r.Code < 400 || r.Code >= 500 {
			t.Errorf("%s: %d %s", path, r.Code, r.Body)
		}
	}
	if r := call(s, "GET", Prefix+"/ops/99", "", ""); r.Code != 404 {
		t.Fatalf("%d %s", r.Code, r.Body)
	}
	if r := call(s, "POST", Prefix+"/ops/1/undo", "", `{}`); r.Code < 400 || r.Code >= 500 {
		t.Fatalf("%d %s", r.Code, r.Body)
	}
}
