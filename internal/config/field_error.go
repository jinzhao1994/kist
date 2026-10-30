package config

// FieldError identifies a setting without echoing its submitted value. Codes
// describe validation failures; clients may translate them without owning rules.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

// InvalidField attaches a stable field identifier to a validation failure.
func InvalidField(field, code, message string) error {
	return &FieldError{Field: field, Code: code, Message: message}
}
