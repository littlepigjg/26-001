package handler

import (
	"net/http"

	"config-center/internal/service"
	"config-center/pkg/response"
)

// ClientPull handles GET /api/client/pull?app_id=X&environment=Y&version=Z
// It returns the configuration for a client, supporting version-based caching.
func (h *Handlers) ClientPull(w http.ResponseWriter, r *http.Request) {
	appID := parseQueryString(r, "app_id", "")
	env := parseQueryString(r, "environment", "")
	clientVersion := parseQueryString(r, "version", "")

	if appID == "" {
		response.BadRequest(w, "app_id query parameter is required")
		return
	}
	if env == "" {
		response.BadRequest(w, "environment query parameter is required")
		return
	}

	result, err := h.ClientService.PullConfig(r.Context(), appID, env, clientVersion)
	if err != nil {
		HandleError(w, err)
		return
	}

	// If not modified, return 304
	if result.NotModified {
		w.Header().Set("ETag", result.ETag)
		response.SuccessNotModified(w)
		return
	}

	// Set ETag header for caching
	w.Header().Set("ETag", result.ETag)
	w.Header().Set("Cache-Control", "public, max-age=30")

	response.Success(w, map[string]interface{}{
		"app_id":       result.AppID,
		"environment":  result.Environment,
		"version":      result.Version,
		"etag":         result.ETag,
		"config":       result.Config,
		"updated_at":   result.UpdatedAt,
	})
}

// BatchPullRequest is the request body for batch config pulls.
type BatchPullRequest struct {
	Requests []BatchPullItem `json:"requests"`
}

// BatchPullItem represents a single item in a batch pull.
type BatchPullItem struct {
	AppID       string `json:"app_id"`
	Environment string `json:"environment"`
	Version     string `json:"version"`
}

// ClientBatchPull handles POST /api/client/batch-pull
// It allows a client to pull configurations for multiple apps/environments in one request.
func (h *Handlers) ClientBatchPull(w http.ResponseWriter, r *http.Request) {
	var req BatchPullRequest
	if err := readJSONBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	if len(req.Requests) == 0 {
		response.BadRequest(w, "at least one request item is required")
		return
	}

	// Build pull requests
	pullRequests := make([]service.PullRequest, len(req.Requests))
	for i, item := range req.Requests {
		pullRequests[i] = service.PullRequest{
			AppID:       item.AppID,
			Environment: item.Environment,
			Version:     item.Version,
		}
	}

	results, err := h.ClientService.BatchPullConfig(r.Context(), pullRequests)
	if err != nil {
		HandleError(w, err)
		return
	}

	// Build response
	responseItems := make([]map[string]interface{}, len(results))
	for i, result := range results {
		item := map[string]interface{}{
			"app_id":       result.AppID,
			"environment":  result.Environment,
			"modified":     result.Modified,
			"not_modified": result.NotModified,
			"version":      result.Version,
			"etag":         result.ETag,
		}
		if result.Config != nil {
			item["config"] = result.Config
		}
		responseItems[i] = item
	}

	response.Success(w, map[string]interface{}{
		"results": responseItems,
	})
}
