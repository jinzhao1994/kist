package media

import (
	"bytes"
	"testing"

	"github.com/jinzhao1994/kist/internal/extract"
)

func TestContentClassification(t *testing.T) {
	for _, tc := range []struct {
		name       string
		data       []byte
		mime, kind string
	}{
		{"mov", []byte("\x00\x00\x00\x14ftypqt  0000qt  "), "video/quicktime", "video"},
		{"pdf", []byte("%PDF-1.7\n"), "application/pdf", "document"},
		{"jpeg", []byte{255, 216, 255, 0}, "image/jpeg", "image"},
		{"heic", []byte("\x00\x00\x00\x18ftypheic0000"), "image/heic", "image"},
		{"avif", []byte("\x00\x00\x00\x18ftypavif0000"), "image/avif", "image"},
		{"text", []byte("hello"), "text/plain; charset=utf-8", "document"},
		{"empty", nil, "application/octet-stream", "other"},
		{"unknown", []byte{0, 1, 2, 3}, "application/octet-stream", "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mime, err := extract.DetectMedia(bytes.NewReader(tc.data))
			if err != nil || mime != tc.mime {
				t.Fatalf("mime=%s %v", mime, err)
			}
			rows, err := New().Extract(t.Context(), bytes.NewReader(tc.data), mime)
			if err != nil {
				t.Fatal(err)
			}
			values := map[string]string{}
			for _, row := range rows {
				values[row.Key] = row.Value
			}
			if values["media:kind"] != tc.kind || values["media:mime"] != tc.mime {
				t.Fatal(values)
			}
		})
	}
}
