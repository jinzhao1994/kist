package ai

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// Synthetic pixels encode upright direction without private document fixtures.
type directionProvider struct {
	calls  int
	choose func(int, []Image, bool) (UprightChoice, error)
}

func (p *directionProvider) Models(context.Context) ([]Model, error) { return nil, nil }
func (p *directionProvider) OCR(context.Context, string, Image, TextOptions) (OCRResult, error) {
	panic("unexpected OCR")
}
func (p *directionProvider) Describe(context.Context, string, Image, TextOptions) (TextResult, error) {
	panic("unexpected description")
}
func (p *directionProvider) ChooseUpright(_ context.Context, _ string, images []Image, overview bool) (UprightChoice, error) {
	p.calls++
	return p.choose(p.calls, images, overview)
}
func directionImage(t *testing.T, w, h int) Image {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(255 * x / max(w-1, 1)), G: uint8(255 * y / max(h-1, 1)), A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return Image{MIME: "image/png", Data: b.Bytes()}
}
func selectClockwise(t *testing.T, images []Image, degrees int) UprightChoice {
	t.Helper()
	if len(images) != 4 {
		t.Fatalf("got %d candidates", len(images))
	}
	// A coordinate gradient distinguishes all rotations, including square crops.
	for i, input := range images {
		img, _, err := image.Decode(bytes.NewReader(input.Data))
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		r1, g1, _, _ := img.At(b.Min.X, b.Min.Y).RGBA()
		r2, g2, _, _ := img.At(b.Max.X-1, b.Max.Y-1).RGBA()
		n := 0
		switch {
		case r1 > r2 && g1 < g2:
			n = 270
		case r1 > r2 && g1 > g2:
			n = 180
		case r1 < r2 && g1 > g2:
			n = 90
		}
		if n == degrees {
			return UprightChoice{Index: i, Confidence: .98, Evidence: "text", Provider: "test", Model: "test"}
		}
	}
	t.Fatal("missing candidate")
	return UprightChoice{}
}
func TestUprightChoiceStrictParsing(t *testing.T) {
	for _, s := range []string{`{}`, `{"decision":"90","confidence":1,"evidence":"text"}`, `{"decision":"A","confidence":1.1,"evidence":"text"}`, `{"decision":"A","confidence":0.9,"evidence":"other"}`, `{"decision":"A","confidence":0.9,"evidence":"text","extra":true}`, `{"decision":"A","confidence":0.9,"evidence":"text"} {}`} {
		if _, err := ParseUprightChoice(s); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
	got, err := ParseUprightChoice(`{"decision":"D","confidence":0.95,"evidence":"text"}`)
	if err != nil || got.Index != 3 {
		t.Fatal(got, err)
	}
}
func TestOrientationCandidateMappingAndConservativeFallback(t *testing.T) {
	input := directionImage(t, 60, 40)
	for _, angle := range []int{0, 90, 180, 270} {
		p := &directionProvider{choose: func(_ int, imgs []Image, overview bool) (UprightChoice, error) {
			if !overview {
				t.Fatal("small image cropped")
			}
			return selectClockwise(t, imgs, angle), nil
		}}
		for range 8 {
			got, err := DetectOrientation(t.Context(), p, "test", input)
			if err != nil || got.Degrees != angle || got.Method != "overview" {
				t.Fatal(got, err)
			}
		}
	}
	for _, c := range []UprightChoice{{Index: 0, Confidence: .89, Evidence: "text"}, {Index: 0, Confidence: 1, Evidence: "none"}, {Index: -1, Confidence: 1, Evidence: "text"}} {
		p := &directionProvider{choose: func(int, []Image, bool) (UprightChoice, error) { return c, nil }}
		got, err := DetectOrientation(t.Context(), p, "test", input)
		if err != nil || got.Degrees != 0 || got.Method != "uncertain" || p.calls != 1 {
			t.Fatal(got, err, p.calls)
		}
	}
}
func TestOrientationRegionsRequireAgreement(t *testing.T) {
	input := directionImage(t, 1600, 2000)
	for _, conflict := range []bool{false, true} {
		p := &directionProvider{choose: func(n int, imgs []Image, overview bool) (UprightChoice, error) {
			if overview {
				return UprightChoice{Index: -1, Confidence: .6, Evidence: "text"}, nil
			}
			angle := 270
			if conflict && n == 6 {
				angle = 90
			}
			return selectClockwise(t, imgs, angle), nil
		}}
		got, err := DetectOrientation(t.Context(), p, "test", input)
		if err != nil {
			t.Fatal(err)
		}
		if p.calls != 6 {
			t.Fatal("expected overview and five independent regions", p.calls)
		}
		if conflict {
			if got.Degrees != 0 || got.Method != "uncertain" {
				t.Fatal(got)
			}
		} else if got.Degrees != 270 || got.Method != "regions" {
			t.Fatal(got)
		}
	}
	p := &directionProvider{choose: func(int, []Image, bool) (UprightChoice, error) {
		return UprightChoice{Index: -1, Evidence: "none"}, nil
	}}
	if _, err := DetectOrientation(t.Context(), p, "test", input); err != nil || p.calls != 1 {
		t.Fatal("scene ambiguity should not crop", err, p.calls)
	}
	p.choose = func(int, []Image, bool) (UprightChoice, error) { return UprightChoice{}, errors.New("network failure") }
	if _, err := DetectOrientation(t.Context(), p, "test", input); err == nil {
		t.Fatal("failure recorded as upright")
	}
}
func TestRotateImageRetainsPixelsAndDimensions(t *testing.T) {
	input := directionImage(t, 120, 80)
	rotated, err := RotateImage(input, 90)
	if err != nil {
		t.Fatal(err)
	}
	img, _, _ := image.Decode(bytes.NewReader(rotated.Data))
	if img.Bounds().Dx() != 80 || img.Bounds().Dy() != 120 {
		t.Fatal(img.Bounds())
	}
	r, g, _, _ := img.At(79, 0).RGBA()
	if r != 0 || g != 0 {
		t.Fatal("clockwise corner mismatch", r, g)
	}
	unchanged, _ := RotateImage(input, 0)
	if !bytes.Equal(unchanged.Data, input.Data) {
		t.Fatal("zero rotation reencoded input")
	}
	if _, err := RotateImage(input, 45); err == nil {
		t.Fatal("invalid angle accepted")
	}
}
