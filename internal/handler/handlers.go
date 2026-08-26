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
	"config-center/pkg/logger"
	"config-center/pkg/response"
)

// Handlers holds all service references needed by HTTP handlers.
type Handlers struct {
	AppService       *service.AppService
	ConfigService    *service.ConfigService
	VersionService   *service.VersionService
	ClientService    *service.ClientService
	AuditService     *service.AuditService
	RollbackService  *service.RollbackService
	ValidationService *service.ValidationService
	DiffService      *service.DiffService
	logger           *logger.Logger
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
		AppService:       appSvc,
		ConfigService:    configSvc,
		VersionService:   versionSvc,
		ClientService:    clientSvc,
		AuditService:     auditSvc,
		RollbackService:  rollbackSvc,
		ValidationService: validationSvc,
		DiffService:      diffSvc,
		logger:           logger.WithField("handler", "http"),
	}
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

	if appErr, ok := err.(*model.AppError); ok {
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

	// Generic error
	response.InternalError(w, err.Error())
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
