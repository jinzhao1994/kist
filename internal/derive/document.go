package derive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DocumentPDF is a bounded, static view of an entire document.
// Re-rendering pages excludes attachments and other non-page content.
type DocumentPDF struct {
	Data              []byte
	Pages, TotalPages int
}

// Document prepares PDF input using the same Office converter as previews.
// An empty recipe means the input is already a PDF.
func (r Renderer) Document(ctx context.Context, input, recipe string, maxPages int, maxBytes int) (DocumentPDF, error) {
	var out DocumentPDF
	if maxPages < 1 || maxBytes < 1 {
		return out, errors.New("invalid document limits")
	}
	st, err := os.Stat(input)
	if err != nil {
		return out, err
	}
	if st.Size() > maxOfficeBytes {
		return out, fail("limit_exceeded", "document exceeds 128 MiB")
	}
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	if recipe != "" {
		if recipe != OfficePDF && recipe != SlidesPDF {
			return out, fail("unsupported", "unsupported document type")
		}
		dir, err := os.MkdirTemp("", "kist-ai-document-")
		if err != nil {
			return out, err
		}
		defer os.RemoveAll(dir)
		pdf := filepath.Join(dir, "document.pdf")
		if _, err = r.Render(ctx, input, pdf, recipe); err != nil {
			return out, err
		}
		input = pdf
	}
	infoTool, err := tool(r.PDFInfo, "pdfinfo")
	if err != nil {
		return out, err
	}
	info := &limitedBuffer{limit: 64 << 10}
	if err = run(ctx, infoTool, []string{input}, info); err != nil {
		return out, err
	}
	// A PDF can be encrypted only to restrict permissions and still open without
	// a password. Let the tools decide whether it can be read and rendered;
	// password-required documents already fail the no-password pdfinfo call.
	for _, line := range strings.Split(info.String(), "\n") {
		if value, ok := strings.CutPrefix(line, "Pages:"); ok {
			out.TotalPages, _ = strconv.Atoi(strings.TrimSpace(value))
		}
	}
	if out.TotalPages < 1 {
		return DocumentPDF{}, fail("decode_failed", "PDF page count is unavailable")
	}
	if out.TotalPages > maxPages {
		return DocumentPDF{}, fail("limit_exceeded", fmt.Sprintf("document has %d pages; maximum is %d; no pages were sent", out.TotalPages, maxPages))
	}
	out.Pages = out.TotalPages
	renderer, err := tool(r.PDFToCairo, "pdftocairo")
	if err != nil {
		return DocumentPDF{}, err
	}
	var data bytes.Buffer
	bounded := &boundedPDFOutput{writer: &data, remaining: maxBytes}
	err = run(ctx, renderer, []string{"-pdf", "-f", "1", "-l", strconv.Itoa(out.Pages), input, "-"}, bounded)
	if bounded.exceeded {
		return DocumentPDF{}, fail("limit_exceeded", "complete document exceeds AI input byte limit; no pages were sent")
	}
	if err != nil {
		return DocumentPDF{}, err
	}
	if !bytes.HasPrefix(data.Bytes(), []byte("%PDF-")) {
		return DocumentPDF{}, fail("decode_failed", "document conversion produced invalid PDF")
	}
	out.Data = data.Bytes()
	return out, nil
}
