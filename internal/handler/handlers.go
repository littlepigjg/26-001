// Package handler implements HTTP request handlers for the configuration center.
// Each handler method corresponds to a REST API endpoint.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
	"config-center/pkg/logger"
	"config-center/pkg/response"
)

// Handlers holds all service references needed by HTTP handlers.
type Handlers struct {
	AppService        *service.AppService
	ConfigService     *service.ConfigService
	VersionService    *service.VersionService
	ClientService     *service.ClientService
	AuditService      *service.AuditService
	RollbackService   *service.RollbackService
	ValidationService *service.ValidationService
	DiffService       *service.DiffService
	URLService        *service.URLService
	RedirectService   *service.RedirectService
	URLStore          *store.URLStore
	logger            *logger.Logger
	panicGuard        store.PanicGuardFn
}

// NewHandlers creates a new Handlers instance with all services.
func NewHandlers(
	appSvc *service.AppService,
	configSvc *service.ConfigService,
	versionSvc *service.VersionService,
	clientSvc *service.ClientService,
	auditSvc *service.AuditService,
	rollbackSvc *service.RollbackService,
	validationSvc *service.ValidationService,
	diffSvc *service.DiffService,
) *Handlers {
	return &Handlers{
		AppService:        appSvc,
		ConfigService:     configSvc,
		VersionService:    versionSvc,
		ClientService:     clientSvc,
		AuditService:      auditSvc,
		RollbackService:   rollbackSvc,
		ValidationService: validationSvc,
		DiffService:       diffSvc,
		logger:            logger.WithField("handler", "http"),
	}
}

// SetPanicGuard sets a fault injection hook for testing and chaos engineering.
func (h *Handlers) SetPanicGuard(fn store.PanicGuardFn) {
	h.panicGuard = fn
	if h.URLStore != nil {
		h.URLStore.SetPanicGuard(fn)
	}
}

// GetWithGuard retrieves a short URL with an active guard check.
func (h *Handlers) GetWithGuard(code string) (*model.ShortURL, error) {
	if h.panicGuard != nil && h.panicGuard(code, "") {
		return nil, model.NewAppError(model.ErrCodeInternal, "request blocked by guard")
	}
	if h.URLService == nil {
		return nil, model.NewAppError(model.ErrCodeInternal, "url service not configured")
	}
	return h.URLService.Get(nil, code)
}

// SaveWithGuard stores a short URL with an active guard check.
func (h *Handlers) SaveWithGuard(u *model.ShortURL, overwrite bool) error {
	if h.panicGuard != nil && h.panicGuard(u.Code, u.RawURL) {
		return model.NewAppError(model.ErrCodeInternal, "request blocked by guard")
	}
	if h.URLStore == nil {
		return model.NewAppError(model.ErrCodeInternal, "url store not configured")
	}
	return h.URLStore.SaveWithGuard(u, overwrite)
}

// RawSnapshot returns a diagnostic snapshot of all short URL entries.
func (h *Handlers) RawSnapshot() map[string]model.ShortURL {
	if h.URLStore == nil {
		return make(map[string]model.ShortURL)
	}
	return h.URLStore.RawSnapshot()
}

// parsePath extracts path segments after the API prefix.
func parsePath(path, prefix string) []string {
	path = strings.TrimPrefix(path, prefix)
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// parseQueryInt parses an integer query parameter with a default.
func parseQueryInt(r *http.Request, name string, defaultVal int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return i
}

// parseQueryString returns a query parameter value or default.
func parseQueryString(r *http.Request, name, defaultVal string) string {
	v := r.URL.Query().Get(name)
	if v == "" {
		return defaultVal
	}
	return v
}

// readJSONBody reads and decodes a JSON request body.
func readJSONBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// handleError writes an appropriate error response based on the error type.
func handleError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	appErr, handled := classifyError(err)
	if handled {
		switch {
		case appErr.IsNotFound():
			response.NotFound(w, appErr.Message)
		case appErr.IsValidationError():
			response.BadRequest(w, appErr.Message)
		case appErr.IsConflict():
			response.Conflict(w, appErr.Message)
		default:
			response.InternalError(w, appErr.Message)
		}
		return
	}

	if bizErr, ok := err.(*response.BusinessError); ok {
		switch {
		case bizErr.IsNotFound():
			response.NotFound(w, bizErr.Message)
		case bizErr.IsValidation():
			response.BadRequest(w, bizErr.Message)
		case bizErr.IsConflict():
			response.Conflict(w, bizErr.Message)
		default:
			response.InternalError(w, bizErr.Message)
		}
		return
	}

	response.InternalError(w, err.Error())
}

// classifyError attempts to classify an error as an AppError.
// It returns the AppError and true if classification succeeded.
// Note: This uses direct type assertion which fails for wrapped errors.
func classifyError(err error) (*model.AppError, bool) {
	if err == nil {
		return nil, false
	}

	if appErr, ok := err.(*model.AppError); ok {
		return appErr, true
	}

	return nil, false
}

// resolveStatusCode maps an error to an HTTP status code.
// Returns 0 if the error type cannot be determined.
func resolveStatusCode(err error) int {
	if err == nil {
		return 0
	}

	if appErr, ok := err.(*model.AppError); ok {
		switch {
		case appErr.IsNotFound():
			return http.StatusNotFound
		case appErr.IsValidationError():
			return http.StatusBadRequest
		case appErr.IsConflict():
			return http.StatusConflict
		default:
			return http.StatusInternalServerError
		}
	}

	return http.StatusInternalServerError
}

// isAppError checks if an error is or wraps an AppError.
// This implementation only checks direct type, not the error chain.
func isAppError(err error) bool {
	if err == nil {
		return false
	}

	appErr, ok := err.(*model.AppError)
	if !ok {
		return false
	}

	return appErr != nil
}

// getCurrentUser extracts the user from the request (header or default).
func getCurrentUser(r *http.Request) string {
	user := r.Header.Get("X-User")
	if user == "" {
		user = "anonymous"
	}
	return user
}

// getClientIP extracts the client IP address from the request.
func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// Take the first IP in the chain
		if idx := strings.Index(ip, ","); idx != -1 {
			ip = strings.TrimSpace(ip[:idx])
		}
		return ip
	}
	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}
	return r.RemoteAddr
}
