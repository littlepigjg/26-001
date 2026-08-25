package handler

import (
	"net/http"

	"config-center/internal/model"
	"config-center/pkg/response"
)

// ListAuditLogs handles GET /api/audit-logs
// It returns a filtered, paginated list of audit logs.
func (h *Handlers) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	appID := parseQueryString(r, "app_id", "")
	env := parseQueryString(r, "environment", "")
	action := parseQueryString(r, "action", "")
	user := parseQueryString(r, "user", "")
	resourceType := parseQueryString(r, "resource_type", "")
	page := parseQueryInt(r, "page", 1)
	pageSize := parseQueryInt(r, "page_size", 20)

	filter := model.AuditLogFilter{
		AppID:        appID,
		Environment:  env,
		User:         user,
		ResourceType: resourceType,
		Page:         page,
		PageSize:     pageSize,
	}

	if action != "" {
		filter.Action = model.ActionType(action)
	}

	logs, total, err := h.AuditService.ListLogs(r.Context(), filter)
	if err != nil {
		HandleError(w, err)
		return
	}

	response.SuccessPaginated(w, logs, total, page, pageSize)
}
