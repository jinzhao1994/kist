package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"math/rand/v2"

	"github.com/disintegration/imaging"
	"github.com/jinzhao1994/kist/internal/derive"
)

type UprightChoice struct {
	Index      int
	Confidence float64
	Evidence   string
	Provider   string
	Model      string
}

type OrientationResult struct {
	Degrees    int
	Confidence float64
	Method     string
	Provider   string
	Model      string
}

// ParseUprightChoice rejects malformed output instead of saving it as a success.
func ParseUprightChoice(text string) (UprightChoice, error) {
	var raw struct {
		Decision   string   `json:"decision"`
		Confidence *float64 `json:"confidence"`
		Evidence   string   `json:"evidence"`
	}
	d := json.NewDecoder(bytes.NewBufferString(text))
	d.DisallowUnknownFields()
	err := d.Decode(&raw)
	var extra any
	if err != nil || d.Decode(&extra) != io.EOF || raw.Confidence == nil || math.IsNaN(*raw.Confidence) || *raw.Confidence < 0 || *raw.Confidence > 1 || (raw.Evidence != "text" && raw.Evidence != "scene" && raw.Evidence != "none") {
		return UprightChoice{}, errors.New("invalid image orientation response")
	}
	index := -1
	switch raw.Decision {
	case "A", "B", "C", "D":
		index = int(raw.Decision[0] - 'A')
	case "uncertain":
	default:
		return UprightChoice{}, errors.New("invalid image orientation choice")
	}
	return UprightChoice{Index: index, Confidence: *raw.Confidence, Evidence: raw.Evidence}, nil
}

func encodePNG(img image.Image) (Image, error) {
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return Image{}, err
	}
	if out.Len() > MaxImageBytes {
		return Image{}, errors.New("normalized AI image exceeds 20 MiB")
	}
	return Image{MIME: "image/png", Data: out.Bytes()}, nil
}

// RotateImage preserves the full normalized pixel grid used for OCR and descriptions.
func RotateImage(input Image, degrees int) (Image, error) {
	if degrees == 0 {
		return input, nil
	}
	if degrees != 90 && degrees != 180 && degrees != 270 {
		return Image{}, errors.New("invalid image rotation")
	}
	img, _, err := image.Decode(bytes.NewReader(input.Data))
	if err != nil {
		return Image{}, err
	}
	return encodeFullImage(derive.RotateClockwise(img, degrees))
}

// Full-resolution photographs can exceed the wire limit as PNG. Retain their
// pixel dimensions and use high-quality JPEG only when lossless encoding is too large.
func encodeFullImage(img image.Image) (Image, error) {
	result, err := encodePNG(img)
	if err == nil {
		return result, nil
	}
	var out bytes.Buffer
	if err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 95}); err != nil {
		return Image{}, err
	}
	if out.Len() > MaxImageBytes {
		return Image{}, errors.New("normalized AI image exceeds 20 MiB")
	}
	return Image{MIME: "image/jpeg", Data: out.Bytes()}, nil
}

func chooseOrientation(ctx context.Context, p Provider, model string, img image.Image, overview bool) (OrientationResult, string, error) {
	angles := []int{0, 90, 180, 270}
	order := rand.Perm(4)
	images := make([]Image, 4)
	for i, n := range order {
		if err := ctx.Err(); err != nil {
			return OrientationResult{}, "", err
		}
		var err error
		images[i], err = encodePNG(derive.RotateClockwise(img, angles[n]))
		if err != nil {
			return OrientationResult{}, "", err
		}
	}
	c, err := p.ChooseUpright(ctx, model, images, overview)
	if err != nil {
		return OrientationResult{}, "", err
	}
	if c.Index < -1 || c.Index > 3 || math.IsNaN(c.Confidence) || c.Confidence < 0 || c.Confidence > 1 || (c.Evidence != "text" && c.Evidence != "scene" && c.Evidence != "none") {
		return OrientationResult{}, "", errors.New("invalid image orientation response")
	}
	result := OrientationResult{Confidence: c.Confidence, Method: "uncertain", Provider: c.Provider, Model: c.Model}
	if c.Index >= 0 && c.Confidence >= .9 && c.Evidence != "none" {
		result.Degrees = angles[order[c.Index]]
		result.Method = "overview"
	}
	return result, c.Evidence, nil
}

// DetectOrientation uses full-scene evidence first; crop votes never override it.
func DetectOrientation(ctx context.Context, p Provider, model string, input Image) (OrientationResult, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil {
		return OrientationResult{}, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || uint64(cfg.Width)*uint64(cfg.Height) > 64_000_000 {
		return OrientationResult{}, errors.New("AI image exceeds pixel limit")
	}
	img, _, err := image.Decode(bytes.NewReader(input.Data))
	if err != nil {
		return OrientationResult{}, err
	}
	result, evidence, err := chooseOrientation(ctx, p, model, imaging.Fit(img, 1024, 1024, imaging.Lanczos), true)
	if err != nil || result.Method != "uncertain" || evidence != "text" || (cfg.Width <= 1024 && cfg.Height <= 1024) {
		return result, err
	}
	w, h := cfg.Width, cfg.Height
	sizeW, sizeH := min(w, 768), min(h, 768)
	centers := []image.Point{{w / 4, h / 4}, {3 * w / 4, h / 4}, {w / 4, 3 * h / 4}, {3 * w / 4, 3 * h / 4}, {w / 2, h / 2}}
	var rects []image.Rectangle
	angle, votes, confidence := -1, 0, 1.0
	for i, center := range centers {
		x, y := max(0, min(center.X-sizeW/2, w-sizeW)), max(0, min(center.Y-sizeH/2, h-sizeH))
		r := image.Rect(x, y, x+sizeW, y+sizeH)
		duplicate := false
		for _, previous := range rects {
			overlap := r.Intersect(previous)
			if overlap.Dx()*overlap.Dy()*5 >= sizeW*sizeH*4 {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		rects = append(rects, r)
		local, _, err := chooseOrientation(ctx, p, model, imaging.Crop(img, r), false)
		if err != nil {
			return OrientationResult{}, err
		}
		if local.Method == "uncertain" {
			continue
		}
		if angle >= 0 && angle != local.Degrees {
			return result, nil
		}
		angle = local.Degrees
		confidence = min(confidence, local.Confidence)
		if i < 4 {
			votes++
		}
	}
	if votes >= 2 {
		result.Degrees = angle
		result.Confidence = confidence
		result.Method = "regions"
	}
	return result, nil
}
