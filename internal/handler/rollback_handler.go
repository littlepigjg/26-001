package handler

import (
	"net/http"
	"strconv"

	"config-center/pkg/response"
)

// RollbackRequest is the request body for a rollback operation.
type RollbackRequest struct {
	AppID         string `json:"app_id"`
	Environment   string `json:"environment"`
	TargetVersion int    `json:"target_version"`
}

// Rollback handles POST /api/rollback
// It restores configuration to a specified historical version.
func (h *Handlers) Rollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, "method not allowed")
		return
	}

	var req RollbackRequest
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
	if req.TargetVersion <= 0 {
		response.BadRequest(w, "target_version must be a positive integer")
		return
	}

	user := getCurrentUser(r)
	result, err := h.RollbackService.Rollback(r.Context(), req.AppID, req.Environment, req.TargetVersion, user, getClientIP(r))
	if err != nil {
		HandleError(w, err)
		return
	}

	response.Success(w, result)
}

// RollbackPreview handles GET /api/rollback/preview?app_id=X&env=Y&version=N
// It shows what would change without performing the rollback.
func (h *Handlers) RollbackPreview(w http.ResponseWriter, r *http.Request) {
	appID := parseQueryString(r, "app_id", "")
	env := parseQueryString(r, "environment", "")
	versionStr := parseQueryString(r, "version", "")

	if appID == "" {
		response.BadRequest(w, "app_id is required")
		return
	}
	if env == "" {
		response.BadRequest(w, "environment is required")
		return
	}
	if versionStr == "" {
		response.BadRequest(w, "version is required")
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		response.BadRequest(w, "version must be a number")
		return
	}

	changes, err := h.RollbackService.GetRollbackPreview(r.Context(), appID, env, version)
	if err != nil {
		HandleError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"app_id":       appID,
		"environment":  env,
		"target_version": version,
		"changes":      changes,
	})
}
