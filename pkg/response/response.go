// Package response provides a unified API response format for HTTP handlers.
// It standardizes success and error responses with consistent JSON structure.
package response

import (
	"encoding/json"
	"net/http"
)

// Response is the standard API response wrapper.
// It provides a consistent structure for all API responses.
type Response struct {
	// Code is the business status code (0 means success).
	Code int `json:"code"`
	// Message is a human-readable description of the response.
	Message string `json:"message"`
	// Data is the response payload (can be any type).
	Data interface{} `json:"data,omitempty"`
	// Error is the error detail when Code != 0.
	Error *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail provides structured error information.
type ErrorDetail struct {
	// Type is the error category (e.g., "validation", "not_found", "internal").
	Type string `json:"type"`
	// Field is the field name that caused the error (for validation errors).
	Field string `json:"field,omitempty"`
	// Details contains additional error context.
	Details string `json:"details,omitempty"`
}

// Success writes a successful JSON response with the given data.
// The HTTP status code defaults to 200 OK.
func Success(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// SuccessCreated writes a successful response for resource creation (201 Created).
func SuccessCreated(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// SuccessNoContent writes a successful response with no body (204 No Content).
func SuccessNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// SuccessNotModified writes a 304 Not Modified response.
func SuccessNotModified(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotModified)
}

// Error writes an error response with the given HTTP status code, business code, and message.
func Error(w http.ResponseWriter, httpStatus int, code int, message string) {
	JSON(w, httpStatus, Response{
		Code:    code,
		Message: message,
		Error: &ErrorDetail{
			Type: mapHTTPCodeToErrorType(httpStatus),
		},
	})
}

// ErrorWithDetail writes an error response with additional detail information.
func ErrorWithDetail(w http.ResponseWriter, httpStatus int, code int, message, errorType, field, details string) {
	JSON(w, httpStatus, Response{
		Code:    code,
		Message: message,
		Error: &ErrorDetail{
			Type:    errorType,
			Field:   field,
			Details: details,
		},
	})
}

// BadRequest writes a 400 Bad Request error response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, 40000, message)
}

// BadRequestWithField writes a 400 Bad Request error response with a specific field.
func BadRequestWithField(w http.ResponseWriter, field, message string) {
	ErrorWithDetail(w, http.StatusBadRequest, 40000, message, "validation", field, "")
}

// Unauthorized writes a 401 Unauthorized error response.
func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnauthorized, 40100, message)
}

// Forbidden writes a 403 Forbidden error response.
func Forbidden(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, 40300, message)
}

// NotFound writes a 404 Not Found error response.
func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, 40400, message)
}

// MethodNotAllowed writes a 405 Method Not Allowed error response.
func MethodNotAllowed(w http.ResponseWriter, message string) {
	Error(w, http.StatusMethodNotAllowed, 40500, message)
}

// Conflict writes a 409 Conflict error response.
func Conflict(w http.ResponseWriter, message string) {
	Error(w, http.StatusConflict, 40900, message)
}

// TooManyRequests writes a 429 Too Many Requests error response.
func TooManyRequests(w http.ResponseWriter, message string) {
	Error(w, http.StatusTooManyRequests, 42900, message)
}

// InternalError writes a 500 Internal Server Error response.
func InternalError(w http.ResponseWriter, message string) {
	Error(w, http.StatusInternalServerError, 50000, message)
}

// ServiceUnavailable writes a 503 Service Unavailable error response.
func ServiceUnavailable(w http.ResponseWriter, message string) {
	Error(w, http.StatusServiceUnavailable, 50300, message)
}

// JSON writes a JSON response with the given status code and body.
func JSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body != nil {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(body)
	}
}

// mapHTTPCodeToErrorType maps HTTP status codes to error type strings.
func mapHTTPCodeToErrorType(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "validation"
	case http.StatusUnauthorized:
		return "authentication"
	case http.StatusForbidden:
		return "authorization"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusGone:
		return "gone"
	case http.StatusTooManyRequests:
		return "rate_limit"
	case http.StatusInternalServerError:
		return "internal"
	case http.StatusNotImplemented:
		return "not_implemented"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "unknown"
	}
}

// PaginatedData represents a paginated list response.
type PaginatedData struct {
	Items      interface{} `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// SuccessPaginated writes a successful paginated response.
func SuccessPaginated(w http.ResponseWriter, items interface{}, total, page, pageSize int) {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	Success(w, PaginatedData{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}
