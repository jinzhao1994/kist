package derive

import (
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
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

	dir, e := os.MkdirTemp(filepath.Dir(output), ".heif-")
	if e != nil {
		return result, e
	}
	defer os.RemoveAll(dir)
	decoded := filepath.Join(dir, "decoded.png")
	if e = r.decodeHEIF(ctx, input, decoded); e != nil {
		return result, e
	}
	// libheif applies HEIF transforms and assembles tiles. The native path checks
	// decoded dimensions again and performs the bounded resize and JPEG encoding.
	result, e = r.Render(ctx, decoded, output, spec.Name)
	result.Engine = "libheif"
	return result, e
}

// decodeHEIF shares the bounded, orientation-aware decoder with previews.
func (r Renderer) decodeHEIF(ctx context.Context, input, decoded string) error {
	infoPath, e := tool(r.HeifInfo, "heif-info")
	if e != nil {
		return e
	}
	decoder, e := tool(r.HeifDec, "heif-dec")
	if e != nil && r.HeifDec == "" {
		decoder, e = tool("", "heif-convert")
	}
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	info := &limitedBuffer{limit: 64 << 10}
	if e = run(ctx, infoPath, []string{input}, info); e != nil {
		return e
	}
	dimensions := heifDimensions.FindAllStringSubmatch(info.String(), -1)
	// heif-dec exports all top-level images. Refuse ambiguous collections rather
	// than accidentally selecting a thumbnail or non-primary image as the photo.
	if len(dimensions) != 1 {
		return fail("unsupported", "HEIF collections require explicit primary-image selection")
	}
	w, _ := strconv.Atoi(dimensions[0][1])
	h, _ := strconv.Atoi(dimensions[0][2])
	if !validDimensions(w, h) {
		return fail("limit_exceeded", "HEIF dimensions exceed the pixel limit")
	}
	stat, e := os.Stat(input)
	if e != nil {
		return e
	}
	if stat.Size() > MaxImageBytes {
		return fail("limit_exceeded", "compressed HEIF exceeds the input limit")
	}
	return run(ctx, decoder, []string{input, decoded}, io.Discard)
}

// HEIFJPEG converts one still HEIF image at its full resolution for image analysis.
// No preview cache or original is changed. maxBytes bounds the returned JPEG.
func (r Renderer) HEIFJPEG(ctx context.Context, input string, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fail("limit_exceeded", "invalid image output limit")
	}
	f, err := os.Open(input)
	if err != nil {
		return nil, err
	}
	var head [4096]byte
	n, _ := f.ReadAt(head[:], 0)
	valid := isHEIF(f)
	f.Close()
	if !valid {
		return nil, fail("unsupported", "not a HEIF image")
	}
	end := int(binary.BigEndian.Uint32(head[:4]))
	if end > n {
		end = n
	}
	for i := 8; i+4 <= end; i += 4 {
		if i == 12 {
			continue
		}
		switch string(head[i : i+4]) {
		case "msf1", "hevc", "hevx", "hevm", "hevs", "avis":
			return nil, fail("unsupported", "AI extraction requires a still HEIF image")
		}
	}
	dir, err := os.MkdirTemp("", "kist-ai-heif-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	decoded := filepath.Join(dir, "decoded.png")
	if err := r.decodeHEIF(ctx, input, decoded); err != nil {
		return nil, err
	}
	pngFile, err := os.Open(decoded)
	if err != nil {
		return nil, err
	}
	defer pngFile.Close()
	cfg, _, err := image.DecodeConfig(contextReader{ctx, pngFile})
	if err != nil {
		return nil, err
	}
	if !validDimensions(cfg.Width, cfg.Height) {
		return nil, fail("limit_exceeded", "decoded HEIF exceeds the pixel limit")
	}
	stat, err := pngFile.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() > MaxImageBytes {
		return nil, fail("limit_exceeded", "decoded HEIF exceeds the input limit")
	}
	if _, err = pngFile.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(contextReader{ctx, pngFile})
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	canvas := image.NewRGBA(img.Bounds())
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), img, img.Bounds().Min, draw.Over)
	// Keep at most one byte beyond the limit to detect oversized encoded output.
	out := &limitedBuffer{limit: maxBytes + 1}
	if err = jpeg.Encode(out, canvas, &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if out.Len() > maxBytes {
		return nil, fail("limit_exceeded", "converted AI image exceeds the output limit")
	}
	return out.Bytes(), nil
}
