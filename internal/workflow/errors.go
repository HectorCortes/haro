package workflow

import "fmt"

// ValidationError is a typed error with code, field path, and human message.
type ValidationError struct {
	Code    string
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s@%s: %s", e.Code, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// AsValidationError helpers
func newValidationError(code, field, msg string) *ValidationError {
	return &ValidationError{Code: code, Field: field, Message: msg}
}
