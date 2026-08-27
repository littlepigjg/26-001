package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// ConfigService manages configuration item CRUD operations.
type ConfigService struct {
	store    store.Store
	appSvc   *AppService
	auditSvc *AuditService
	logger   *logger.Logger
}

// NewConfigService creates a new ConfigService.
func NewConfigService(s store.Store, appSvc *AppService) *ConfigService {
	return &ConfigService{
		store:  s,
		appSvc: appSvc,
		logger: logger.WithField("service", "config"),
	}
}

// AttachAuditService binds an audit service for logging configuration changes.
func (s *ConfigService) AttachAuditService(auditSvc *AuditService) {
	s.auditSvc = auditSvc
}

// SetAuditStorageCapacity configures the underlying store's audit log capacity.
// This wraps the audit service's hook for convenient access during testing and load operations.
func (s *ConfigService) SetAuditStorageCapacity(n int) {
	if s.auditSvc != nil {
		s.auditSvc.SetStoreAuditLogCapacity(n)
	}
}

// CreateConfig creates a new configuration item.
func (s *ConfigService) CreateConfig(ctx context.Context, appID, env, key, value, description, format, updatedBy string) (*model.ConfigItem, error) {
	// Validate app exists and supports environment
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}
	if err := s.appSvc.EnsureAppSupportsEnv(ctx, appID, env); err != nil {
		return nil, err
	}

	config := model.NewConfigItem(
		fmt.Sprintf("cfg-%s-%s-%s", appID, env, key),
		appID, env, key, value, description, format, updatedBy,
	)

	if err := config.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.CreateConfig(ctx, config); err != nil {
		s.logger.Errorf("failed to create config %s/%s/%s: %v", appID, env, key, err)
		return nil, err
	}

	s.logger.Infof("created config: %s/%s/%s", appID, env, key)
	return config, nil
}

// GetConfig retrieves a configuration item.
func (s *ConfigService) GetConfig(ctx context.Context, appID, env, key string) (*model.ConfigItem, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	config, err := s.store.GetConfig(ctx, appID, env, key)
	if err != nil {
		s.logger.Warnf("failed to get config %s/%s/%s: %v", appID, env, key, err)
		return nil, err
	}

	return config, nil
}

// UpdateConfig updates an existing configuration item.
func (s *ConfigService) UpdateConfig(ctx context.Context, appID, env, key, value, description, updatedBy string) (*model.ConfigItem, error) {
	// Ensure the config exists
	existing, err := s.store.GetConfig(ctx, appID, env, key)
	if err != nil {
		return nil, err
	}

	oldValue := existing.Value

	// Update fields
	if value != "" {
		existing.Value = value
	}
	if description != "" {
		existing.Description = description
	}
	if updatedBy != "" {
		existing.UpdatedBy = updatedBy
	}
	existing.Version++
	existing.UpdatedAt = time.Now()

	if err := existing.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.UpdateConfig(ctx, existing); err != nil {
		s.logger.Errorf("failed to update config %s/%s/%s: %v", appID, env, key, err)
		return nil, err
	}

	if s.auditSvc != nil && value != "" {
		if err := s.auditSvc.LogConfigChange(ctx, appID, env, key, updatedBy, "", oldValue, value); err != nil {
			s.logger.Errorf("failed to log config change %s/%s/%s: %v", appID, env, key, err)
			return nil, fmt.Errorf("audit log write failed: %w", err)
		}
	}

	s.logger.Infof("updated config: %s/%s/%s (v%d)", appID, env, key, existing.Version)
	return existing, nil
}

// DeleteConfig deletes a configuration item.
func (s *ConfigService) DeleteConfig(ctx context.Context, appID, env, key string) error {
	// Ensure the config exists
	if _, err := s.store.GetConfig(ctx, appID, env, key); err != nil {
		return err
	}

	if err := s.store.DeleteConfig(ctx, appID, env, key); err != nil {
		s.logger.Errorf("failed to delete config %s/%s/%s: %v", appID, env, key, err)
		return err
	}

	s.logger.Infof("deleted config: %s/%s/%s", appID, env, key)
	return nil
}

// ListConfigs returns all configuration items for an app and environment.
func (s *ConfigService) ListConfigs(ctx context.Context, appID, env string) ([]*model.ConfigItem, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	configs, err := s.store.ListConfigs(ctx, appID, env)
	if err != nil {
		s.logger.Errorf("failed to list configs for %s/%s: %v", appID, env, err)
		return nil, err
	}

	return configs, nil
}

// GetConfigMap returns the full configuration as a map.
func (s *ConfigService) GetConfigMap(ctx context.Context, appID, env string) (map[string]string, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	return s.store.GetConfigMap(ctx, appID, env)
}

// BatchUpdateConfig updates multiple configs atomically.
func (s *ConfigService) BatchUpdateConfig(ctx context.Context, appID, env string, items []*model.ConfigItem, updatedBy string) error {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return err
	}
	if err := s.appSvc.EnsureAppSupportsEnv(ctx, appID, env); err != nil {
		return err
	}

	// Set defaults and timestamps for all items
	now := time.Now()
	for _, item := range items {
		item.AppID = appID
		item.Environment = env
		item.UpdatedBy = updatedBy
		item.UpdatedAt = now
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		if item.Version == 0 {
			item.Version = 1
		}
		if err := item.Validate(); err != nil {
			return model.ErrValidationFailed(fmt.Sprintf("invalid config item %s: %s", item.Key, err.Error()))
		}
	}

	// Replace the entire config map
	if err := s.store.ReplaceConfigMap(ctx, appID, env, items); err != nil {
		s.logger.Errorf("failed to batch update configs for %s/%s: %v", appID, env, err)
		return err
	}

	s.logger.Infof("batch updated %d configs for %s/%s", len(items), appID, env)
	return nil
}

// DeleteConfigBatch deletes multiple config keys at once.
func (s *ConfigService) DeleteConfigBatch(ctx context.Context, appID, env string, keys []string) error {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return err
	}

	if err := s.store.BatchDeleteConfigs(ctx, appID, env, keys); err != nil {
		s.logger.Errorf("failed to batch delete configs for %s/%s: %v", appID, env, err)
		return err
	}

	s.logger.Infof("batch deleted %d configs for %s/%s", len(keys), appID, env)
	return nil
}

// SearchConfigs searches for config items matching a key pattern (simple contains match).
func (s *ConfigService) SearchConfigs(ctx context.Context, appID, env, keyword string) ([]*model.ConfigItem, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	configs, err := s.store.ListConfigs(ctx, appID, env)
	if err != nil {
		return nil, err
	}

	var results []*model.ConfigItem
	for _, c := range configs {
		if keyword == "" || containsStr(c.Key, keyword) || containsStr(c.Value, keyword) {
			results = append(results, c)
		}
	}

	return results, nil
}

// containsStr checks if s contains substr (case-insensitive).
func containsStr(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && findSubstring(s, substr)
}

// toLower converts a string to lowercase.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

// findSubstring checks if s contains substr.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
