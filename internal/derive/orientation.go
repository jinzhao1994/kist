package derive

import (
	"context"
	"fmt"
	"image"
	"io"
	"strconv"
	"time"

	"github.com/bep/imagemeta"
	"github.com/disintegration/imaging"
)

// Read orientation from the original container, including PNG/WebP/TIFF EXIF.
// imaging's AutoOrientation only handles JPEG, so using it alone loses rotations
// when a camera or image editor writes EXIF to another supported format.
func orientation(ctx context.Context, r io.ReadSeeker, format string) int {
	var f imagemeta.ImageFormat
	switch format {
	case "jpeg":
		f = imagemeta.JPEG
	case "png":
		f = imagemeta.PNG
	case "webp":
		f = imagemeta.WebP
	case "tiff":
		f = imagemeta.TIFF
	default:
		return 1
	}
	value := 1
	_, _ = imagemeta.Decode(imagemeta.Options{
		R: seekReader{ctx, r}, ImageFormat: f, Sources: imagemeta.EXIF, Timeout: 5 * time.Second,
		ShouldHandleTag: func(t imagemeta.TagInfo) bool { return t.Namespace == "IFD0" && t.Tag == "Orientation" },
		HandleTag: func(t imagemeta.TagInfo) error {
			n, e := strconv.Atoi(fmt.Sprint(t.Value))
			if e == nil && n >= 1 && n <= 8 {
				value = n
			}
			return imagemeta.ErrStopWalking
		},
	})
	return value
}
func orient(img image.Image, value int) image.Image {
	switch value {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img)
	}
	return img
}

type seekReader struct {
	ctx context.Context
	io.ReadSeeker
}

func (r seekReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.ReadSeeker.Read(p)
}
