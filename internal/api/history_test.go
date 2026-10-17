package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
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
	summaryResponse := call(s, "GET", Prefix+"/ops/2/summary", "", "")
	var summary library.HistorySummary
	if err := json.Unmarshal(summaryResponse.Body.Bytes(), &summary); err != nil || summaryResponse.Code != 200 || summary.Before != "original" || summary.After != "renamed" || summary.CurrentNames {
		t.Fatalf("summary %d %s %v", summaryResponse.Code, summaryResponse.Body, err)
	}
	if missing := call(s, "GET", Prefix+"/ops/9999/summary", "", ""); missing.Code != 404 {
		t.Fatalf("missing summary: %d", missing.Code)
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

func TestHistoryGroupFilterAndAtomicRestoreConflict(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	box, err := l.CreateBox(ctx, "history group", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	importFiles := func(names ...string) {
		t.Helper()
		dir := t.TempDir()
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		root := int64(2)
		opt := library.ImportOptions{Box: box.ID}
		if n, _ := l.OpCount(ctx); n >= root {
			opt.RootOp = &root
		}
		if _, err := l.Import(ctx, dir, opt); err != nil {
			t.Fatal(err)
		}
	}
	importFiles("first.txt")               // #2 heads a group.
	importFiles("second.txt", "third.txt") // #3 joins it.
	for _, member := range []int{2, 3} {
		page := decodeBody[historyList](t, s, fmt.Sprintf("/ops?group=%d&action=import_files&limit=1", member))
		if len(page.Ops) != 1 || page.Ops[0].ID != 3 || page.Before != 3 {
			t.Fatalf("group member %d: %+v", member, page)
		}
		older := decodeBody[historyList](t, s, fmt.Sprintf("/ops?group=%d&before=3", member))
		if len(older.Ops) != 1 || older.Ops[0].ID != 2 {
			t.Fatalf("group older: %+v", older)
		}
	}
	mustWrite(t, s, "POST", "/ops/3/undo", "delete-group-member", `{}`) // #4, outside group.
	// Occupy one of the deleted batch's names using a new file identity.
	importFiles("third.txt") // #5
	before := decodeBody[wire.FileList](t, s, "/files")
	conflict := call(s, "POST", Prefix+"/ops/4/undo", "restore-name-conflict", `{}`)
	if conflict.Code != 409 || !strings.Contains(conflict.Body.String(), `"blocking_op":5`) {
		t.Fatalf("restore conflict: %d %s", conflict.Code, conflict.Body)
	}
	after := decodeBody[wire.FileList](t, s, "/files")
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("conflicting restore partially changed the batch")
	}
	if count, _ := l.OpCount(ctx); count != 5 {
		t.Fatalf("failed restore wrote an event: %d", count)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	group := decodeBody[historyList](t, s, "/ops?group=3")
	if len(group.Ops) != 3 || group.Ops[0].ID != 5 || group.Ops[1].ID != 3 || group.Ops[2].ID != 2 {
		t.Fatalf("rebuilt group included undo or lost members: %+v", group)
	}
	trash := decodeBody[historyList](t, s, "/ops?trash=true")
	if len(trash.Ops) != 1 || trash.Ops[0].ID != 4 {
		t.Fatalf("failed restore changed recycle bin: %+v", trash)
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

func TestTrashFiltersBeforePagingAndRestoresAtomically(t *testing.T) {
	s, l := testServer(t)
	for _, name := range []string{"first", "second"} {
		tag, err := l.CreateTag(t.Context(), name, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := l.DeleteTag(t.Context(), tag.ID); err != nil {
			t.Fatal(err)
		}
	}
	page := decodeBody[historyList](t, s, "/ops?trash=true&limit=1")
	if len(page.Ops) != 1 || page.Ops[0].ID != 4 || page.Before != 4 {
		t.Fatalf("page=%+v", page)
	}
	older := decodeBody[historyList](t, s, "/ops?trash=true&limit=1&before=4")
	if len(older.Ops) != 1 || older.Ops[0].ID != 2 || older.Before != 0 {
		t.Fatalf("older=%+v", older)
	}
	mustWrite(t, s, "POST", "/ops/4/undo", "trash-restore-4", `{}`)
	remaining := decodeBody[historyList](t, s, "/ops?trash=true")
	if len(remaining.Ops) != 1 || remaining.Ops[0].ID != 2 {
		t.Fatalf("remaining=%+v", remaining)
	}
	mustWrite(t, s, "POST", "/ops/5/undo", "trash-redelete-5", `{}`)
	latest := decodeBody[historyList](t, s, "/ops?trash=true")
	if len(latest.Ops) != 2 || latest.Ops[0].ID != 6 {
		t.Fatalf("latest=%+v", latest)
	}
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	rebuilt := decodeBody[historyList](t, s, "/ops?trash=true")
	if len(rebuilt.Ops) != 2 || rebuilt.Ops[0].ID != 6 {
		t.Fatalf("rebuilt=%+v", rebuilt)
	}
}
