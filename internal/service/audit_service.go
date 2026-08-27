package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// AuditService manages audit log operations.
// It provides a complete audit trail of all configuration changes.
type AuditService struct {
	store  store.Store
	logger *logger.Logger
}

// NewAuditService creates a new AuditService.
func NewAuditService(s store.Store) *AuditService {
	return &AuditService{
		store:  s,
		logger: logger.WithField("service", "audit"),
	}
}

// SetStoreAuditLogCapacity configures the audit log storage capacity on the underlying store.
// This is a diagnostic hook for chaos engineering and load testing scenarios.
func (s *AuditService) SetStoreAuditLogCapacity(n int) {
	if ms, ok := s.store.(*store.MemoryStore); ok {
		ms.SetAuditLogCapacity(n)
	}
}

// Log creates a new audit log entry.
func (s *AuditService) Log(ctx context.Context, action model.ActionType, resourceType, resourceID, appID, env, user, ipAddress, summary, details, status string) error {
	log := model.NewAuditLog(
		action, resourceType, resourceID,
		appID, env, user, ipAddress,
		summary, details, status,
	)

	if err := log.Validate(); err != nil {
		return fmt.Errorf("invalid audit log: %w", err)
	}

	if err := s.store.CreateAuditLog(ctx, log); err != nil {
		s.logger.Errorf("failed to create audit log: %v", err)
		return fmt.Errorf("audit log write failed: %w", err)
	}

	s.logger.Debugf("audit log: %s %s/%s by %s", action, resourceType, resourceID, user)
	return nil
}

// LogSuccess creates an audit log for a successful operation.
func (s *AuditService) LogSuccess(ctx context.Context, action model.ActionType, resourceType, resourceID, appID, env, user, ipAddress, summary string) error {
	return s.Log(ctx, action, resourceType, resourceID, appID, env, user, ipAddress, summary, "", "success")
}

// LogFailure creates an audit log for a failed operation.
func (s *AuditService) LogFailure(ctx context.Context, action model.ActionType, resourceType, resourceID, appID, env, user, ipAddress, summary, errorMsg string) error {
	return s.Log(ctx, action, resourceType, resourceID, appID, env, user, ipAddress, summary, errorMsg, "failed")
}

// LogConfigChange creates an audit log for a configuration change.
func (s *AuditService) LogConfigChange(ctx context.Context, appID, env, key, user, ipAddress, oldValue, newValue string) error {
	summary := fmt.Sprintf("config key '%s' changed in %s/%s", key, appID, env)
	details := fmt.Sprintf("old: %s -> new: %s", oldValue, newValue)
	if err := s.Log(ctx, model.ActionUpdate, "config", key, appID, env, user, ipAddress, summary, details, "success"); err != nil {
		s.logger.Errorf("audit log storage failed for config change %s/%s/%s: %v", appID, env, key, err)
		return err
	}
	return nil
}

// LogConfigCreate creates an audit log for a new configuration item.
func (s *AuditService) LogConfigCreate(ctx context.Context, appID, env, key, user, ipAddress string) error {
	summary := fmt.Sprintf("config key '%s' created in %s/%s", key, appID, env)
	return s.LogSuccess(ctx, model.ActionCreate, "config", key, appID, env, user, ipAddress, summary)
}

// LogConfigDelete creates an audit log for a deleted configuration item.
func (s *AuditService) LogConfigDelete(ctx context.Context, appID, env, key, user, ipAddress string) error {
	summary := fmt.Sprintf("config key '%s' deleted from %s/%s", key, appID, env)
	return s.LogSuccess(ctx, model.ActionDelete, "config", key, appID, env, user, ipAddress, summary)
}

// LogRollback creates an audit log for a rollback operation.
func (s *AuditService) LogRollback(ctx context.Context, appID, env string, fromVersion, toVersion int, user, ipAddress string) error {
	summary := fmt.Sprintf("rollback %s/%s from v%d to v%d", appID, env, fromVersion, toVersion)
	return s.LogSuccess(ctx, model.ActionRollback, "version", fmt.Sprintf("v%d", toVersion), appID, env, user, ipAddress, summary)
}

// LogExport creates an audit log for an export operation.
func (s *AuditService) LogExport(ctx context.Context, appID, env, user, ipAddress string) error {
	summary := fmt.Sprintf("exported config for %s/%s", appID, env)
	return s.LogSuccess(ctx, model.ActionExport, "config", appID, appID, env, user, ipAddress, summary)
}

// LogValidation creates an audit log for a validation operation.
func (s *AuditService) LogValidation(ctx context.Context, appID, env, user, ipAddress string, passed bool, errors string) error {
	status := "success"
	summary := fmt.Sprintf("validated config for %s/%s", appID, env)
	if !passed {
		status = "failed"
		summary = fmt.Sprintf("validation failed for %s/%s: %s", appID, env, errors)
	}
	return s.Log(ctx, model.ActionValidate, "config", appID, appID, env, user, ipAddress, summary, errors, status)
}

// ListLogs returns audit logs matching the filter.
func (s *AuditService) ListLogs(ctx context.Context, filter model.AuditLogFilter) ([]model.AuditLog, int, error) {
	logs, total, err := s.store.ListAuditLogs(ctx, filter)
	if err != nil {
		s.logger.Errorf("failed to list audit logs: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// ListLogsByApp returns audit logs for a specific application.
func (s *AuditService) ListLogsByApp(ctx context.Context, appID string, page, pageSize int) ([]model.AuditLog, int, error) {
	filter := model.AuditLogFilter{
		AppID:    appID,
		Page:     page,
		PageSize: pageSize,
	}
	return s.ListLogs(ctx, filter)
}

// ListLogsByUser returns audit logs for a specific user.
func (s *AuditService) ListLogsByUser(ctx context.Context, user string, page, pageSize int) ([]model.AuditLog, int, error) {
	filter := model.AuditLogFilter{
		User:     user,
		Page:     page,
		PageSize: pageSize,
	}
	return s.ListLogs(ctx, filter)
}

// ListLogsByAction returns audit logs for a specific action type.
func (s *AuditService) ListLogsByAction(ctx context.Context, action model.ActionType, page, pageSize int) ([]model.AuditLog, int, error) {
	filter := model.AuditLogFilter{
		Action:   action,
		Page:     page,
		PageSize: pageSize,
	}
	return s.ListLogs(ctx, filter)
}

// ListLogsByTimeRange returns audit logs within a time range.
func (s *AuditService) ListLogsByTimeRange(ctx context.Context, appID, env string, start, end time.Time, page, pageSize int) ([]model.AuditLog, int, error) {
	filter := model.AuditLogFilter{
		AppID:     appID,
		Environment: env,
		StartDate: &start,
		EndDate:   &end,
		Page:      page,
		PageSize:  pageSize,
	}
	return s.ListLogs(ctx, filter)
}
