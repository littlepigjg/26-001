package handler

import (
	"net/http"

	"config-center/pkg/response"
)

// CreateConfigRequest is the request body for creating a config item.
type CreateConfigRequest struct {
	AppID       string `json:"app_id"`
	Environment string `json:"environment"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Format      string `json:"format"`
}

// UpdateConfigRequest is the request body for updating a config item.
type UpdateConfigRequest struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// BatchUpdateConfigRequest is the request body for batch updating configs.
type BatchUpdateConfigRequest struct {
	AppID       string                  `json:"app_id"`
	Environment string                  `json:"environment"`
	Configs     []ConfigItemRequest     `json:"configs"`
}

// ConfigItemRequest represents a single config item in batch operations.
type ConfigItemRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ConfigByKey handles GET/PUT/DELETE /api/configs/{app_id}/{env}/{key}
func (h *Handlers) ConfigByKey(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/api/configs/"
	rest := path[len(prefix):]

	// Parse path: app_id/env/key
	parts := splitPath(rest)
	if len(parts) < 3 {
		response.BadRequest(w, "path must be /api/configs/{app_id}/{env}/{key}")
		return
	}

	appID := parts[0]
	env := parts[1]
	key := parts[2]

	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r, appID, env, key)
	case http.MethodPut:
		h.updateConfig(w, r, appID, env, key)
	case http.MethodDelete:
		h.deleteConfig(w, r, appID, env, key)
	default:
		response.MethodNotAllowed(w, "method not allowed")
	}
}

// ListConfigs handles GET /api/configs and POST /api/configs.
func (h *Handlers) ListConfigs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listConfigs(w, r)
	case http.MethodPost:
		h.createConfig(w, r)
	default:
		response.MethodNotAllowed(w, "method not allowed")
	}
}

// listConfigs returns config items for an app and environment.
func (h *Handlers) listConfigs(w http.ResponseWriter, r *http.Request) {
	appID := parseQueryString(r, "app_id", "")
	env := parseQueryString(r, "environment", "")
	keyword := parseQueryString(r, "keyword", "")

	if appID == "" {
		response.BadRequest(w, "app_id query parameter is required")
		return
	}
	if env == "" {
		response.BadRequest(w, "environment query parameter is required")
		return
	}

	var configs interface{}
	var err error

	if keyword != "" {
		configs, err = h.ConfigService.SearchConfigs(r.Context(), appID, env, keyword)
	} else {
		configs, err = h.ConfigService.ListConfigs(r.Context(), appID, env)
	}

	if err != nil {
		HandleError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"app_id":       appID,
		"environment":  env,
		"items":        configs,
	})
}

// createConfig creates a new configuration item.
func (h *Handlers) createConfig(w http.ResponseWriter, r *http.Request) {
	var req CreateConfigRequest
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
	if req.Key == "" {
		response.BadRequest(w, "key is required")
		return
	}

	user := getCurrentUser(r)
	config, err := h.ConfigService.CreateConfig(r.Context(), req.AppID, req.Environment, req.Key, req.Value, req.Description, req.Format, user)
	if err != nil {
		HandleError(w, err)
		return
	}

	// Log and auto-create version
	_ = h.AuditService.LogConfigCreate(r.Context(), req.AppID, req.Environment, req.Key, user, getClientIP(r))
	_, _, _ = h.VersionService.AutoSnapshot(r.Context(), req.AppID, req.Environment, user)

	response.SuccessCreated(w, config)
}

// getConfig returns a single configuration item.
func (h *Handlers) getConfig(w http.ResponseWriter, r *http.Request, appID, env, key string) {
	config, err := h.ConfigService.GetConfig(r.Context(), appID, env, key)
	if err != nil {
		HandleError(w, err)
		return
	}

	response.Success(w, config)
}

// updateConfig updates an existing configuration item.
func (h *Handlers) updateConfig(w http.ResponseWriter, r *http.Request, appID, env, key string) {
	var req UpdateConfigRequest
	if err := readJSONBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	user := getCurrentUser(r)
	config, err := h.ConfigService.UpdateConfig(r.Context(), appID, env, key, req.Value, req.Description, user)
	if err != nil {
		HandleError(w, err)
		return
	}

	// Log change and auto-create version
	_ = h.AuditService.LogConfigChange(r.Context(), appID, env, key, user, getClientIP(r), "", config.Value)
	_, _, _ = h.VersionService.AutoSnapshot(r.Context(), appID, env, user)

	response.Success(w, config)
}

// deleteConfig deletes a configuration item.
func (h *Handlers) deleteConfig(w http.ResponseWriter, r *http.Request, appID, env, key string) {
	user := getCurrentUser(r)
	if err := h.ConfigService.DeleteConfig(r.Context(), appID, env, key); err != nil {
		HandleError(w, err)
		return
	}

	_ = h.AuditService.LogConfigDelete(r.Context(), appID, env, key, user, getClientIP(r))
	_, _, _ = h.VersionService.AutoSnapshot(r.Context(), appID, env, user)

	response.SuccessNoContent(w)
}

// splitPath splits a path string by "/".
func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, ch := range path {
		if ch == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
