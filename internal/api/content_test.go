package api

import (
	"bytes"
	"mime"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestOriginalContentAndRanges(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "files", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	fixtures := map[string][]byte{
		"合同.html":    []byte("<!DOCTYPE html><script>alert(1)</script>"),
		"active.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`),
		"image.jpg":  append([]byte{0xff, 0xd8, 0xff}, bytes.Repeat([]byte{0}, 600)...),
		"empty.bin":  {},
		"report.pdf": []byte("%PDF-1.7\nPDF test bytes"),
		"fake.pdf":   []byte("<!DOCTYPE html><script>alert(1)</script>"),
	}
	for name, data := range fixtures {
		if err := os.WriteFile(filepath.Join(src, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.Import(t.Context(), src, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	for _, file := range files.Files {
		t.Run(file.Name, func(t *testing.T) {
			path := Prefix + "/files/" + file.ID + "/content"
			w := call(s, "GET", path, "", "")
			if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), fixtures[file.Name]) {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			disposition, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
			if err != nil || params["filename"] != file.Name {
				t.Fatalf("disposition=%v %v", params, err)
			}
			expected := "attachment"
			if file.Name == "image.jpg" || file.Name == "report.pdf" {
				expected = "inline"
			}
			if disposition != expected {
				t.Fatal(disposition)
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
				t.Fatal(w.Header())
			}
			head := call(s, "HEAD", path, "", "")
			if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") != w.Header().Get("Content-Length") {
				t.Fatalf("HEAD %d %s %v", head.Code, head.Body, head.Header())
			}
			forced := call(s, "GET", path+"?download=true", "", "")
			if !strings.HasPrefix(forced.Header().Get("Content-Disposition"), "attachment") {
				t.Fatal(forced.Header())
			}
			if len(fixtures[file.Name]) > 8 {
				r := httptest.NewRequest("GET", "http://localhost:8473"+path, nil)
				r.Header.Set("Range", "bytes=2-7")
				part := httptest.NewRecorder()
				s.ServeHTTP(part, r)
				if part.Code != 206 || !bytes.Equal(part.Body.Bytes(), fixtures[file.Name][2:8]) {
					t.Fatalf("range %d %s", part.Code, part.Body)
				}
				r.Header.Set("Range", "bytes=99999-")
				part = httptest.NewRecorder()
				s.ServeHTTP(part, r)
				if part.Code != 416 {
					t.Fatalf("invalid range: %d", part.Code)
				}
			}
		})
	}
	w := call(s, "GET", Prefix+"/files/"+domain.NewFileID().String()+"/content", "", "")
	if w.Code != 404 {
		t.Fatalf("missing=%d %s", w.Code, w.Body)
	}
}
