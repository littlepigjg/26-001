package handler

import (
	"net/http"

	"config-center/pkg/response"
)

// CreateAppRequest is the request body for creating an application.
type CreateAppRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
}

// UpdateAppRequest is the request body for updating an application.
type UpdateAppRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
}

// AppByID handles GET/PUT/DELETE /api/apps/{id}
func (h *Handlers) AppByID(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/api/apps/"
	appID := path[len(prefix):]

	switch r.Method {
	case http.MethodGet:
		h.getApp(w, r, appID)
	case http.MethodPut:
		h.updateApp(w, r, appID)
	case http.MethodDelete:
		h.deleteApp(w, r, appID)
	default:
		response.MethodNotAllowed(w, "method not allowed")
	}
}

// ListApps handles GET /api/apps (list) and POST /api/apps (create).
func (h *Handlers) ListApps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listApps(w, r)
	case http.MethodPost:
		h.createApp(w, r)
	default:
		response.MethodNotAllowed(w, "method not allowed")
	}
}

// createApp creates a new application.
func (h *Handlers) createApp(w http.ResponseWriter, r *http.Request) {
	var req CreateAppRequest
	if err := readJSONBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	user := getCurrentUser(r)
	appID := req.Name // Use name as ID for simplicity

	app, err := h.AppService.CreateApp(r.Context(), appID, req.Name, req.Description, req.Owner)
	if err != nil {
		handleError(w, err)
		return
	}

	// Log the creation
	_ = h.AuditService.LogSuccess(r.Context(), "CREATE", "app", app.ID, app.ID, "", user, getClientIP(r),
		"created application: "+app.Name)

	response.SuccessCreated(w, app)
}

// listApps returns a paginated list of applications.
func (h *Handlers) listApps(w http.ResponseWriter, r *http.Request) {
	page := parseQueryInt(r, "page", 1)
	pageSize := parseQueryInt(r, "page_size", 20)

	apps, total, err := h.AppService.ListApps(r.Context(), page, pageSize)
	if err != nil {
		handleError(w, err)
		return
	}

	response.SuccessPaginated(w, apps, total, page, pageSize)
}

// getApp returns a single application by ID.
func (h *Handlers) getApp(w http.ResponseWriter, r *http.Request, appID string) {
	app, err := h.AppService.GetApp(r.Context(), appID)
	if err != nil {
		handleError(w, err)
		return
	}

	response.Success(w, app)
}

// updateApp updates an existing application.
func (h *Handlers) updateApp(w http.ResponseWriter, r *http.Request, appID string) {
	var req UpdateAppRequest
	if err := readJSONBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	user := getCurrentUser(r)
	app, err := h.AppService.UpdateApp(r.Context(), appID, req.Name, req.Description, req.Owner)
	if err != nil {
		handleError(w, err)
		return
	}

	_ = h.AuditService.LogSuccess(r.Context(), "UPDATE", "app", app.ID, app.ID, "", user, getClientIP(r),
		"updated application: "+app.Name)

	response.Success(w, app)
}

// deleteApp deletes an application.
func (h *Handlers) deleteApp(w http.ResponseWriter, r *http.Request, appID string) {
	user := getCurrentUser(r)
	if err := h.AppService.DeleteApp(r.Context(), appID); err != nil {
		handleError(w, err)
		return
	}

	_ = h.AuditService.LogSuccess(r.Context(), "DELETE", "app", appID, appID, "", user, getClientIP(r),
		"deleted application: "+appID)

	response.SuccessNoContent(w)
}
