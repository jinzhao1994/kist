package derive

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOfficeValidation(t *testing.T) {
	missing := Renderer{LibreOffice: filepath.Join(t.TempDir(), "absent")}
	for _, name := range []string{"two-pages.docx", "two-pages.doc"} {
		_, err := missing.Render(t.Context(), "testdata/"+name, filepath.Join(t.TempDir(), "output"), OfficePDF)
		if Code(err) != "dependency_missing" {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := missing.Render(t.Context(), "testdata/two-pages.pdf", filepath.Join(t.TempDir(), "output"), OfficePDF); Code(err) != "unsupported" {
		t.Fatal(err)
	}
	large := filepath.Join(t.TempDir(), "large")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate(maxOfficeBytes + 1)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := officeFormat(t.Context(), large); Code(err) != "limit_exceeded" {
		t.Fatal(err)
	}
	for _, tc := range []struct{ kind, mode, want string }{
		{"image", "External", "unsupported"}, {"attachedTemplate", "External", "unsupported"}, {"hyperlink", "External", ""}, {"image", "Internal", ""},
	} {
		err := checkOfficeRelationships(t.Context(), strings.NewReader(`<Relationships><Relationship Type="http://example/`+tc.kind+`" TargetMode="`+tc.mode+`" Target="https://example.invalid/"/></Relationships>`))
		if tc.want == "" {
			if err != nil {
				t.Fatal(err)
			}
		} else if Code(err) != tc.want {
			t.Fatal(err)
		}
	}
	// A renamed ZIP is not a Word document, even if it contains a content type file.
	path := filepath.Join(t.TempDir(), "other.docx")
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	w, _ := z.Create("[Content_Types].xml")
	_, _ = w.Write([]byte("<Types/>"))
	z.Close()
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := officeFormat(t.Context(), path); Code(err) != "unsupported" {
		t.Fatal(err)
	}
}

func TestOfficeCancellationKillsChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process group fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "converter")
	marker := filepath.Join(dir, "leaked-child")
	// The child inherits stdout; killing only the launcher would leave it alive.
	if err := os.WriteFile(script, []byte("#!/bin/sh\n(sleep 1; touch '"+marker+"') &\nwait\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err := (Renderer{LibreOffice: script}).Render(ctx, "testdata/two-pages.docx", filepath.Join(dir, "output"), OfficePDF)
	if Code(err) != "timeout" {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("converter child survived cancellation", err)
	}
	entries, _ := filepath.Glob(filepath.Join(dir, ".render-office-*"))
	if len(entries) != 0 {
		t.Fatal("scratch files survived", entries)
	}
}

func TestRealOfficePreview(t *testing.T) {
	if _, err := exec.LookPath("libreoffice"); err != nil {
		if _, err := exec.LookPath("soffice"); err != nil {
			t.Skip("LibreOffice not installed")
		}
	}
	for _, name := range []string{"two-pages.docx", "two-pages.doc"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "blob")
			data, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(input, data, 0600); err != nil {
				t.Fatal(err)
			}
			pdf := filepath.Join(dir, "output.pdf")
			result, err := (Renderer{}).Render(t.Context(), input, pdf, OfficePDF)
			if err != nil || result.Bytes == 0 || result.Engine != "libreoffice" {
				t.Fatal(result, err)
			}
			info, err := exec.Command("pdfinfo", pdf).CombinedOutput()
			if err != nil || !strings.Contains(string(info), "Pages:           2") {
				t.Fatal(string(info), err)
			}
			text, err := exec.Command("pdftotext", pdf, "-").Output()
			if err != nil || !bytes.Contains(text, []byte("Kist Word first page")) || !bytes.Contains(text, []byte("Kist Word second page")) {
				t.Fatal(string(text), err)
			}
			thumb := filepath.Join(dir, "thumb.jpg")
			result, err = (Renderer{}).Render(t.Context(), input, thumb, OfficeThumbnail)
			if err != nil || result.Height != 512 || result.Width <= 0 || result.Width >= 512 {
				t.Fatal(result, err)
			}
			after, _ := os.ReadFile(input)
			if !bytes.Equal(data, after) {
				t.Fatal("original was modified")
			}
		})
	}
}
