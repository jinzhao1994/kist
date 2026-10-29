package library

import (
	"os"
	"reflect"
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
