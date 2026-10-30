package library

import (
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
)

func TestLocationSettingsResetPreservesActivationBoundary(t *testing.T) {
	l := openTestLibrary(t)
	s, err := l.LocationSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	s.Automatic, s.PauseOnError = false, false
	s.Endpoint, s.RequestIntervalSeconds = "https://example.test/reverse", 9
	s, err = l.SaveLocationSettings(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	old := ocrFile(t, l)
	s.Reset = []string{"automatic", "endpoint", "request_interval_seconds", "pause_on_error"}
	// Client-supplied defaults are never authoritative.
	s.Defaults = &LocationSettingDefaults{Endpoint: "https://invalid.test"}
	s, err = l.SaveLocationSettings(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	d := config.DefaultGeocoding()
	if !s.Automatic || !s.PauseOnError || s.Endpoint != d.Endpoint || s.RequestIntervalSeconds != d.RequestIntervalSeconds {
		t.Fatal(s)
	}
	reloaded, err := l.LocationSettings(t.Context())
	if err != nil || !reflect.DeepEqual(s, reloaded) {
		t.Fatal("reset response differs from persisted settings", s, reloaded, err)
	}
	fresh := ocrFile(t, l)
	q := l.LocationQueue()
	if err = q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "missing")
	requireOCRState(t, q, fresh, "ready")
	raw, err := os.ReadFile(l.layout.ConfigPath())
	if err != nil || strings.Contains(string(raw), "example.test") {
		t.Fatal("custom endpoint was retained", err)
	}
	for _, key := range []string{"endpoint", "request_interval_seconds", "pause_on_error"} {
		if strings.Contains(string(raw), key+" =") {
			t.Fatalf("reset retained explicit GPS option %s", key)
		}
	}
}

func TestSettingsPresenceAndPartialSave(t *testing.T) {
	l := openTestLibrary(t)
	a, err := l.OCRSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Explicit) != 0 {
		t.Fatal("fresh AI settings must follow defaults", a.Explicit)
	}
	stale := a
	a.Changed = []string{"auto_ocr"}
	// Saving false explicitly is meaningful even though the default is false.
	a, err = l.SaveOCRSettings(t.Context(), a)
	if err != nil || !slices.Contains(a.Explicit, "auto_ocr") {
		t.Fatal(a, err)
	}
	if a.Revision == stale.Revision {
		t.Fatal("presence-only edit did not change revision")
	}
	if _, err = l.SaveOCRSettings(t.Context(), stale); !errors.Is(err, config.ErrChanged) {
		t.Fatal("stale presence accepted", err)
	}
	a.Changed = []string{"proxy_url"}
	a.ProxyURL = "http://127.0.0.1:7890"
	a.AutoDesc = true // An unselected field must be ignored, not enabled.
	a, err = l.SaveOCRSettings(t.Context(), a)
	if err != nil || a.AutoDesc || !reflect.DeepEqual(a.Explicit, []string{"auto_ocr", "proxy_url"}) {
		t.Fatal(a, err)
	}
	a.Changed, a.Reset = []string{}, []string{"auto_ocr"}
	a, err = l.SaveOCRSettings(t.Context(), a)
	if err != nil || slices.Contains(a.Explicit, "auto_ocr") || !slices.Contains(a.Explicit, "proxy_url") {
		t.Fatal(a, err)
	}

	g, err := l.LocationSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Explicit) != 0 {
		t.Fatal("fresh GPS settings must follow defaults", g.Explicit)
	}
	g.Changed = []string{"request_interval_seconds"}
	g, err = l.SaveLocationSettings(t.Context(), g)
	if err != nil || !reflect.DeepEqual(g.Explicit, []string{"request_interval_seconds"}) {
		t.Fatal(g, err)
	}
	g.Changed, g.Reset = []string{}, []string{"request_interval_seconds"}
	g, err = l.SaveLocationSettings(t.Context(), g)
	if err != nil || len(g.Explicit) != 0 {
		t.Fatal(g, err)
	}
}

func TestCombinedSettingsAreAtomicAndKeepActivationBoundaries(t *testing.T) {
	l := openTestLibrary(t)
	setOCR(t, l, false)
	old := ocrFile(t, l)
	snapshot, _ := l.InstanceSettings(t.Context())
	a, _ := l.OCRSettings(t.Context())
	g, _ := l.LocationSettings(t.Context())
	a.AutoOCR, a.Model, a.Changed = true, "", []string{"auto_ocr", "model"}
	g.RequestIntervalSeconds, g.Changed = 7, []string{"request_interval_seconds"}
	before, _ := os.ReadFile(l.layout.ConfigPath())
	changes := map[string]any{"appearance.primary_color": "green"}
	if err := l.UpdateCombinedSettings(t.Context(), snapshot.Revision, changes, &a, &g); err == nil {
		t.Fatal("invalid combined draft accepted")
	}
	after, _ := os.ReadFile(l.layout.ConfigPath())
	if string(before) != string(after) {
		t.Fatal("failed combined save changed configuration")
	}
	a.Model = "test-model"
	if err := l.UpdateCombinedSettings(t.Context(), snapshot.Revision, changes, &a, &g); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil || cfg.Appearance.PrimaryColor != "green" || cfg.Geocoding.RequestIntervalSeconds != 7 || !cfg.AIOptions.AutoOCR {
		t.Fatal(cfg, err)
	}
	p := &fakeOCR{}
	q := queueFor(t, l, p)
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "missing")
	if p.calls != 0 {
		t.Fatal("combined save processed an old image")
	}
	if err := l.UpdateCombinedSettings(t.Context(), snapshot.Revision, changes, &a, &g); !errors.Is(err, config.ErrChanged) {
		t.Fatal("stale combined draft accepted", err)
	}
}

func TestAISettingsResetRemovesOverrides(t *testing.T) {
	l := openTestLibrary(t)
	s, err := l.OCRSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	s.Model, s.ProxyURL = "test-model", "http://127.0.0.1:7890"
	s.AutoOCR, s.AutoDesc, s.AutoFacts, s.AutoOrientation = true, true, true, true
	s, err = l.SaveOCRSettings(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	s.Reset = []string{"model", "proxy_url", "auto_ocr", "auto_desc", "auto_facts", "auto_orientation"}
	s, err = l.SaveOCRSettings(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != "" || s.ProxyURL != "" || s.AutoOCR || s.AutoDesc || s.AutoFacts || s.AutoOrientation {
		t.Fatal(s)
	}
	reloaded, err := l.OCRSettings(t.Context())
	if err != nil || !reflect.DeepEqual(s, reloaded) {
		t.Fatal("reset response differs from persisted settings", s, reloaded, err)
	}
	raw, err := os.ReadFile(l.layout.ConfigPath())
	if err != nil || strings.Contains(string(raw), "test-model") || strings.Contains(string(raw), "127.0.0.1:7890") {
		t.Fatal("custom AI values were retained", err)
	}
	for _, key := range []string{"model", "proxy_url", "auto_ocr", "auto_desc", "auto_facts", "auto_orientation"} {
		if strings.Contains(string(raw), key+" =") {
			t.Fatalf("reset retained explicit AI option %s", key)
		}
	}
}

func TestSettingsRejectPrivateResetKeys(t *testing.T) {
	l := openTestLibrary(t)
	before, err := os.ReadFile(l.layout.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	s, _ := l.LocationSettings(t.Context())
	s.Reset = []string{"after_op"}
	if _, err := l.SaveLocationSettings(t.Context(), s); err == nil {
		t.Fatal("private activation boundary accepted")
	}
	a, _ := l.OCRSettings(t.Context())
	a.Reset = []string{"after_op"}
	if _, err := l.SaveOCRSettings(t.Context(), a); err == nil {
		t.Fatal("private activation boundary accepted")
	}
	after, err := os.ReadFile(l.layout.ConfigPath())
	if err != nil || string(before) != string(after) {
		t.Fatal("rejected reset changed configuration", err)
	}
}
