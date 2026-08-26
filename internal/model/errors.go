package model

import "fmt"

// ErrorCode defines application-level error codes.
type ErrorCode int

const (
	// ErrCodeSuccess indicates success (not actually an error).
	ErrCodeSuccess ErrorCode = 0
	// ErrCodeInvalidParam indicates invalid parameter.
	ErrCodeInvalidParam ErrorCode = 40001
	// ErrCodeNotFound indicates resource not found.
	ErrCodeNotFound ErrorCode = 40401
	// ErrCodeAlreadyExists indicates duplicate resource.
	ErrCodeAlreadyExists ErrorCode = 40901
	// ErrCodeConflict indicates a state conflict.
	ErrCodeConflict ErrorCode = 40902
	// ErrCodeValidationFailed indicates validation failure.
	ErrCodeValidationFailed ErrorCode = 40002
	// ErrCodeInternal indicates an internal server error.
	ErrCodeInternal ErrorCode = 50001
	// ErrCodeStorageFailure indicates a storage backend failure.
	ErrCodeStorageFailure ErrorCode = 50002
	// ErrCodeVersionConflict indicates a version conflict.
	ErrCodeVersionConflict ErrorCode = 40903
	// ErrCodeEnvironmentMismatch indicates environment mismatch.
	ErrCodeEnvironmentMismatch ErrorCode = 40003
	// ErrCodeLocked indicates a resource is locked.
	ErrCodeLocked ErrorCode = 42301
	// ErrCodeTooManyRequests indicates rate limiting.
	ErrCodeTooManyRequests ErrorCode = 42901
	// ErrCodeUnauthorized indicates authentication failure.
	ErrCodeUnauthorized ErrorCode = 40101
	// ErrCodeForbidden indicates authorization failure.
	ErrCodeForbidden ErrorCode = 40301
)

// AppError represents an application-level error with a specific error code.
type AppError struct {
	// Code is the error code.
	Code ErrorCode `json:"code"`
	// Message is the error message.
	Message string `json:"message"`
	// Details provides additional error details.
	Details string `json:"details,omitempty"`
	// Err is the underlying error.
	Err error `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is/errors.As support.
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new AppError.
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// NewAppErrorWithCause creates an AppError wrapping an underlying cause.
func NewAppErrorWithCause(code ErrorCode, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     cause,
	}
}

// IsNotFound checks if the error is a not-found error.
func (e *AppError) IsNotFound() bool {
	return e.Code == ErrCodeNotFound
}

// IsValidationError checks if the error is a validation error.
func (e *AppError) IsValidationError() bool {
	return e.Code == ErrCodeInvalidParam || e.Code == ErrCodeValidationFailed
}

// IsConflict checks if the error is a conflict error.
func (e *AppError) IsConflict() bool {
	return e.Code == ErrCodeAlreadyExists || e.Code == ErrCodeConflict || e.Code == ErrCodeVersionConflict
}

// IsInternal checks if the error is an internal error.
func (e *AppError) IsInternal() bool {
	return e.Code == ErrCodeInternal || e.Code == ErrCodeStorageFailure
}

// Common error constructors

// ErrInvalidParam creates an invalid parameter error.
func ErrInvalidParam(param, reason string) *AppError {
	return &AppError{
		Code:    ErrCodeInvalidParam,
		Message: fmt.Sprintf("invalid parameter '%s': %s", param, reason),
	}
}

// ErrAppNotFound creates an application not-found error.
func ErrAppNotFound(appID string) *AppError {
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("application '%s' not found", appID),
	}
}

// ErrConfigNotFound creates a config not-found error.
func ErrConfigNotFound(appID, env, key string) *AppError {
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("config key '%s' not found for app '%s' in environment '%s'", key, appID, env),
	}
}

// ErrVersionNotFound creates a version not-found error.
func ErrVersionNotFound(appID string, version int) *AppError {
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("version %d not found for app '%s'", version, appID),
	}
}

// ErrAppAlreadyExists creates a duplicate application error.
func ErrAppAlreadyExists(appID string) *AppError {
	return &AppError{
		Code:    ErrCodeAlreadyExists,
		Message: fmt.Sprintf("application '%s' already exists", appID),
	}
}

// ErrValidationFailed creates a validation failure error.
func ErrValidationFailed(message string) *AppError {
	return &AppError{
		Code:    ErrCodeValidationFailed,
		Message: message,
	}
}

// ErrInternal creates an internal server error.
func ErrInternal(message string) *AppError {
	return &AppError{
		Code:    ErrCodeInternal,
		Message: message,
	}
}

// ErrEnvironmentNotSupported creates an environment mismatch error.
func ErrEnvironmentNotSupported(appID, env string) *AppError {
	return &AppError{
		Code:    ErrCodeEnvironmentMismatch,
		Message: fmt.Sprintf("environment '%s' is not supported by app '%s'", env, appID),
	}
}

// ErrVersionConflict creates a version conflict error for optimistic locking.
func ErrVersionConflict(appID, env string, expectedVersion int) *AppError {
	return &AppError{
		Code:    ErrCodeVersionConflict,
		Message: fmt.Sprintf("version conflict for app '%s' in env '%s': expected version %d", appID, env, expectedVersion),
	}
}
