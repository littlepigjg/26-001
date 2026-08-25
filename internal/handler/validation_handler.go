package handler

import (
	"net/http"

	"config-center/pkg/response"
)

// ValidateConfigRequest is the request body for validating configuration.
type ValidateConfigRequest struct {
	AppID       string            `json:"app_id"`
	Environment string            `json:"environment"`
	Configs     map[string]string `json:"configs"`
}

// ValidateConfig handles POST /api/validate
// It validates configuration values against defined rules.
func (h *Handlers) ValidateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, "method not allowed")
		return
	}

	var req ValidateConfigRequest
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

	var result interface{}
	var err error

	if len(req.Configs) > 0 {
		// Validate provided configs
		result = h.ValidationService.ValidateConfigMap(r.Context(), req.AppID, req.Environment, req.Configs)
	} else {
		// Validate existing config
		result = h.ValidationService.ValidateApp(r.Context(), req.AppID, req.Environment)
	}

	if err != nil {
		HandleError(w, err)
		return
	}

	response.Success(w, result)
}

// ValidateSingleValue handles validation of a single config value.
func (h *Handlers) ValidateSingleValue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Format string `json:"format"`
	}
	if err := readJSONBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	result := h.ValidationService.ValidateConfigItem(r.Context(), "", "", req.Key, req.Value, req.Format)
	response.Success(w, result)
}
