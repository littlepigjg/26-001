package handler

import (
	"net/http"
	"strconv"

	"config-center/pkg/response"
)

// CreateVersionRequest is the request body for creating a version.
type CreateVersionRequest struct {
	AppID       string `json:"app_id"`
	Environment string `json:"environment"`
	Summary     string `json:"summary"`
}

// ListVersions handles GET /api/versions and POST /api/versions.
func (h *Handlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listVersions(w, r)
	case http.MethodPost:
		h.createVersion(w, r)
	default:
		response.MethodNotAllowed(w, "method not allowed")
	}
}

// VersionByNumber handles GET /api/versions/{app_id}/{env}/{version}
func (h *Handlers) VersionByNumber(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/api/versions/"
	rest := path[len(prefix):]

	parts := splitPath(rest)
	if len(parts) < 3 {
		response.BadRequest(w, "path must be /api/versions/{app_id}/{env}/{version}")
		return
	}

	appID := parts[0]
	env := parts[1]
	versionStr := parts[2]

	switch r.Method {
	case http.MethodGet:
		h.getVersion(w, r, appID, env, versionStr)
	default:
		response.MethodNotAllowed(w, "method not allowed")
	}
}

// listVersions returns version history.
func (h *Handlers) listVersions(w http.ResponseWriter, r *http.Request) {
	appID := parseQueryString(r, "app_id", "")
	env := parseQueryString(r, "environment", "")
	page := parseQueryInt(r, "page", 1)
	pageSize := parseQueryInt(r, "page_size", 20)

	if appID == "" {
		response.BadRequest(w, "app_id query parameter is required")
		return
	}
	if env == "" {
		response.BadRequest(w, "environment query parameter is required")
		return
	}

	versions, total, err := h.VersionService.ListVersions(r.Context(), appID, env, page, pageSize)
	if err != nil {
		handleError(w, err)
		return
	}

	response.SuccessPaginated(w, versions, total, page, pageSize)
}

// createVersion creates a new version snapshot.
func (h *Handlers) createVersion(w http.ResponseWriter, r *http.Request) {
	var req CreateVersionRequest
	if err := readJSONBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	if req.AppID == "" {
		response.BadRequest(w, "app_id is required")
		return
	}
	if req.Environment == "" {
		response.BadRequest(w, "environment is required")
		return
	}

	user := getCurrentUser(r)
	version, err := h.VersionService.CreateVersion(r.Context(), req.AppID, req.Environment, user, req.Summary)
	if err != nil {
		handleError(w, err)
		return
	}

	response.SuccessCreated(w, version)
}

// getVersion retrieves a specific version.
func (h *Handlers) getVersion(w http.ResponseWriter, r *http.Request, appID, env, versionStr string) {
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		response.BadRequest(w, "version must be a number")
		return
	}

	ver, err := h.VersionService.GetVersion(r.Context(), appID, env, version)
	if err != nil {
		handleError(w, err)
		return
	}

	response.Success(w, ver)
}
