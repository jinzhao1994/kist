package syncclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jinzhao1994/kist/internal/blob"
)

// OpenOriginal resumes an immutable original at an acknowledged local offset.
// The caller owns durable staging, byte limits and verification of the complete
// file's digest before making it available as cached content.
func (p *Peer) OpenOriginal(ctx context.Context, name string, size, offset int64) (io.ReadCloser, error) {
	if _, err := blob.ParseKey(name); err != nil {
		return nil, err
	}
	if size < 0 || offset < 0 || offset > size || offset == size && size != 0 {
		return nil, errors.New("invalid original download range")
	}
	headers := make(http.Header)
	if offset > 0 {
		headers.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
		headers.Set("If-Range", `"`+name+`"`)
	}
	response, err := p.request(ctx, "GET", "originals/"+url.PathEscape(name)+"/content", nil, headers)
	if err != nil {
		return nil, err
	}
	status := http.StatusOK
	if offset > 0 {
		status = http.StatusPartialContent
	}
	if response.StatusCode != status {
		response.Body.Close()
		return nil, &HTTPError{Status: response.StatusCode}
	}
	if response.Header.Get("ETag") != `"`+name+`"` || response.ContentLength != size-offset || offset > 0 && response.Header.Get("Content-Range") != fmt.Sprintf("bytes %d-%d/%d", offset, size-1, size) {
		response.Body.Close()
		return nil, errors.New("original response does not match the requested immutable range")
	}
	return response.Body, nil
}
