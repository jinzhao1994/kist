package api

import (
	"encoding/json"
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"testing"
)

func TestBoxTrashAPIRequiresFreshConfirmationAndDeduplicates(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "trash test", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	path := Prefix + "/boxes/" + box.ID.String()
	p := decodeBody[library.BoxDeletionPreview](t, s, "/boxes/"+box.ID.String()+"/deletion-preview")
	if p.FileCount != 0 || p.Name != "trash test" || p.Revision <= 0 {
		t.Fatalf("preview: %+v", p)
	}
	body := fmt.Sprintf(`{"revision":%d}`, p.Revision)
	for _, bad := range []string{`{}`, `{"revision":0}`} {
		r := call(s, "DELETE", path, "invalid-delete-key", bad)
		if r.Code != 422 {
			t.Fatalf("unconfirmed: %d %s", r.Code, r.Body)
		}
	}
	if _, err = l.UpdateBox(t.Context(), box.ID, library.BoxFields{Name: "changed"}); err != nil {
		t.Fatal(err)
	}
	stale := call(s, "DELETE", path, "stale-delete-key", body)
	if stale.Code != 409 {
		t.Fatalf("stale: %d %s", stale.Code, stale.Body)
	}
	p = decodeBody[library.BoxDeletionPreview](t, s, "/boxes/"+box.ID.String()+"/deletion-preview")
	body = fmt.Sprintf(`{"revision":%d}`, p.Revision)
	first := call(s, "DELETE", path, "delete-box-key", body)
	if first.Code != 200 {
		t.Fatalf("delete: %d %s", first.Code, first.Body)
	}
	var result library.RequestResult
	if err = json.Unmarshal(first.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if r := call(s, "GET", path, "", ""); r.Code != 404 {
		t.Fatalf("deleted box: %d", r.Code)
	}
	restored := call(s, "POST", Prefix+fmt.Sprintf("/ops/%d/undo", result.OpID), "restore-box-key", `{}`)
	if restored.Code != 200 {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body)
	}
	if err = l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	// A delayed retry after restore and rebuild returns the original receipt;
	// it must never remove the restored box again.
	retry := call(s, "DELETE", path, "delete-box-key", body)
	if retry.Code != 200 || retry.Body.String() != first.Body.String() {
		t.Fatalf("retry: %d %s", retry.Code, retry.Body)
	}
	if r := call(s, "GET", path, "", ""); r.Code != 200 {
		t.Fatalf("retry deleted restored box: %d", r.Code)
	}
}
