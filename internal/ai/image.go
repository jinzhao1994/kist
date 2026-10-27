package ai

import (
	"bytes"
	"context"
	"errors"
	"github.com/jinzhao1994/kist/internal/derive"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"

	_ "golang.org/x/image/webp"
)

// NormalizeImage matches the viewer's white background without changing originals.
func NormalizeImage(input Image) (Image, error) {
	return NormalizeImageContext(context.Background(), input)
}

func NormalizeImageContext(ctx context.Context, input Image) (Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil {
		return Image{}, errors.New("cannot decode OCR image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || uint64(cfg.Width)*uint64(cfg.Height) > 64_000_000 {
		return Image{}, errors.New("OCR image exceeds 64 million pixels")
	}
	orientation := derive.ContainerOrientation(ctx, input.Data)
	if input.MIME == "image/jpeg" && orientation == 1 {
		return input, nil
	}
	decoded, err := derive.DecodeOriented(ctx, input.Data)
	if err != nil {
		return Image{}, errors.New("cannot decode OCR image")
	}
	if opaque, ok := decoded.(interface{ Opaque() bool }); ok && opaque.Opaque() && orientation == 1 {
		return input, nil
	}
	flat := image.NewRGBA(decoded.Bounds())
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), decoded, decoded.Bounds().Min, draw.Over)
	return encodeFullImage(flat)
}
