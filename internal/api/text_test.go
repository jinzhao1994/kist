package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/textpreview"
	"github.com/jinzhao1994/kist/internal/wire"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestTextPreviewContentAndLimits(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "text", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	chinese, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文日志"))
	fixtures := map[string]string{"chinese.log": string(chinese), "README": "plain text", "source.unusual": "<script>alert(1)</script>", "binary.txt": "a\x00b", "large.log": strings.Repeat("entry\n", 200000)}
	for name, data := range fixtures {
		if err := os.WriteFile(filepath.Join(src, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.Import(t.Context(), src, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	for _, file := range files.Files {
		path := "/files/" + file.ID + "/text-preview"
		got := decodeBody[textpreview.Preview](t, s, path)
		if file.Name == "binary.txt" {
			if got.Supported || got.Reason != "binary" {
				t.Fatal(got)
			}
			continue
		}
		if !got.Supported {
			t.Fatal(got)
		}
		if file.Name == "chinese.log" {
			if got.Text != "中文日志" || got.Encoding != "GB18030" {
				t.Fatal(got)
			}
			detail := decodeBody[textpreview.Preview](t, s, path+"?detail=true")
			if !detail.Supported || detail.Text != "中文日志" {
				t.Fatal(detail)
			}
		} else if file.Name == "large.log" {
			if !got.Truncated || len(got.Text) > textpreview.ThumbnailBytes || strings.Count(got.Text, "\n") > 19 {
				t.Fatalf("invalid thumbnail size %d", len(got.Text))
			}
			full := decodeBody[textpreview.Preview](t, s, path+"?detail=true")
			if !full.Truncated || len(full.Text) > textpreview.DetailBytes {
				t.Fatalf("invalid detail size %d", len(full.Text))
			}
		} else if got.Text != fixtures[file.Name] {
			t.Fatal(got)
		}
		bad := call(s, "GET", Prefix+path+"?encoding=invalid", "", "")
		if bad.Code != 422 {
			t.Fatalf("invalid encoding status %d", bad.Code)
		}
	}
}
