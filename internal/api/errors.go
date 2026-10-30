package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/errcodes"
	"github.com/jinzhao1994/kist/internal/wire"
)

// HTTPError preserves the CLI error envelope and adds settings field context.
type ErrorFields = wire.Error

type HTTPError struct {
	ErrorFields
	Status int                 `json:"-"`
	Fields []SettingFieldError `json:"fields,omitempty"`
}

// SettingFieldError carries a safe reason without echoing submitted values.
type SettingFieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func settingsFailure(err error) error {
	var field *config.FieldError
	if errors.As(err, &field) {
		return &HTTPError{Status: 400, ErrorFields: wire.Error{Code: "invalid_request", Class: "invalid", Message: "Invalid settings; correct the indicated field."}, Fields: []SettingFieldError{{Field: field.Field, Code: field.Code, Message: field.Message}}}
	}
	return invalid("Invalid settings; check the values and reload the configuration.")
}

func (e *HTTPError) Error() string  { return e.Message }
func (e *HTTPError) GetStatus() int { return e.Status }

func init() {
	// Huma's documented error factory also covers decoding and validation.
	// Install once, not in New: creating multiple servers must not mutate globals.
	huma.NewError = func(status int, message string, details ...error) huma.StatusError {
		code, class := "invalid_request", "invalid"
		switch {
		case status >= 500:
			code, class = "internal", "internal"
		case status == 404:
			code, class = "not_found", "not_found"
		case status == 403:
			code, class = "forbidden", "invalid"
		case status == 401:
			code, class = "unauthorized", "invalid"
		case status == 405:
			code = "method_not_allowed"
		case status == 413:
			code = "request_too_large"
		case status == 415:
			code = "unsupported_media_type"
		}
		out := &HTTPError{ErrorFields: wire.Error{Code: code, Class: class, Message: message}, Status: status}
		for _, err := range details {
			var detail huma.ErrorDetailer
			if !errors.As(err, &detail) {
				continue
			}
			location := detail.ErrorDetail().Location
			field := ""
			for _, pair := range [][2]string{{"body.ai.", "ai_options."}, {"body.location.", "geocoding."}, {"body.changes.", ""}} {
				if strings.HasPrefix(location, pair[0]) {
					field = pair[1] + strings.TrimPrefix(location, pair[0])
					break
				}
			}
			if field != "" {
				out.Fields = append(out.Fields, SettingFieldError{Field: field, Code: "value", Message: "Check the value's type, length and range."})
			}
		}
		return out
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
