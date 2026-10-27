package ai

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
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

func TestNormalizeAppliesContainerBeforeCorrection(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			src.Set(x, y, color.NRGBA{B: 255, A: 255})
		}
	}
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			src.Set(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	jpeg.Encode(&b, src, &jpeg.Options{Quality: 100})
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	marker := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(marker[2:], uint16(len(exif)+2))
	raw := append([]byte{}, b.Bytes()[:2]...)
	raw = append(raw, marker...)
	raw = append(raw, exif...)
	raw = append(raw, b.Bytes()[2:]...)
	normalized, err := NormalizeImage(Image{MIME: "image/jpeg", Data: raw})
	if err != nil {
		t.Fatal(err)
	}
	img, _, _ := image.Decode(bytes.NewReader(normalized.Data))
	if img.Bounds().Dx() != 40 || img.Bounds().Dy() != 80 {
		t.Fatal("EXIF not applied", img.Bounds())
	}
	corrected, err := RotateImage(normalized, 270)
	if err != nil {
		t.Fatal(err)
	}
	img, _, _ = image.Decode(bytes.NewReader(corrected.Data))
	if img.Bounds().Dx() != 80 || img.Bounds().Dy() != 40 {
		t.Fatal("correction not applied after EXIF", img.Bounds())
	}
	red, _, blue, _ := img.At(2, 2).RGBA()
	if red < 60000 || blue > 5000 {
		t.Fatal("unexpected corner", red, blue)
	}
	repeated, err := NormalizeImage(corrected)
	if err != nil || !bytes.Equal(repeated.Data, corrected.Data) {
		t.Fatal("container orientation applied twice", err)
	}
}
