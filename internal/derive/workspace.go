package derive

import (
	"context"
	"image"
	"io"
	"os"
)

// WorkspaceBudget is a conservative admission estimate, not a second format
// validator. The caller must still measure and bound actual scratch usage.
func WorkspaceBudget(ctx context.Context, input, recipe string) (int64, error) {
	spec, ok := Lookup(recipe)
	if !ok {
		return 0, fail("unsupported", "unknown media recipe")
	}
	// Native image conversion writes only a resized JPEG. External converters
	// may also materialize full decoded images, PDFs and private profiles.
	if recipe == Thumbnail || recipe == Preview || recipe == VideoThumbnail {
		f, err := os.Open(input)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		cfg, _, err := image.DecodeConfig(io.LimitReader(contextReader{ctx, f}, MaxImageBytes+1))
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if err == nil && validDimensions(cfg.Width, cfg.Height) {
			pixels := int64(min(spec.Size, cfg.Width)) * int64(min(spec.Size, cfg.Height))
			return max(1<<20, pixels*8+(64<<10)), nil
		}
	}
	return MaxOutputBytes + maxOfficeWorkBytes, nil
}
