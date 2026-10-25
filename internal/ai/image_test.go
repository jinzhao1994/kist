package ai

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestNormalizeTransparentTextOnWhite(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	src.SetNRGBA(1, 0, color.NRGBA{A: 255})
	var raw bytes.Buffer
	png.Encode(&raw, src)
	out, err := NormalizeImage(Image{MIME: "image/png", Data: raw.Bytes()})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out.Data))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(img.At(0, 0)); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatal(got)
	}
	if got := color.RGBAModel.Convert(img.At(1, 0)); got != (color.RGBA{0, 0, 0, 255}) {
		t.Fatal(got)
	}
	if src.NRGBAAt(0, 0).A != 0 {
		t.Fatal("original modified")
	}
}
