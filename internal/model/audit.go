package model

import (
	"fmt"
	"time"
)

// ActionType represents the type of action performed in an audit log.
type ActionType string

const (
	// ActionCreate indicates a creation action.
	ActionCreate ActionType = "CREATE"
	// ActionUpdate indicates an update action.
	ActionUpdate ActionType = "UPDATE"
	// ActionDelete indicates a deletion action.
	ActionDelete ActionType = "DELETE"
	// ActionRollback indicates a rollback action.
	ActionRollback ActionType = "ROLLBACK"
	// ActionExport indicates an export action.
	ActionExport ActionType = "EXPORT"
	// ActionImport indicates an import action.
	ActionImport ActionType = "IMPORT"
	// ActionLogin indicates a login action.
	ActionLogin ActionType = "LOGIN"
	// ActionValidate indicates a validation action.
	ActionValidate ActionType = "VALIDATE"
)

// Valid checks if the action type is a known value.
func (a ActionType) Valid() bool {
	switch a {
	case ActionCreate, ActionUpdate, ActionDelete, ActionRollback,
		ActionExport, ActionImport, ActionLogin, ActionValidate:
		return true
	default:
		return false
	}
}

// AuditLog records an operation performed on the configuration center.
// It provides a complete audit trail for compliance and debugging.
type AuditLog struct {
	// ID is the unique identifier for this audit log.
	ID string `json:"id"`
	// Action is the type of action performed.
	Action ActionType `json:"action"`
	// ResourceType is the type of resource affected (e.g., "app", "config", "version").
	ResourceType string `json:"resource_type"`
	// ResourceID is the identifier of the affected resource.
	ResourceID string `json:"resource_id"`
	// AppID is the application context.
	AppID string `json:"app_id"`
	// Environment is the environment context.
	Environment string `json:"environment"`
	// User is the user who performed the action.
	User string `json:"user"`
	// IPAddress is the IP address of the user.
	IPAddress string `json:"ip_address"`
	// Summary provides a brief description of the action.
	Summary string `json:"summary"`
	// Details contains additional details about the action (JSON string).
	Details string `json:"details"`
	// Status is the outcome of the action ("success", "failed").
	Status string `json:"status"`
	// ErrorMessage contains the error message if the action failed.
	ErrorMessage string `json:"error_message,omitempty"`
	// CreatedAt records when the action was performed.
	CreatedAt time.Time `json:"created_at"`
}

// NewAuditLog creates a new AuditLog.
func NewAuditLog(action ActionType, resourceType, resourceID, appID, env, user, ipAddress, summary, details, status string) *AuditLog {
	return &AuditLog{
		ID:           fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		AppID:        appID,
		Environment:  env,
		User:         user,
		IPAddress:    ipAddress,
		Summary:      summary,
		Details:      details,
		Status:       status,
		CreatedAt:    time.Now(),
	}
}

// Validate checks if the audit log fields are valid.
func (a *AuditLog) Validate() error {
	if !a.Action.Valid() {
		return fmt.Errorf("invalid action type: %s", a.Action)
	}
	if a.User == "" {
		return fmt.Errorf("user is required")
	}
	if a.ResourceType == "" {
		return fmt.Errorf("resource_type is required")
	}
	return nil
}

// AuditLogFilter provides filtering options for querying audit logs.
type AuditLogFilter struct {
	// AppID filters by application.
	AppID string `json:"app_id,omitempty"`
	// Environment filters by environment.
	Environment string `json:"environment,omitempty"`
	// Action filters by action type.
	Action ActionType `json:"action,omitempty"`
	// User filters by user.
	User string `json:"user,omitempty"`
	// ResourceType filters by resource type.
	ResourceType string `json:"resource_type,omitempty"`
	// StartDate filters logs created after this time.
	StartDate *time.Time `json:"start_date,omitempty"`
	// EndDate filters logs created before this time.
	EndDate *time.Time `json:"end_date,omitempty"`
	// Page is the page number for pagination (1-based).
	Page int `json:"page"`
	// PageSize is the number of items per page.
	PageSize int `json:"page_size"`
}

// AuditLogList is a paginated list of audit logs.
type AuditLogList struct {
	// Items is the list of audit logs.
	Items []AuditLog `json:"items"`
	// Total is the total number of matching logs.
	Total int `json:"total"`
	// Page is the current page number.
	Page int `json:"page"`
	// PageSize is the page size.
	PageSize int `json:"page_size"`
}
