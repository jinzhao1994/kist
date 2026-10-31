package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func saveRuntimeSettings(t *testing.T, s *Server, changes map[string]any) {
	t.Helper()
	current := decodeBody[SettingsResponse](t, s, "/settings")
	raw, _ := json.Marshal(map[string]any{"revision": current.Revision, "changes": changes})
	result := call(s, "PUT", Prefix+"/settings", "", string(raw))
	if result.Code != 200 {
		t.Fatalf("save: %d %s", result.Code, result.Body)
	}
}

func TestRuntimeSettingsSaveKeepsUploadsAndHistory(t *testing.T) {
	s, l := testServer(t)
	_, upload, manifest := sessionFixture(t, s, l, "abcdefghij")
	var multi map[string]any
	if err := json.Unmarshal([]byte(manifest), &multi); err != nil {
		t.Fatal(err)
	}
	files := multi["files"].([]any)
	second := map[string]any{}
	for k, v := range files[0].(map[string]any) {
		second[k] = v
	}
	second["path"] = "second.bin"
	multi["files"] = append(files, second)
	multiBody, _ := json.Marshal(multi)
	accepted := decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", "multi-before-reload", string(multiBody)))
	before, _ := l.OpCount(t.Context())
	old := s.current.Load()
	captured := context.WithValue(t.Context(), runtimeKey{}, old)
	saveRuntimeSettings(t, s, map[string]any{"server.max_upload_bytes": 2, "server.max_upload_files": 1, "library.timezone": "America/New_York", "storage.link_mode": "copy", "server.map_style": "https://maps.example/style.json", "server.allowed_origins": []string{"https://trusted.example"}})
	if l.Zone().String() != "America/New_York" {
		t.Fatal(l.Zone())
	}
	if s.runtime(captured) != old || s.current.Load() == old {
		t.Fatal("request snapshot was replaced")
	}
	if info := decodeBody[libraryInfo](t, s, "/info"); info.MaxUploadBytes != 2 || info.MaxUploadFiles != 1 {
		t.Fatal(info)
	}
	if provider := decodeBody[MapProvider](t, s, "/map-provider"); provider.StyleURL != "https://maps.example/style.json" {
		t.Fatal(provider)
	}
	r := httptest.NewRequest("GET", "http://localhost:8473"+Prefix+"/info", nil)
	r.Header.Set("Origin", "https://trusted.example")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "https://maps.example") {
		t.Fatal(w)
	}
	after, _ := l.OpCount(t.Context())
	if before != after {
		t.Fatal("reload wrote history")
	}
	// Lost creation replies remain retryable under both the new size and count limits.
	retry := decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", "multi-before-reload", string(multiBody)))
	if retry.Session.ID != accepted.Session.ID {
		t.Fatal("retry lost session")
	}
	if fresh := call(s, "POST", Prefix+"/upload-sessions", "multi-after-reload", string(multiBody)); fresh.Code != 400 {
		t.Fatal("new upload ignored limit", fresh.Code)
	}
	// A lower limit affects new uploads, not blocks belonging to an accepted session.
	decodeSession(t, sessionChunk(s, upload.Session.ID, 0, "abcdefghij"))
	decodeSession(t, call(s, "POST", sessionURL(upload.Session.ID)+"/complete", "", "{}"))
	decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", "create-resumable-001", manifest))
	saveRuntimeSettings(t, s, map[string]any{"server.allowed_origins": []string{}})
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("old origin still accepted", w.Code)
	}
}

func TestRuntimeSettingsReloadRejectsInvalidFile(t *testing.T) {
	s, l := testServer(t)
	path := l.Layout().ConfigPath()
	raw, _ := os.ReadFile(path)
	if err := config.Update(path, "", map[string]any{"server.max_upload_files": int64(7), "library.timezone": "UTC"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReloadSettings(t.Context()); err != nil {
		t.Fatal(err)
	}
	active := s.current.Load()
	if active.maxUploadFiles != 7 {
		t.Fatal(active.maxUploadFiles)
	}
	if err := os.WriteFile(path, []byte("broken = ["), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReloadSettings(t.Context()); err == nil {
		t.Fatal("invalid file accepted")
	}
	if s.current.Load() != active {
		t.Fatal("invalid reload replaced runtime")
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeSettingsConcurrentRequests(t *testing.T) {
	s, _ := testServer(t)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 20 {
				w := call(s, "GET", Prefix+"/map-provider", "", "")
				if w.Code != 200 {
					t.Error(w.Code)
				}
			}
		})
	}
	for range 5 {
		if err := s.ReloadSettings(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}

func TestClientRuntimeSettingsReloadPreservesPendingWork(t *testing.T) {
	_, nas := testServer(t)
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	client, err := library.CreateClient(t.Context(), t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	box, err := client.CreateBox(t.Context(), "offline work", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewClient(client, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	saveRuntimeSettings(t, s, map[string]any{"server.max_upload_files": 4})
	if err := s.ReloadSettings(t.Context()); err != nil {
		t.Fatal(err)
	}
	if info := decodeBody[libraryInfo](t, s, "/info"); info.MaxUploadFiles != 4 {
		t.Fatal(info)
	}
	result := call(s, "GET", Prefix+"/boxes/"+box.ID.String(), "", "")
	if result.Code != 200 || !strings.Contains(result.Body.String(), "offline work") {
		t.Fatal("pending work lost", result.Code)
	}
}
