package derive

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// isHEIF reads brands rather than filenames or user-supplied MIME headers.
func isHEIF(f *os.File) bool {
	var head [4096]byte
	n, _ := f.ReadAt(head[:], 0)
	if n < 16 || string(head[4:8]) != "ftyp" {
		return false
	}
	size := int(binary.BigEndian.Uint32(head[:4]))
	if size > n {
		size = n
	}
	for i := 8; i+4 <= size; i += 4 {
		if i == 12 {
			continue
		}
		switch string(head[i : i+4]) {
		case "heic", "heix", "hevc", "hevx", "mif1", "msf1", "avif", "avis":
			return true
		}
	}
	return false
}

var heifDimensions = regexp.MustCompile(`(?m)^image: ([0-9]+)x([0-9]+) `)

func (r Renderer) heif(ctx context.Context, input, output string, spec Recipe) (Result, error) {
	var result Result
	if spec.Name == Video {
		return result, fail("unsupported", "HEIF is a still-image source")
	}
	infoPath, e := tool(r.HeifInfo, "heif-info")
	if e != nil {
		return result, e
	}
	decoder, e := tool(r.HeifDec, "heif-dec")
	if e != nil && r.HeifDec == "" {
		decoder, e = tool("", "heif-convert")
	}
	if e != nil {
		return result, e
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	info := &limitedBuffer{limit: 64 << 10}
	if e = run(ctx, infoPath, []string{input}, info); e != nil {
		return result, e
	}
	dimensions := heifDimensions.FindAllStringSubmatch(info.String(), -1)
	// heif-dec exports all top-level images. Refuse ambiguous collections rather
	// than accidentally selecting a thumbnail or non-primary image as the photo.
	if len(dimensions) != 1 {
		return result, fail("unsupported", "HEIF collections require explicit primary-image selection")
	}
	w, _ := strconv.Atoi(dimensions[0][1])
	h, _ := strconv.Atoi(dimensions[0][2])
	if !validDimensions(w, h) {
		return result, fail("limit_exceeded", "HEIF dimensions exceed the pixel limit")
	}
	stat, e := os.Stat(input)
	if e != nil {
		return result, e
	}
	if stat.Size() > MaxImageBytes {
		return result, fail("limit_exceeded", "compressed HEIF exceeds the input limit")
	}
	dir, e := os.MkdirTemp(filepath.Dir(output), ".heif-")
	if e != nil {
		return result, e
	}
	defer os.RemoveAll(dir)
	decoded := filepath.Join(dir, "decoded.png")
	if e = run(ctx, decoder, []string{input, decoded}, io.Discard); e != nil {
		return result, e
	}
	// libheif applies HEIF transforms and assembles tiles. The native path checks
	// decoded dimensions again and performs the bounded resize and JPEG encoding.
	result, e = r.Render(ctx, decoded, output, spec.Name)
	result.Engine = "libheif"
	return result, e
}
