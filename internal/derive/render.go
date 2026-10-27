// Package derive produces replaceable media from immutable original bytes.
// It has no database or operation-log access. Library owns publication.
package derive

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

const (
	STLThumbnail         = "stl_thumb_512_v1"
	STLPreview           = "stl_preview_2048_v1"
	Thumbnail            = "thumb_512_v1"
	VideoThumbnail       = "video_thumb_512_v2"
	Preview              = "preview_2048_v1"
	UprightImage         = "image_upright_v1"
	Video                = "preview_1080p_v1"
	MaxPixels      int64 = 64_000_000
	MaxImageBytes  int64 = 256 << 20
	MaxOutputBytes int64 = 2 << 30
)

type Recipe struct {
	Name      string `json:"name"`
	Extension string `json:"extension"`
	MediaType string `json:"media_type"`
	Size      int    `json:"size"`
}

// Lookup is the only registry of public recipes and their output types.
func Lookup(name string) (Recipe, bool) {
	if base, angle, ok := strings.Cut(name, "_cw"); ok {
		n, err := strconv.Atoi(angle)
		if err != nil || RotatedRecipe(base, n) != name || n == 0 {
			return Recipe{}, false
		}
		spec, ok := Lookup(base)
		spec.Name = name
		return spec, ok
	}
	switch name {
	case UprightImage:
		return Recipe{name, ".png", "image/png", int(MaxPixels)}, true
	case OfficePDF, SlidesPDF:
		return Recipe{name, ".pdf", "application/pdf", 0}, true
	case GPXMap:
		return Recipe{name, ".geojson", "application/geo+json", 0}, true
	case Thumbnail, VideoThumbnail, STLThumbnail, OfficeThumbnail, SlidesThumbnail:
		return Recipe{name, ".jpg", "image/jpeg", 512}, true
	case Preview, STLPreview:
		return Recipe{name, ".jpg", "image/jpeg", 2048}, true
	case Video:
		return Recipe{name, ".mp4", "video/mp4", 1080}, true
	default:
		return Recipe{}, false
	}
}

type Failure struct {
	Code  string
	Cause error
}

func (e *Failure) Error() string      { return e.Code + ": " + e.Cause.Error() }
func (e *Failure) Unwrap() error      { return e.Cause }
func fail(code, message string) error { return &Failure{code, errors.New(message)} }
func Code(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var f *Failure
	if errors.As(err, &f) {
		return f.Code
	}
	return "generation_failed"
}

type Result struct {
	GenerationMS int64  `json:"generation_ms"`
	Bytes        int64  `json:"bytes"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Engine       string `json:"engine"`
}

// Renderer does not publish outputs. The caller provides a disposable output
// path and must discard it on any error, including cancellation.
type Renderer struct {
	// Empty paths use PATH at each invocation, so installing a tool takes effect
	// without restarting serve. Explicit paths also make missing-tool tests portable.
	FFmpeg      string
	FFprobe     string
	HeifDec     string
	HeifInfo    string
	PDFToPPM    string
	LibreOffice string
}

func (r Renderer) Render(ctx context.Context, input, output, recipe string) (result Result, err error) {
	started := time.Now()
	spec, ok := Lookup(recipe)
	if !ok {
		return result, fail("unsupported", "unknown media recipe")
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	f, err := os.Open(input)
	if err != nil {
		return result, err
	}
	defer f.Close()
	cfg, format, decodeErr := image.DecodeConfig(io.LimitReader(contextReader{ctx, f}, MaxImageBytes+1))
	if recipe == OfficePDF || recipe == OfficeThumbnail || recipe == SlidesPDF || recipe == SlidesThumbnail {
		result, err = r.office(ctx, input, output, spec)
		if err != nil {
			return result, err
		}
	} else if recipe == GPXMap {
		result, err = renderGPX(ctx, input, output)
		if err != nil {
			return result, err
		}
	} else if recipe == STLThumbnail || recipe == STLPreview {
		result, err = r.stl(ctx, input, output, spec)
		if err != nil {
			return result, err
		}
	} else if decodeErr == nil {
		if recipe == Video {
			return result, fail("unsupported", "video recipe requires a video")
		}
		if !validDimensions(cfg.Width, cfg.Height) {
			return result, fail("limit_exceeded", "image dimensions exceed the pixel limit")
		}
		stat, e := f.Stat()
		if e != nil {
			return result, e
		}
		if stat.Size() > MaxImageBytes {
			return result, fail("limit_exceeded", "compressed image exceeds the input limit")
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return result, err
		}
		o := orientation(ctx, f, format)
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return result, err
		}
		img, e := imaging.Decode(contextReader{ctx, f})
		if e != nil {
			return result, &Failure{"decode_failed", e}
		}
		if err = ctx.Err(); err != nil {
			return result, err
		}
		img = orient(img, o)
		if _, angle, ok := strings.Cut(recipe, "_cw"); ok {
			n, _ := strconv.Atoi(angle)
			img = RotateClockwise(img, n)
		}
		if img.Bounds().Dx() > spec.Size || img.Bounds().Dy() > spec.Size {
			img = imaging.Fit(img, spec.Size, spec.Size, imaging.Lanczos)
		}
		if err = ctx.Err(); err != nil {
			return result, err
		}
		// JPEG has no alpha. A white background avoids silently turning transparent
		// scans and diagrams into black rectangles.
		canvas := image.NewRGBA(img.Bounds())
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(canvas, canvas.Bounds(), img, img.Bounds().Min, draw.Over)
		dst, e := os.Create(output)
		if e != nil {
			return result, e
		}
		if spec.MediaType == "image/png" {
			e = png.Encode(dst, img)
		} else {
			e = jpeg.Encode(dst, canvas, &jpeg.Options{Quality: 85})
		}
		if e == nil {
			e = dst.Sync()
		}
		closeErr := dst.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return result, e
		}
		result.Width, result.Height, result.Engine = img.Bounds().Dx(), img.Bounds().Dy(), "go-imaging-1.6.2"
	} else {
		if !errors.Is(decodeErr, image.ErrFormat) {
			return result, &Failure{"decode_failed", decodeErr}
		}
		if isPDF(f) {
			result, err = r.pdf(ctx, input, output, spec)
		} else if isHEIF(f) {
			result, err = r.heif(ctx, input, output, spec)
		} else {
			result, err = r.external(ctx, input, output, spec)
		}
		if err != nil {
			return result, err
		}
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	stat, err := os.Stat(output)
	if err != nil {
		return result, err
	}
	if stat.Size() == 0 || stat.Size() > MaxOutputBytes {
		return result, fail("limit_exceeded", "empty or oversized derived output")
	}
	result.Bytes, result.GenerationMS = stat.Size(), time.Since(started).Milliseconds()
	return result, nil
}

func validDimensions(w, h int) bool {
	return w > 0 && h > 0 && int64(w) <= MaxPixels/int64(h)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (s Recipe) Filename() string { return s.Name + s.Extension }
