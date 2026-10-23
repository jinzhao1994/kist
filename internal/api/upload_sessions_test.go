package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	staging "github.com/jinzhao1994/kist/internal/upload"
	"github.com/jinzhao1994/kist/internal/wire"
)

func sessionFixture(t *testing.T, s *Server, l *library.Library, data string) (domain.Box, sessionStatus, string) {
	t.Helper()
	box, err := l.CreateBox(t.Context(), "resumable", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "box": box.ID.String(), "folder": box.RootFolderID.String(), "files": []staging.File{{Path: "folder/file.bin", LastModified: 1600000000123, Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(data)))}}})
	result := call(s, "POST", Prefix+"/upload-sessions", "create-resumable-001", string(raw))
	return box, decodeSession(t, result), string(raw)
}
func decodeSession(t *testing.T, w *httptest.ResponseRecorder) sessionStatus {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var result sessionStatus
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func sessionChunk(s *Server, id string, offset int64, data string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("PUT", "http://localhost:8473"+Prefix+"/upload-sessions/"+id+"/files/0?offset="+fmt.Sprint(offset), strings.NewReader(data))
	r.Header.Set("Content-Type", "application/octet-stream")
	r.Header.Set("Upload-Length", fmt.Sprint(len(data)))
	r.Header.Set("Upload-SHA256", fmt.Sprintf("%x", sha256.Sum256([]byte(data))))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func sessionURL(id string) string { return Prefix + "/upload-sessions/" + id }
func assertAPIError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status || !strings.Contains(w.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("want %d %s, got %d %s", status, code, w.Code, w.Body)
	}
}

func TestResumableUploadRestartReceiptRebuildAndUndo(t *testing.T) {
	s, l := testServer(t)
	_, state, body := sessionFixture(t, s, l, "abcdefghij")
	id := state.Session.ID
	first := decodeSession(t, sessionChunk(s, id, 0, "abcd"))
	if first.Session.Offsets[0] != 4 {
		t.Fatal(first)
	}
	count, _ := l.OpCount(t.Context())
	if count != 1 {
		t.Fatal("staging wrote archive ops", count)
	}
	other, err := library.Open(l.Layout().Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	restarted, err := New(other, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	recovered := decodeSession(t, call(restarted, "GET", sessionURL(id), "", ""))
	if recovered.Session.Offsets[0] != 4 {
		t.Fatal(recovered)
	}
	decodeSession(t, sessionChunk(restarted, id, 4, "efghij"))
	result := decodeSession(t, call(restarted, "POST", sessionURL(id)+"/complete", "", "{}"))
	if result.OpID == 0 {
		t.Fatal(result)
	}
	if err := restarted.current.Load().uploads.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	// A receipt survives staging deletion, database rebuild and a later undo.
	if err := other.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	retry := decodeSession(t, call(restarted, "POST", sessionURL(id)+"/complete", "", "{}"))
	if retry.OpID != result.OpID {
		t.Fatal(retry)
	}
	files := decodeBody[wire.FileList](t, restarted, "/files")
	if len(files.Files) != 1 {
		t.Fatal(files)
	}
	content := call(restarted, "GET", Prefix+"/files/"+files.Files[0].ID+"/content", "", "")
	if content.Code != 200 || content.Body.String() != "abcdefghij" {
		t.Fatal(content.Code, content.Body)
	}
	if _, err := other.Undo(t.Context(), result.OpID); err != nil {
		t.Fatal(err)
	}
	retry = decodeSession(t, call(restarted, "POST", Prefix+"/upload-sessions", "create-resumable-001", body))
	if retry.OpID != result.OpID {
		t.Fatal(retry)
	}
	if len(decodeBody[wire.FileList](t, restarted, "/files").Files) != 0 {
		t.Fatal("retry resurrected undone upload")
	}
	// Changed business input cannot reuse an already committed creation key.
	changed := strings.Replace(body, "file.bin", "different.bin", 1)
	assertAPIError(t, call(restarted, "POST", Prefix+"/upload-sessions", "create-resumable-001", changed), 409, "request_conflict")
}

func TestResumableTargetChangesAtCommitBoundary(t *testing.T) {
	for _, kind := range []string{"rename", "delete", "unrelated_folder"} {
		t.Run(kind, func(t *testing.T) {
			s, l := testServer(t)
			box, state, body := sessionFixture(t, s, l, "data")
			id := state.Session.ID
			decodeSession(t, sessionChunk(s, id, 0, "data"))
			s.uploadFault = func(point string) error {
				if point != "before_commit" {
					return nil
				}
				switch kind {
				case "rename":
					_, err := l.UpdateBox(t.Context(), box.ID, library.BoxFields{Name: "changed"})
					return err
				case "delete":
					preview, err := l.PreviewBoxDeletion(t.Context(), box.ID)
					if err != nil {
						return err
					}
					_, err = l.DeleteBox(t.Context(), box.ID, preview.Revision)
					return err
				default:
					_, err := l.CreateFolder(t.Context(), box.ID, box.RootFolderID, "unrelated")
					return err
				}
			}
			result := call(s, "POST", sessionURL(id)+"/complete", "", "{}")
			if kind == "unrelated_folder" {
				decodeSession(t, result)
				return
			}
			if kind == "rename" {
				assertAPIError(t, result, 409, "upload_target_changed")
			} else {
				assertAPIError(t, result, 404, "box_not_found")
			}
			if len(decodeBody[wire.FileList](t, s, "/files").Files) != 0 {
				t.Fatal("archived into changed target")
			}
			// A creation response retry retains the original target precondition.
			retry := decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", "create-resumable-001", body))
			if retry.Session.Manifest.Target != state.Session.Manifest.Target {
				t.Fatal("snapshot silently refreshed")
			}
		})
	}
}

func TestResumableCompletionResponseLostAndConcurrentRetry(t *testing.T) {
	s, l := testServer(t)
	_, state, _ := sessionFixture(t, s, l, "data")
	id := state.Session.ID
	decodeSession(t, sessionChunk(s, id, 0, "data"))
	s.uploadFault = func(point string) error {
		if point == "after_commit" {
			return io.ErrUnexpectedEOF
		}
		return nil
	}
	result := decodeSession(t, call(s, "POST", sessionURL(id)+"/complete", "", "{}"))
	if result.OpID == 0 {
		t.Fatal("failed to recover durable receipt", result)
	}
	s.uploadFault = nil
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			w := call(s, "POST", sessionURL(id)+"/complete", "", "{}")
			if w.Code != 200 {
				t.Error(w.Code, w.Body)
			}
		})
	}
	wg.Wait()
	count, _ := l.OpCount(t.Context())
	if count != 2 {
		t.Fatal("duplicate op", count)
	}
	cancel := decodeSession(t, call(s, "DELETE", sessionURL(id), "", ""))
	if cancel.OpID != result.OpID {
		t.Fatal("cancel hid committed result")
	}
}

func TestResumableRejectsCorruptBlocksAndFinalFiles(t *testing.T) {
	s, l := testServer(t)
	_, state, _ := sessionFixture(t, s, l, "good")
	id := state.Session.ID
	assertAPIError(t, sessionChunk(s, id, 2, "od"), 409, "upload_offset")
	decodeSession(t, sessionChunk(s, id, 0, "go"))
	assertAPIError(t, sessionChunk(s, id, 0, "xx"), 400, "upload_chunk_integrity")
	assertAPIError(t, call(s, "POST", sessionURL(id)+"/complete", "", "{}"), 409, "upload_incomplete")
	decodeSession(t, sessionChunk(s, id, 2, "od"))
	// Corrupt after per-block validation; final verification must reject it.
	if err := os.WriteFile(filepath.Join(l.Layout().Root, "uploads", id, "0"), []byte("evil"), 0600); err != nil {
		t.Fatal(err)
	}
	assertAPIError(t, call(s, "POST", sessionURL(id)+"/complete", "", "{}"), 400, "upload_integrity_mismatch")
	count, _ := l.OpCount(t.Context())
	if count != 1 {
		t.Fatal(count)
	}
	decodeSession(t, call(s, "DELETE", sessionURL(id), "", ""))
	decodeSession(t, call(s, "DELETE", sessionURL(id), "", ""))
	assertAPIError(t, call(s, "GET", sessionURL(id), "", ""), 404, "upload_session_missing")
}

type brokenUploadReader struct{ read bool }

func (r *brokenUploadReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.ErrUnexpectedEOF
	}
	r.read = true
	return copy(p, []byte("go")), nil
}
func TestResumableReceiveInterruptionAndAccessGuards(t *testing.T) {
	s, l := testServer(t)
	_, state, body := sessionFixture(t, s, l, "good")
	id := state.Session.ID
	for _, mode := range []string{"interrupted", "bad_digest", "wrong_length", "wrong_host", "wrong_origin", "cross_site", "wrong_media"} {
		t.Run(mode, func(t *testing.T) {
			var reader io.Reader = strings.NewReader("good")
			if mode == "interrupted" {
				reader = &brokenUploadReader{}
			}
			r := httptest.NewRequest("PUT", "http://localhost:8473"+sessionURL(id)+"/files/0?offset=0", reader)
			r.Header.Set("Content-Type", "application/octet-stream")
			r.Header.Set("Upload-Length", "4")
			r.Header.Set("Upload-SHA256", fmt.Sprintf("%x", sha256.Sum256([]byte("good"))))
			switch mode {
			case "wrong_length":
				r.Header.Set("Upload-Length", "3")
			case "bad_digest":
				r.Header.Set("Upload-SHA256", strings.Repeat("0", 64))
			case "wrong_host":
				r.Host = "evil.test"
			case "wrong_origin":
				r.Header.Set("Origin", "http://evil.test")
			case "cross_site":
				r.Header.Set("Sec-Fetch-Site", "cross-site")
			case "wrong_media":
				r.Header.Set("Content-Type", "text/plain")
			}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code == 200 {
				t.Fatal(mode, "accepted")
			}
			got := decodeSession(t, call(s, "GET", sessionURL(id), "", ""))
			if got.Session.Offsets[0] != 0 {
				t.Fatal("bad request advanced progress")
			}
		})
	}
	// Missing identity fields and unsupported versions never create sessions.
	for _, bad := range []string{strings.Replace(body, `"version":1`, `"version":2`, 1), strings.Replace(body, `"size":4,`, "", 1), strings.Replace(body, `folder/file.bin`, `../escape`, 1)} {
		w := call(s, "POST", Prefix+"/upload-sessions", "different-creation", bad)
		if w.Code == 200 {
			t.Fatalf("accepted bad manifest %s", bad)
		}
	}
	// The error offset is structured even when it is zero.
	conflict := sessionChunk(s, id, 1, "ood")
	if !bytes.Contains(conflict.Body.Bytes(), []byte(`"upload_offset":0`)) {
		t.Fatal(conflict.Body)
	}
}

func TestResumableExpiredSessionCanBeDiscarded(t *testing.T) {
	s, l := testServer(t)
	_, state, _ := sessionFixture(t, s, l, "data")
	id := state.Session.ID
	state.Session.Expires = state.Session.Expires.Add(-2 * staging.IdleTTL)
	raw, err := json.Marshal(state.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l.Layout().Root, "uploads", id, "state.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	assertAPIError(t, call(s, "POST", sessionURL(id)+"/resume", "", "{}"), 409, "upload_session_expired")
	decodeSession(t, call(s, "DELETE", sessionURL(id), "", ""))
	if _, err := os.Stat(filepath.Join(l.Layout().Root, "uploads", id)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestStreamingSessionFinalIntegrity(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			s, l := testServer(t)
			box, err := l.CreateBox(t.Context(), "streaming", "", domain.LocalDate{}, domain.LocalDate{})
			if err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(map[string]any{"version": 2, "box": box.ID.String(), "folder": box.RootFolderID.String(), "files": []staging.File{{Path: "file.txt", Size: 8, LastModified: 1000}}})
			status := decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", "streaming-session-001", string(body)))
			id := status.Session.ID
			decodeSession(t, sessionChunk(s, id, 0, "abcd"))
			prefix := fmt.Sprintf(`{"index":0,"offset":4,"sha256":"%x"}`, sha256.Sum256([]byte("abcd")))
			decodeSession(t, call(s, "POST", sessionURL(id)+"/prefix", "", prefix))
			decodeSession(t, sessionChunk(s, id, 4, "efgh"))
			if corrupt {
				if err := os.WriteFile(filepath.Join(l.Layout().Root, "uploads", id, "0"), []byte("xxxxefgh"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			count, _ := l.OpCount(t.Context())
			final := fmt.Sprintf(`{"digests":["%x"]}`, sha256.Sum256([]byte("abcdefgh")))
			result := call(s, "POST", sessionURL(id)+"/complete", "", final)
			if corrupt {
				assertAPIError(t, result, 400, "upload_integrity_mismatch")
				after, _ := l.OpCount(t.Context())
				if after != count {
					t.Fatal("corrupt upload committed")
				}
			} else {
				receipt := decodeSession(t, result)
				if receipt.OpID == 0 {
					t.Fatal("missing receipt")
				}
				retry := decodeSession(t, call(s, "POST", sessionURL(id)+"/complete", "", final))
				if retry.OpID != receipt.OpID {
					t.Fatal("duplicate commit")
				}
				if _, err := l.Verify(t.Context(), library.VerifyOptions{Deep: true}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
