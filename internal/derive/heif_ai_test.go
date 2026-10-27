package derive

import (
	"bytes"
	"context"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHEIFJPEGRejectsInvalidAndMissingDecoder(t *testing.T) {
	r := Renderer{HeifInfo: filepath.Join(t.TempDir(), "absent")}
	if _, err := r.HEIFJPEG(t.Context(), "testdata/pattern.heic", 20<<20); Code(err) != "dependency_missing" {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "sequence.heic")
	if err := os.WriteFile(p, []byte("\x00\x00\x00\x18ftypmsf1\x00\x00\x00\x00heicmsf1"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.HEIFJPEG(t.Context(), p, 20<<20); Code(err) != "unsupported" {
		t.Fatal(err)
	}
}

func TestRealHEIFJPEG(t *testing.T) {
	if _, err := exec.LookPath("heif-info"); err != nil {
		t.Skip("libheif tools not installed")
	}
	for _, tc := range []struct {
		name string
		w, h int
	}{{"pattern.heic", 128, 96}, {"tiled.heic", 3072, 2048}} {
		t.Run(tc.name, func(t *testing.T) {
			input := filepath.Join("testdata", tc.name)
			original, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			data, err := (Renderer{}).HEIFJPEG(t.Context(), input, 20<<20)
			if err != nil {
				t.Fatal(err)
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil || format != "jpeg" || cfg.Width != tc.w || cfg.Height != tc.h {
				t.Fatal(cfg, format, err)
			}
			after, _ := os.ReadFile(input)
			if !bytes.Equal(original, after) {
				t.Fatal("original changed")
			}
		})
	}
	if _, err := (Renderer{}).HEIFJPEG(t.Context(), "testdata/pattern.heic", 1); Code(err) != "limit_exceeded" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (Renderer{}).HEIFJPEG(ctx, "testdata/pattern.heic", 20<<20); Code(err) != "cancelled" {
		t.Fatal(err)
	}
}
