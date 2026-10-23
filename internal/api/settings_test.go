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
	if len(changed.RestartRequired) != 1 || changed.RestartRequired[0] != "server.max_upload_files" {
		t.Fatalf("restart=%v", changed.RestartRequired)
	}
	info := call(s, "GET", Prefix+"/info", "", "")
	if !strings.Contains(info.Body.String(), `"primary_color":"green"`) {
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
