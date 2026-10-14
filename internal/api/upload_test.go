package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/jinzhao1994/kist/internal/library"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/wire"
)

type uploadFixture struct{ path, data string }

func multipartUpload(t *testing.T, manifest []uploadFile, files []uploadFixture) ([]byte, string) {
	t.Helper()
	manifest = append([]uploadFile(nil), manifest...)
	for i := range manifest {
		if manifest[i].SHA256 != "" {
			continue
		}
		for _, file := range files {
			if file.path == manifest[i].Path {
				digest := sha256.Sum256([]byte(file.data))
				manifest[i].SHA256 = hex.EncodeToString(digest[:])
				manifest[i].Size = int64(len(file.data))
				break
			}
		}
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	field, err := writer.CreateFormField("manifest")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(field).Encode(uploadManifest{Files: manifest}); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(file.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}
func upload(t *testing.T, s *Server, box, key string, body []byte, media string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "http://localhost:8473"+Prefix+"/boxes/"+box+"/uploads", bytes.NewReader(body))
	r.Header.Set("Content-Type", media)
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestUploadBatchRetryAndContent(t *testing.T) {
	s, l := testServer(t)
	s.uploadTempDir = t.TempDir()
	box, err := l.CreateBox(t.Context(), "upload", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	manifest := []uploadFile{{Path: "目录/合同:一.txt", LastModified: 1600000000123}, {Path: "photo.txt", LastModified: 1600000000000}}
	files := []uploadFixture{{manifest[0].Path, "contract"}, {manifest[1].Path, "photo"}}
	body, media := multipartUpload(t, manifest, files)
	first := upload(t, s, box.ID.String(), "upload-batch-01", body, media)
	if first.Code != 200 {
		t.Fatalf("%d %s", first.Code, first.Body)
	}
	// Reorder manifest and file parts and regenerate the random boundary.
	secondBody, secondMedia := multipartUpload(t, []uploadFile{manifest[1], manifest[0]}, []uploadFixture{files[1], files[0]})
	second := upload(t, s, box.ID.String(), "upload-batch-01", secondBody, secondMedia)
	if second.Code != 200 || second.Body.String() != first.Body.String() {
		t.Fatalf("retry %d %s", second.Code, second.Body)
	}
	n, err := l.OpCount(t.Context())
	if err != nil || n != 2 {
		t.Fatalf("ops=%d %v", n, err)
	}
	page := decodeBody[wire.FileList](t, s, "/files?sort=name&asc=true")
	if len(page.Files) != 2 {
		t.Fatal(page)
	}
	for _, file := range page.Files {
		w := call(s, "GET", Prefix+"/files/"+file.ID+"/content", "", "")
		want := "photo"
		if file.Name == "合同:一.txt" {
			want = "contract"
			if file.Path != "目录/合同:一.txt" {
				t.Fatal(file.Path)
			}
		}
		if w.Body.String() != want {
			t.Fatalf("content=%s", w.Body)
		}
		if file.CapturedAt == nil || file.CapturedAt.UnixMilli() != 1600000000000 && file.CapturedAt.UnixMilli() != 1600000000123 {
			t.Fatalf("mtime not preserved: %+v", file)
		}
	}
	files[0].data = "different"
	body, media = multipartUpload(t, manifest, files)
	conflict := upload(t, s, box.ID.String(), "upload-batch-01", body, media)
	if conflict.Code != 409 {
		t.Fatalf("conflict=%d %s", conflict.Code, conflict.Body)
	}
	entries, err := os.ReadDir(s.uploadTempDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("staging not cleaned: %v %v", entries, err)
	}
}
func TestUploadRejectsIncompleteOrUnsafeBatches(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest []uploadFile
		files    []uploadFixture
		truncate bool
		limit    int64
		count    int
	}{
		{name: "traversal", manifest: []uploadFile{{Path: "../secret"}}},
		{name: "absolute", manifest: []uploadFile{{Path: "/secret"}}},
		{name: "backslash", manifest: []uploadFile{{Path: `a\b`}}},
		{name: "duplicate folded", manifest: []uploadFile{{Path: "a"}, {Path: "A"}}},
		{name: "missing file", manifest: []uploadFile{{Path: "a"}}},
		{name: "duplicate part", manifest: []uploadFile{{Path: "a"}}, files: []uploadFixture{{"a", "one"}, {"a", "two"}}},
		{name: "unexpected part", manifest: []uploadFile{{Path: "a"}}, files: []uploadFixture{{"b", "one"}}},
		{name: "truncated", manifest: []uploadFile{{Path: "a"}}, files: []uploadFixture{{"a", "one"}}, truncate: true},
		{name: "byte limit", manifest: []uploadFile{{Path: "a"}}, files: []uploadFixture{{"a", strings.Repeat("x", 2048)}}, limit: 1024},
		{name: "file limit", manifest: []uploadFile{{Path: "a"}, {Path: "b"}}, count: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, l := testServer(t)
			s.uploadTempDir = t.TempDir()
			if tc.limit > 0 {
				s.maxUploadBytes = tc.limit
			}
			if tc.count > 0 {
				s.maxUploadFiles = tc.count
			}
			box, err := l.CreateBox(t.Context(), "upload", "", domain.LocalDate{}, domain.LocalDate{})
			if err != nil {
				t.Fatal(err)
			}
			body, media := multipartUpload(t, tc.manifest, tc.files)
			if tc.truncate {
				body = body[:len(body)-10]
			}
			w := upload(t, s, box.ID.String(), "bad-upload-001", body, media)
			if w.Code != 400 && w.Code != 413 {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			n, _ := l.OpCount(t.Context())
			if n != 1 {
				t.Fatalf("rejected upload wrote ops: %d", n)
			}
			entries, err := os.ReadDir(s.uploadTempDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("residue: %v %v", entries, err)
			}
		})
	}
}
func TestCancelledUploadDoesNotCommit(t *testing.T) {
	s, l := testServer(t)
	s.uploadTempDir = t.TempDir()
	box, err := l.CreateBox(t.Context(), "upload", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	body, media := multipartUpload(t, []uploadFile{{Path: "a"}}, []uploadFixture{{"a", "one"}})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := httptest.NewRequest("POST", "http://localhost:8473"+Prefix+"/boxes/"+box.ID.String()+"/uploads", bytes.NewReader(body)).WithContext(ctx)
	r.Header.Set("Content-Type", media)
	r.Header.Set("Idempotency-Key", "cancel-upload-01")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	n, _ := l.OpCount(t.Context())
	if n != 1 || w.Code == 200 {
		t.Fatalf("cancelled upload ops=%d response=%d", n, w.Code)
	}
	entries, _ := os.ReadDir(s.uploadTempDir)
	if len(entries) != 0 {
		t.Fatal(entries)
	}
}

func TestConcurrentUploadAndRestart(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "upload", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	body, media := multipartUpload(t, []uploadFile{{Path: "a", LastModified: 1600000000000}}, []uploadFixture{{"a", "one"}})
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 4)
	for range 4 {
		wg.Go(func() { results <- upload(t, s, box.ID.String(), "concurrent-upload", body, media) })
	}
	wg.Wait()
	close(results)
	receipt := ""
	for result := range results {
		if result.Code != 200 {
			t.Fatalf("%d %s", result.Code, result.Body)
		}
		if receipt != "" && receipt != result.Body.String() {
			t.Fatal("different receipts")
		}
		receipt = result.Body.String()
	}
	n, _ := l.OpCount(t.Context())
	if n != 2 {
		t.Fatalf("ops=%d", n)
	}
	other, err := library.Open(l.Layout().Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	reopened, err := New(other, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	result := upload(t, reopened, box.ID.String(), "concurrent-upload", body, media)
	if result.Code != 200 || result.Body.String() != receipt {
		t.Fatalf("restart=%d %s", result.Code, result.Body)
	}
	var committed library.RequestResult
	if err := json.Unmarshal(result.Body.Bytes(), &committed); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Undo(t.Context(), committed.OpID); err != nil {
		t.Fatal(err)
	}
	result = upload(t, reopened, box.ID.String(), "concurrent-upload", body, media)
	if result.Code != 200 || result.Body.String() != receipt {
		t.Fatalf("after undo=%d %s", result.Code, result.Body)
	}
	page := decodeBody[wire.FileList](t, reopened, "/files")
	if len(page.Files) != 0 {
		t.Fatal("retry resurrected an undone import")
	}
}

func TestStreamingOpenAPIContract(t *testing.T) {
	s, _ := testServer(t)
	doc := decodeBody[map[string]any](t, s, "/openapi.json")
	paths := doc["paths"].(map[string]any)
	operation := paths[Prefix+"/boxes/{id}/uploads"].(map[string]any)["post"].(map[string]any)
	content := operation["requestBody"].(map[string]any)["content"].(map[string]any)
	if content["multipart/form-data"] == nil {
		t.Fatal("multipart body missing from OpenAPI")
	}
	response := operation["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)
	if !strings.Contains(response["schema"].(map[string]any)["$ref"].(string), "RequestResult") {
		t.Fatal(response)
	}
	download := paths[Prefix+"/files/{id}/content"].(map[string]any)["get"].(map[string]any)
	binary := download["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/octet-stream"].(map[string]any)["schema"].(map[string]any)
	if binary["format"] != "binary" {
		t.Fatal(binary)
	}
}

func TestUploadIntegrityRejectsBatch(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		size       int64
	}{
		{"same length changed", "modified", 8},
		{"wrong size", "original", 9},
		{"truncated content", "orig", 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, l := testServer(t)
			s.uploadTempDir = t.TempDir()
			box, err := l.CreateBox(t.Context(), "integrity", "", domain.LocalDate{}, domain.LocalDate{})
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte("original"))
			manifest := []uploadFile{{Path: "good.txt"}, {Path: "bad.txt", SHA256: hex.EncodeToString(digest[:]), Size: tc.size}}
			body, media := multipartUpload(t, manifest, []uploadFixture{{"good.txt", "valid"}, {"bad.txt", tc.data}})
			response := upload(t, s, box.ID.String(), "integrity-batch-01", body, media)
			if response.Code != 400 || !strings.Contains(response.Body.String(), "upload_integrity_mismatch") {
				t.Fatalf("%d %s", response.Code, response.Body)
			}
			n, err := l.OpCount(t.Context())
			if err != nil || n != 1 {
				t.Fatalf("ops=%d %v", n, err)
			}
			if page := decodeBody[wire.FileList](t, s, "/files"); len(page.Files) != 0 {
				t.Fatal("mismatching batch archived")
			}
			entries, err := os.ReadDir(s.uploadTempDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary files: %v %v", entries, err)
			}
		})
	}
}

func TestUploadRejectsLegacyManifest(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "legacy", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	field, err := writer.CreateFormField("manifest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := field.Write([]byte(`{"files":[{"path":"old.txt","last_modified":0}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	response := upload(t, s, box.ID.String(), "legacy-upload-01", body.Bytes(), writer.FormDataContentType())
	if response.Code != 400 || !strings.Contains(response.Body.String(), "refresh") {
		t.Fatalf("%d %s", response.Code, response.Body)
	}
	n, err := l.OpCount(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("ops=%d %v", n, err)
	}
}
