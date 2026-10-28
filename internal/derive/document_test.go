package derive

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDocumentPDFPageAndByteLimits(t *testing.T) {
	for _, tool := range []string{"pdfinfo", "pdftocairo"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("poppler not installed")
		}
	}
	r := Renderer{}
	doc, err := r.Document(t.Context(), "testdata/two-pages.pdf", "", 2, 20<<20)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Pages != 2 || doc.TotalPages != 2 || !bytes.HasPrefix(doc.Data, []byte("%PDF-")) {
		t.Fatal(doc.Pages, doc.TotalPages)
	}
	path := filepath.Join(t.TempDir(), "selected.pdf")
	if err := os.WriteFile(path, doc.Data, 0600); err != nil {
		t.Fatal(err)
	}
	selected, err := r.Document(t.Context(), path, "", 10, 20<<20)
	if err != nil || selected.Pages != 2 || selected.TotalPages != 2 {
		t.Fatal(selected.Pages, selected.TotalPages, err)
	}
	// Reject an over-limit document before rendering instead of sending an excerpt.
	noRenderer := Renderer{PDFToCairo: filepath.Join(t.TempDir(), "absent")}
	if rejected, err := noRenderer.Document(t.Context(), "testdata/two-pages.pdf", "", 1, 20<<20); Code(err) != "limit_exceeded" || len(rejected.Data) != 0 {
		t.Fatal("over-limit document was not rejected before rendering", err)
	}
	if _, err := r.Document(t.Context(), "testdata/two-pages.pdf", "", 10, 10); Code(err) != "limit_exceeded" {
		t.Fatal(err)
	}
	if _, err := r.Document(t.Context(), "testdata/two-pages.pdf", "", 0, 100); err == nil {
		t.Fatal("accepted invalid limits")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.Document(ctx, path, "", 10, 20<<20); err == nil {
		t.Fatal("ignored cancellation")
	}
	if err := os.WriteFile(path, []byte("%PDF-1.7\nbroken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Document(t.Context(), path, "", 10, 20<<20); err == nil {
		t.Fatal("accepted invalid PDF")
	}
}

func TestDocumentMissingConverters(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	if _, err := (Renderer{PDFInfo: missing}).Document(t.Context(), "testdata/two-pages.pdf", "", 10, 20<<20); Code(err) != "dependency_missing" {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, recipe string }{{"two-pages.doc", OfficePDF}, {"two-pages.docx", OfficePDF}, {"two-slides.ppt", SlidesPDF}, {"two-slides.pptx", SlidesPDF}} {
		if _, err := (Renderer{LibreOffice: missing}).Document(t.Context(), "testdata/"+tc.name, tc.recipe, 10, 20<<20); Code(err) != "dependency_missing" {
			t.Fatal(tc.name, err)
		}
	}
}

func TestDocumentPDFEncryption(t *testing.T) {
	for _, name := range []string{"pdfinfo", "pdftocairo"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip("poppler not installed")
		}
	}
	for _, tc := range []struct {
		name     string
		readable bool
	}{{"permissions.pdf", true}, {"password.pdf", false}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("testdata", tc.name)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := (Renderer{}).Document(t.Context(), path, "", 2, 20<<20)
			if tc.readable {
				if err != nil || doc.Pages != 2 || doc.TotalPages != 2 {
					t.Fatalf("readable protected PDF: pages=%d total=%d err=%v", doc.Pages, doc.TotalPages, err)
				}
				selected := filepath.Join(t.TempDir(), "selected.pdf")
				if err := os.WriteFile(selected, doc.Data, 0600); err != nil {
					t.Fatal(err)
				}
				info, err := exec.CommandContext(t.Context(), "pdfinfo", selected).CombinedOutput()
				if err != nil || !bytes.Contains(info, []byte("Encrypted:       no")) {
					t.Fatalf("output is not readable without encryption: %s: %v", info, err)
				}
			} else if err == nil || len(doc.Data) != 0 {
				t.Fatal("password-required PDF produced output", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("original PDF changed", err)
			}
		})
	}
}

func TestRealOfficeDescriptionPages(t *testing.T) {
	for _, name := range []string{"pdfinfo", "pdftocairo"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip("poppler not installed")
		}
	}
	if _, err := exec.LookPath("libreoffice"); err != nil {
		if _, err := exec.LookPath("soffice"); err != nil {
			t.Skip("LibreOffice not installed")
		}
	}
	for _, tc := range []struct{ name, recipe string }{{"two-pages.doc", OfficePDF}, {"two-pages.docx", OfficePDF}, {"two-slides.ppt", SlidesPDF}, {"two-slides.pptx", SlidesPDF}} {
		doc, err := (Renderer{}).Document(t.Context(), "testdata/"+tc.name, tc.recipe, 2, 20<<20)
		if err != nil || doc.Pages != 2 || doc.TotalPages != 2 {
			t.Fatal(tc.name, doc.Pages, doc.TotalPages, err)
		}
	}
}
