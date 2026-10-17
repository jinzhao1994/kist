// Package media records content-derived MIME types and broad media kinds.
package media

import (
	"context"
	"io"
	"strings"

	"github.com/jinzhao1994/kist/internal/extract"
)

const ID = "media"
const Version = 2

type extractor struct{}

func New() extract.Extractor           { return extractor{} }
func (extractor) ID() string           { return ID }
func (extractor) Version() int         { return Version }
func (extractor) Namespaces() []string { return []string{"media"} }
func (extractor) Wants(string) bool    { return true }
func (extractor) Extract(ctx context.Context, r io.ReadSeeker, mime string) ([]extract.KV, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	kind := "other"
	switch {
	case strings.HasPrefix(mime, "image/"):
		kind = "image"
	case strings.HasPrefix(mime, "video/"):
		kind = "video"
	case strings.HasPrefix(mime, "audio/"):
		kind = "audio"
	case strings.HasPrefix(mime, "text/"), mime == "application/pdf", mime == "application/postscript":
		kind = "document"
	}
	return []extract.KV{{Key: "media:kind", Value: kind, Type: extract.TypeText}, {Key: "media:mime", Value: mime, Type: extract.TypeText}}, nil
}
func init() { extract.Register(New()) }
