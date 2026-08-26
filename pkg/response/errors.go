package response

import "fmt"

// BusinessError represents an error with a business error code.
// It implements the error interface and can be used to pass structured
// error information up through the call stack.
type BusinessError struct {
	// Code is the business error code.
	Code int `json:"code"`
	// Message is the human-readable error message.
	Message string `json:"message"`
	// Type is the error category.
	Type string `json:"type"`
	// Field is the field name associated with the error (for validation).
	Field string `json:"field,omitempty"`
	// Details contains additional error context.
	Details string `json:"details,omitempty"`
	// Err is the underlying error that caused this BusinessError.
	Err error `json:"-"`
}

// NewBusinessError creates a new BusinessError.
func NewBusinessError(code int, message string) *BusinessError {
	return &BusinessError{
		Code:    code,
		Message: message,
		Type:    "business",
	}
}

// NewValidationError creates a new validation error.
func NewValidationError(message string) *BusinessError {
	return &BusinessError{
		Code:    40000,
		Message: message,
		Type:    "validation",
	}
}

// NewValidationFieldError creates a validation error for a specific field.
func NewValidationFieldError(field, message string) *BusinessError {
	return &BusinessError{
		Code:    40000,
		Message: message,
		Type:    "validation",
		Field:   field,
	}
}

// NewNotFoundError creates a not-found error.
func NewNotFoundError(resource string) *BusinessError {
	return &BusinessError{
		Code:    40400,
		Message: fmt.Sprintf("%s not found", resource),
		Type:    "not_found",
	}
}

// NewConflictError creates a conflict error.
func NewConflictError(message string) *BusinessError {
	return &BusinessError{
		Code:    40900,
		Message: message,
		Type:    "conflict",
	}
}

// NewInternalError creates an internal server error.
func NewInternalError(message string) *BusinessError {
	return &BusinessError{
		Code:    50000,
		Message: message,
		Type:    "internal",
	}
}

// NewInternalErrorWithCause creates an internal error wrapping an underlying cause.
func NewInternalErrorWithCause(message string, cause error) *BusinessError {
	return &BusinessError{
		Code:    50000,
		Message: message,
		Type:    "internal",
		Err:     cause,
	}
}

// Error implements the error interface.
func (e *BusinessError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Err.Error())
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause for errors.Is/errors.As support.
func (e *BusinessError) Unwrap() error {
	return e.Err
}

// WithCause adds an underlying cause to the error.
func (e *BusinessError) WithCause(err error) *BusinessError {
	e.Err = err
	return e
}

// WithField adds a field name to the error.
func (e *BusinessError) WithField(field string) *BusinessError {
	e.Field = field
	return e
}

// IsValidation checks if this error is a validation error.
func (e *BusinessError) IsValidation() bool {
	return e.Type == "validation"
}

// IsNotFound checks if this error is a not-found error.
func (e *BusinessError) IsNotFound() bool {
	return e.Type == "not_found"
}

// IsConflict checks if this error is a conflict error.
func (e *BusinessError) IsConflict() bool {
	return e.Type == "conflict"
}

// IsInternal checks if this error is an internal error.
func (e *BusinessError) IsInternal() bool {
	return e.Type == "internal"
}
