package derive

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPDFFailures(t *testing.T) {
	input := "testdata/two-pages.pdf"
	output := filepath.Join(t.TempDir(), "output")
	r := Renderer{PDFToPPM: filepath.Join(t.TempDir(), "absent")}
	if _, err := r.Render(t.Context(), input, output, Thumbnail); Code(err) != "dependency_missing" {
		t.Fatal(err)
	}
	if _, err := r.Render(t.Context(), input, output, Video); Code(err) != "unsupported" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.Render(ctx, input, output, Preview); Code(err) != "cancelled" {
		t.Fatal(err)
	}
	huge := filepath.Join(t.TempDir(), "huge")
	f, err := os.Create(huge)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("%PDF-1.4\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxImageBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err = r.Render(t.Context(), huge, output, Thumbnail); Code(err) != "limit_exceeded" {
		t.Fatal(err)
	}
	var b bytes.Buffer
	bounded := &boundedPDFOutput{writer: &b, remaining: 2}
	if _, err = bounded.Write([]byte("abc")); Code(err) != "limit_exceeded" || b.Len() != 0 {
		t.Fatal(err, b.Len())
	}
}

func TestRealPDFFirstPage(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("poppler not installed")
	}
	// Blob paths have no file extension.
	input := filepath.Join(t.TempDir(), "blob")
	data, err := os.ReadFile("testdata/two-pages.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		recipe string
		size   int
	}{{Thumbnail, 512}, {Preview, 2048}} {
		t.Run(tc.recipe, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "output")
			result, err := (Renderer{}).Render(t.Context(), input, output, tc.recipe)
			if err != nil {
				t.Fatal(err)
			}
			if result.Width != tc.size/2 || result.Height != tc.size || result.Engine != "poppler" || result.Bytes == 0 {
				t.Fatal(result)
			}
			f, err := os.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			img, err := jpeg.Decode(f)
			if err != nil {
				t.Fatal(err)
			}
			// Verify the first page's pixels and all four edges, not just file existence.
			for _, xy := range [][2]int{{2, 2}, {result.Width - 3, 2}, {2, result.Height - 3}, {result.Width - 3, result.Height - 3}} {
				red, green, blue, _ := img.At(xy[0], xy[1]).RGBA()
				if red < 60000 || green > 5000 || blue > 5000 {
					t.Fatal("wrong page or cropped content", xy, red, green, blue)
				}
			}
		})
	}
	if err = os.WriteFile(input, []byte("%PDF-1.4\nbroken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = (Renderer{}).Render(t.Context(), input, filepath.Join(t.TempDir(), "bad"), Thumbnail); Code(err) != "decode_failed" {
		t.Fatal(err)
	}
}
