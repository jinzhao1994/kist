package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
)

func TestInstanceSettingsCASDefaultsAndSecrets(t *testing.T) {
	s, l := testServer(t)
	credential, err := l.AuthorizeSync(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	fetch := func() settingsResponse {
		t.Helper()
		w := call(s, "GET", Prefix+"/settings", "", "")
		if w.Code != 200 {
			t.Fatalf("settings: %d %s", w.Code, w.Body)
		}
		if strings.Contains(w.Body.String(), credential.Token) || strings.Contains(w.Body.String(), "digest") {
			t.Fatal("settings leaked credential")
		}
		var v settingsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	initial := fetch()
	save := func(revision string, changes map[string]any) int {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"revision": revision, "changes": changes})
		return call(s, "PUT", Prefix+"/settings", "", string(body)).Code
	}
	if code := save(initial.Revision, map[string]any{"appearance.primary_color": "green", "server.max_upload_files": 123}); code != 200 {
		t.Fatalf("save=%d", code)
	}
	if code := save(initial.Revision, map[string]any{"appearance.primary_color": "pink"}); code != 409 {
		t.Fatalf("stale save=%d", code)
	}
	changed := fetch()
	if len(changed.RestartRequired) != 0 {
		t.Fatalf("restart=%v", changed.RestartRequired)
	}
	info := call(s, "GET", Prefix+"/info", "", "")
	if !strings.Contains(info.Body.String(), `"primary_color":"green"`) || !strings.Contains(info.Body.String(), `"max_upload_files":123`) {
		t.Fatal("live color did not change")
	}
	if code := save(changed.Revision, map[string]any{"appearance.primary_color": nil}); code != 200 {
		t.Fatalf("reset=%d", code)
	}
	cfg, err := config.Load(l.Layout().ConfigPath())
	if err != nil || cfg.Appearance.PrimaryColor != "blue" {
		t.Fatalf("reset=%v", err)
	}
	raw, _ := os.ReadFile(l.Layout().ConfigPath())
	if strings.Contains(string(raw), "primary_color") {
		t.Fatal("reset wrote explicit default")
	}
	fresh := fetch()
	if code := save(fresh.Revision, map[string]any{"library.id": "replacement"}); code != 400 {
		t.Fatalf("identity edit=%d", code)
	}
	if ok, err := l.CheckSyncCredential(t.Context(), credential.Token); err != nil || !ok {
		t.Fatalf("unrelated settings lost credential: %v", err)
	}
	if count, _ := l.OpCount(t.Context()); count != 0 {
		t.Fatal("settings created archive ops")
	}
}

func TestSettingsStorageUsage(t *testing.T) {
	s, _ := testServer(t)
	w := call(s, "GET", Prefix+"/settings/storage", "", "")
	if w.Code != 200 {
		t.Fatalf("storage: %d %s", w.Code, w.Body)
	}
	var usage map[string]int64
	if err := json.Unmarshal(w.Body.Bytes(), &usage); err != nil {
		t.Fatal(err)
	}
	if len(usage) != 3 || usage["metadata_bytes"] <= 0 {
		t.Fatalf("usage: %v", usage)
	}
}

func TestCredentialNotesDoNotRotateOrRestoreTokens(t *testing.T) {
	s, l := testServer(t)
	w := call(s, "POST", Prefix+"/settings/credentials", "", `{"note":"  旅行备用机  "}`)
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var created struct {
		ID    string `json:"id"`
		Token string `json:"token"`
		Note  string `json:"note"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Note != "旅行备用机" {
		t.Fatalf("note: %q", created.Note)
	}
	for _, note := range []string{"旅行电脑", ""} {
		body, _ := json.Marshal(map[string]string{"note": note})
		w = call(s, "PUT", Prefix+"/settings/credentials/"+created.ID, "", string(body))
		if w.Code != 200 {
			t.Fatalf("rename: %d %s", w.Code, w.Body)
		}
		if ok, err := l.CheckSyncCredential(t.Context(), created.Token); err != nil || !ok {
			t.Fatalf("token changed: %v", err)
		}
		items, err := l.SyncCredentials(t.Context())
		if err != nil || len(items) != 1 || items[0].Note != note {
			t.Fatalf("notes: %+v %v", items, err)
		}
		cfg, err := config.Load(l.Layout().ConfigPath())
		if err != nil || cfg.Sync.Grants[0].Note != note {
			t.Fatalf("persisted note: %+v %v", cfg, err)
		}
	}
	w = call(s, "GET", Prefix+"/settings/credentials", "", "")
	if strings.Contains(w.Body.String(), created.Token) || strings.Contains(w.Body.String(), "digest") {
		t.Fatal("list leaked secret")
	}
	body, _ := json.Marshal(map[string]string{"note": strings.Repeat("a", 201)})
	if w = call(s, "PUT", Prefix+"/settings/credentials/"+created.ID, "", string(body)); w.Code < 400 {
		t.Fatal("oversized note accepted")
	}
	if _, err := l.RevokeSync(t.Context(), created.ID); err != nil {
		t.Fatal(err)
	}
	w = call(s, "PUT", Prefix+"/settings/credentials/"+created.ID, "", `{"note":"old"}`)
	if w.Code != 404 {
		t.Fatalf("revoked note: %d", w.Code)
	}
	if ok, _ := l.CheckSyncCredential(t.Context(), created.Token); ok {
		t.Fatal("rename restored revoked grant")
	}
}
