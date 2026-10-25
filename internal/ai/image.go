package ai

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"

	_ "golang.org/x/image/webp"
)

// NormalizeImage matches the viewer's white background without changing originals.
func NormalizeImage(input Image) (Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil {
		return Image{}, errors.New("cannot decode OCR image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || uint64(cfg.Width)*uint64(cfg.Height) > 64_000_000 {
		return Image{}, errors.New("OCR image exceeds 64 million pixels")
	}
	if input.MIME == "image/jpeg" {
		return input, nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(input.Data))
	if err != nil {
		return Image{}, errors.New("cannot decode OCR image")
	}
	if opaque, ok := decoded.(interface{ Opaque() bool }); ok && opaque.Opaque() {
		return input, nil
	}
	flat := image.NewRGBA(decoded.Bounds())
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), decoded, decoded.Bounds().Min, draw.Over)
	var out bytes.Buffer
	if err = png.Encode(&out, flat); err != nil {
		return Image{}, err
	}
	if out.Len() > MaxImageBytes {
		return Image{}, errors.New("normalized OCR image exceeds 20 MiB")
	}
	return Image{MIME: "image/png", Data: out.Bytes()}, nil
}
