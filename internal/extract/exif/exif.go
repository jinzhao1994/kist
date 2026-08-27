// Package exif reads EXIF and GPS metadata out of image files.
//
// It is the one extractor v0.1 ships (spec 7, M3). Everything it knows about
// how metadata is stored lives in the extract package's contract; this one
// only decides which tags become rows and what type each one is.
package exif

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/bep/imagemeta"

	"github.com/jinzhao1994/kist/internal/extract"
)

// ID and Version go into file_metadata.source and file_extractions.
//
// Bump Version when the mapping below changes what a file would produce.
// Nothing re-extracts for any other reason (spec 1.7.6 ④), so a fix that is
// not accompanied by a bump reaches new files only.
const (
	ID      = "exif"
	Version = 1
)

// New returns the extractor.
func New() extract.Extractor { return extractor{} }

type extractor struct{}

func (extractor) ID() string           { return ID }
func (extractor) Version() int         { return Version }
func (extractor) Namespaces() []string { return []string{"exif", "gps"} }

// Wants answers from the media type rather than the file name, because an
// imported file's name is whatever the user called it and says nothing
// reliable about the bytes.
func (extractor) Wants(mime string) bool {
	_, ok := formats[mime]
	return ok
}

// formats maps a media type onto the container parser to use.
//
// HEIC is in here because iPhones shoot it by default, and its metadata is
// readable in pure Go even though its pixels are not: ffmpeg is needed for
// decoding, not for EXIF (spec 5.3). That is the difference between M3 being
// complete for iPhone users and being a degraded path for them.
var formats = map[string]imagemeta.ImageFormat{
	"image/jpeg":        imagemeta.JPEG,
	"image/tiff":        imagemeta.TIFF,
	"image/png":         imagemeta.PNG,
	"image/webp":        imagemeta.WebP,
	"image/heic":        imagemeta.HEIF,
	"image/heif":        imagemeta.HEIF,
	"image/avif":        imagemeta.AVIF,
	"image/x-adobe-dng": imagemeta.DNG,
}

// Extract reads r and returns the tags worth keeping, ordered by key.
//
// The order is not cosmetic. library compares a new extraction against the
// stored rows to decide what actually changed (spec 1.7.6), and an unordered
// result would make an unchanged file look changed on every run.
func (e extractor) Extract(ctx context.Context, r io.ReadSeeker, mime string) ([]extract.KV, error) {
	format, ok := formats[mime]
	if !ok {
		return nil, fmt.Errorf("exif: %s is not a format this extractor reads", mime)
	}

	seen := map[string]extract.KV{}
	_, err := imagemeta.Decode(imagemeta.Options{
		R:           r,
		ImageFormat: format,
		Sources:     imagemeta.EXIF,
		HandleTag: func(t imagemeta.TagInfo) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if isVendorBlob(t.Tag) {
				return nil
			}
			key, ok := mapKey(t.Namespace, t.Tag)
			if !ok {
				return nil
			}
			// First writer wins. imagemeta walks the main image before the
			// embedded thumbnail's IFD1, and IFD0:Make is the photograph's
			// make while IFD1:Make is the thumbnail's copy of it.
			if _, dup := seen[key]; dup {
				return nil
			}
			value, typ, ok := mapValue(t.Value)
			if !ok {
				return nil
			}
			seen[key] = extract.KV{Key: key, Value: value, Type: typ}
			return nil
		},
	})
	if !isNoMetadata(err) {
		return nil, fmt.Errorf("exif: %w", err)
	}

	out := make([]extract.KV, 0, len(seen))
	for _, kv := range seen {
		out = append(out, kv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// mapKey turns imagemeta's IFD path into one of our two namespaces.
//
// The IFD path is dropped, not preserved: IFD0:Make and IFD1:Make mean the
// same thing to a person, and "exif:Make" is what somebody would search for.
// Anything outside the two namespaces this extractor owns is skipped rather
// than renamed — the ownership rule is enforced by extract.Check, and a
// silent rename would be a way around it.
func mapKey(namespace, tag string) (string, bool) {
	if strings.Contains(namespace, "GPSInfoIFD") {
		return "gps:" + strings.TrimPrefix(tag, "GPS"), true
	}
	if strings.HasPrefix(namespace, "IFD") {
		return "exif:" + tag, true
	}
	return "", false
}

func init() { extract.Register(New()) }
