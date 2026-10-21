package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestSyncOriginalHTTPResumeAndRange(t *testing.T) {
	s, l := testServer(t)
	credential, err := l.AuthorizeSync(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	content := "offline-original-content"
	key := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
	create := func() syncOriginalUpload {
		t.Helper()
		w := syncCall(t, s, "POST", "original-uploads", credential.Token, map[string]any{"key": key, "size": len(content)})
		var upload syncOriginalUpload
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &upload) != nil || upload.ID == "" {
			t.Fatalf("create: %d %s", w.Code, w.Body)
		}
		return upload
	}
	upload := create()
	chunk := func(offset int, data string) {
		t.Helper()
		r := httptest.NewRequest("PUT", "http://localhost:8473"+Prefix+"/sync/original-uploads/"+upload.ID+"/files/0?offset="+strconv.Itoa(offset), strings.NewReader(data))
		r.Header.Set("Authorization", "Bearer "+credential.Token)
		r.Header.Set("Content-Type", "application/octet-stream")
		r.Header.Set("Upload-Length", strconv.Itoa(len(data)))
		r.Header.Set("Upload-SHA256", fmt.Sprintf("%x", sha256.Sum256([]byte(data))))
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("chunk: %d %s", w.Code, w.Body)
		}
	}
	chunk(0, content[:8])
	chunk(0, content[:8])
	// Reopen the server's staging store as a new process would.
	s, err = New(l, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	resumed := create()
	if resumed.ID != upload.ID || resumed.Offset != 8 {
		t.Fatalf("lost durable progress: %+v", resumed)
	}
	w := syncCall(t, s, "POST", "original-uploads/"+upload.ID+"/prefix", credential.Token, map[string]any{"index": 0, "offset": 8, "sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(content[:8])))})
	if w.Code != 200 {
		t.Fatalf("prefix: %d %s", w.Code, w.Body)
	}
	chunk(8, content[8:])
	w = syncCall(t, s, "POST", "original-uploads/"+upload.ID+"/finish", credential.Token, struct{}{})
	if w.Code != 200 {
		t.Fatalf("finish: %d %s", w.Code, w.Body)
	}
	w = syncCall(t, s, "GET", "originals/"+key, credential.Token, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"present":true`) {
		t.Fatalf("probe after lost finish reply: %d %s", w.Code, w.Body)
	}
	r := httptest.NewRequest("GET", "http://localhost:8473"+Prefix+"/sync/originals/"+key+"/content", nil)
	r.Header.Set("Authorization", "Bearer "+credential.Token)
	r.Header.Set("Range", "bytes=8-")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusPartialContent || w.Body.String() != content[8:] || w.Header().Get("ETag") != `"`+key+`"` {
		t.Fatalf("range: %d %s", w.Code, w.Body)
	}
	if count, _ := l.OpCount(t.Context()); count != 0 {
		t.Fatal("original upload created an archive event")
	}
}
