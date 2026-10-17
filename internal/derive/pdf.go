package derive

import (
	"bytes"
	"context"
	"image"
	"io"
	"os"
	"strconv"
	"time"
)

// PDF identification uses the bytes, never an extension or an upload header.
func isPDF(f *os.File) bool {
	var header [1024]byte
	n, _ := f.ReadAt(header[:], 0)
	return bytes.Contains(header[:n], []byte("%PDF-"))
}

// pdf rasterizes only the first page. No original PDF is served as active
// same-origin content, and the existing worker owns cancellation/publication.
func (r Renderer) pdf(ctx context.Context, input, output string, spec Recipe) (Result, error) {
	var result Result
	if spec.Name == Video {
		return result, fail("unsupported", "PDF is a document source")
	}
	st, err := os.Stat(input)
	if err != nil {
		return result, err
	}
	if st.Size() > MaxImageBytes {
		return result, fail("limit_exceeded", "PDF exceeds the input limit")
	}
	decoder, err := tool(r.PDFToPPM, "pdftoppm")
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	f, err := os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o600)
	if err != nil {
		return result, err
	}
	defer f.Close()
	// An omitted output prefix writes the single JPEG to stdout. Bound output
	// during generation as well as checking decoded dimensions before publishing.
	dst := &boundedPDFOutput{writer: f, remaining: 32 << 20}
	err = run(ctx, decoder, []string{"-f", "1", "-l", "1", "-singlefile", "-scale-to", strconv.Itoa(spec.Size), "-jpeg", "-jpegopt", "quality=85", input}, dst)
	if dst.exceeded {
		return result, fail("limit_exceeded", "PDF raster exceeds the output limit")
	}
	if err != nil {
		return result, err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return result, &Failure{"decode_failed", err}
	}
	if format != "jpeg" || !validDimensions(cfg.Width, cfg.Height) || cfg.Width > spec.Size || cfg.Height > spec.Size {
		return result, fail("limit_exceeded", "PDF raster exceeds the recipe dimensions")
	}
	if err = f.Sync(); err != nil {
		return result, err
	}
	if err = f.Close(); err != nil {
		return result, err
	}
	result.Width, result.Height, result.Engine = cfg.Width, cfg.Height, "poppler"
	return result, nil
}

type boundedPDFOutput struct {
	writer    io.Writer
	remaining int
	exceeded  bool
}

func (w *boundedPDFOutput) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		w.exceeded = true
		return 0, fail("limit_exceeded", "PDF raster exceeds the output limit")
	}
	n, err := w.writer.Write(p)
	w.remaining -= n
	return n, err
}
