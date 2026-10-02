package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestExtractionKindsAndRetry(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "types", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	fixtures := []uploadFixture{{"fake.jpg", "%PDF-1.7\n"}, {"empty", ""}, {"unlabelled", string([]byte{255, 216, 255, 0})}, {"report.txt", "hello"}}
	manifest := make([]uploadFile, 0, len(fixtures))
	for _, f := range fixtures {
		manifest = append(manifest, uploadFile{Path: f.path, LastModified: 1600000000000})
	}
	body, media := multipartUpload(t, manifest, fixtures)
	w := upload(t, s, box.ID.String(), "type-upload-001", body, media)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	unknown := decodeBody[wire.FileList](t, s, "/files?kind=other")
	if len(unknown.Files) != 0 {
		t.Fatal("unprocessed files were classified as other")
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	for _, file := range files.Files {
		if len(file.ExtractionPending) != 2 || file.Kind != "" {
			t.Fatalf("initial=%+v", file)
		}
		route := "/files/" + file.ID + "/extractions/media"
		first := call(s, "POST", Prefix+route, "media-"+file.ID, `{}`)
		if first.Code != 200 {
			t.Fatalf("%d %s", first.Code, first.Body)
		}
		retry := call(s, "POST", Prefix+route, "media-"+file.ID, `{}`)
		if retry.Body.String() != first.Body.String() {
			t.Fatal("retry changed receipt")
		}
		detail := decodeBody[fileDetail](t, s, "/files/"+file.ID)
		if len(detail.File.ExtractionPending) != 1 || detail.File.ExtractionPending[0] != "exif" {
			t.Fatal(detail)
		}
		if file.Name == "fake.jpg" && detail.File.Kind != "document" {
			t.Fatal("extension overrode bytes")
		}
		if file.Name == "empty" {
			mustWrite(t, s, "POST", "/files/"+file.ID+"/extractions/exif", "empty-exif-001", `{}`)
		}
	}
	for _, tc := range []struct {
		kind  string
		count int
	}{{"document", 2}, {"image", 1}, {"other", 1}, {"audio", 0}, {"video", 0}, {"document,image", 3}} {
		page := decodeBody[wire.FileList](t, s, "/files?kind="+tc.kind)
		if len(page.Files) != tc.count {
			t.Fatalf("%s: %d", tc.kind, len(page.Files))
		}
	}
	page := decodeBody[wire.FileList](t, s, "/files?kind=document&sort=name&asc=true&limit=1")
	if len(page.Files) != 1 || page.Cursor == "" {
		t.Fatal(page)
	}
	next := decodeBody[wire.FileList](t, s, "/files?kind=document&sort=name&asc=true&limit=1&cursor="+page.Cursor)
	if len(next.Files) != 1 || next.Files[0].ID == page.Files[0].ID || next.Cursor != "" {
		t.Fatal(next)
	}
	bad := call(s, "GET", Prefix+"/files?kind=wrong", "", "")
	if bad.Code != 400 {
		t.Fatal(bad.Code)
	}
	// An unavailable blob does not claim extraction success.
	var image wire.File
	for _, f := range files.Files {
		if f.Name == "unlabelled" {
			image = f
		}
	}
	id, err := domain.ParseID(image.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, handle, err := l.OpenContent(t.Context(), domain.FileID{ID: id})
	if err != nil {
		t.Fatal(err)
	}
	blobPath := handle.Name()
	handle.Close()
	if err := os.Rename(blobPath, blobPath+".test-missing"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(blobPath+".test-missing", blobPath) })
	failed := call(s, "POST", Prefix+"/files/"+image.ID+"/extractions/exif", "bad-exif-0001", `{}`)
	if failed.Code == 200 {
		t.Fatal("unavailable blob unexpectedly extracted")
	}
	detail := decodeBody[fileDetail](t, s, "/files/"+image.ID)
	if strings.Join(detail.File.ExtractionPending, ",") != "exif" {
		t.Fatal(detail)
	}
	// A successful no-op receives a durable receipt, still distinct from the
	// original extraction event.
	noop := call(s, "POST", Prefix+"/files/"+image.ID+"/extractions/media", "noop-media-001", `{}`)
	var receipt library.RequestResult
	if err := json.Unmarshal(noop.Body.Bytes(), &receipt); err != nil || receipt.OpID == 0 {
		t.Fatalf("%s %v", noop.Body, err)
	}
}
