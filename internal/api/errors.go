package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/errcodes"
	"github.com/jinzhao1994/kist/internal/wire"
)

// HTTPError preserves the same machine-readable error fields as the CLI.
type ErrorFields = wire.Error

type HTTPError struct {
	ErrorFields
	Status int `json:"-"`
}

func (e *HTTPError) Error() string  { return e.Message }
func (e *HTTPError) GetStatus() int { return e.Status }

func init() {
	// Huma's documented error factory also covers decoding and validation.
	// Install once, not in New: creating multiple servers must not mutate globals.
	huma.NewError = func(status int, message string, _ ...error) huma.StatusError {
		code, class := "invalid_request", "invalid"
		switch {
		case status >= 500:
			code, class = "internal", "internal"
		case status == 404:
			code, class = "not_found", "not_found"
		case status == 403:
			code, class = "forbidden", "invalid"
		case status == 405:
			code = "method_not_allowed"
		case status == 413:
			code = "request_too_large"
		case status == 415:
			code = "unsupported_media_type"
		}
		return &HTTPError{ErrorFields: wire.Error{Code: code, Class: class, Message: message}, Status: status}
	}
}

func failure(err error) error {
	if err == nil {
		return nil
	}
	var existing *HTTPError
	if errors.As(err, &existing) {
		return existing
	}
	out := errcodes.Render(err)
	status := http.StatusInternalServerError
	switch errcodes.ClassOf(err) {
	case errcodes.ClassNotFound:
		status = http.StatusNotFound
	case errcodes.ClassInvalid, errcodes.ClassAmbiguous:
		status = http.StatusBadRequest
	case errcodes.ClassConflict:
		status = http.StatusConflict
	case errcodes.ClassUnavailable:
		status = http.StatusServiceUnavailable
	}
	if status == 500 {
		slog.Error("API request failed", "error", err)
		out.Message = "The library could not complete this request; inspect the server log."
	}
	return &HTTPError{ErrorFields: out, Status: status}
}

func invalid(message string) error { return huma.Error400BadRequest(message) }
