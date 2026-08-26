package handler

import (
	"net/http"
	"strconv"

	"config-center/pkg/response"
)

// DiffConfig handles GET /api/diff
// It compares two versions of configuration and returns the differences.
func (h *Handlers) DiffConfig(w http.ResponseWriter, r *http.Request) {
	appID := parseQueryString(r, "app_id", "")
	env := parseQueryString(r, "environment", "")
	v1Str := parseQueryString(r, "v1", "")
	v2Str := parseQueryString(r, "v2", "")

	if appID == "" {
		response.BadRequest(w, "app_id is required")
		return
	}
	if env == "" {
		response.BadRequest(w, "environment is required")
		return
	}
	if v1Str == "" {
		response.BadRequest(w, "v1 (source version) is required")
		return
	}
	if v2Str == "" {
		response.BadRequest(w, "v2 (target version) is required")
		return
	}

	v1, err := strconv.Atoi(v1Str)
	if err != nil {
		response.BadRequest(w, "v1 must be a number")
		return
	}
	v2, err := strconv.Atoi(v2Str)
	if err != nil {
		response.BadRequest(w, "v2 must be a number")
		return
	}

	result, err := h.DiffService.DiffVersions(r.Context(), appID, env, v1, v2)
	if err != nil {
		handleError(w, err)
		return
	}

	response.Success(w, result)
}
