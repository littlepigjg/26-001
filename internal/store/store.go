// Package store provides the storage layer for the configuration center.
// It defines the Store interface and provides an in-memory implementation.
package store

import (
	"context"
	"io"

	"config-center/internal/model"
)

// Store defines the interface for configuration data persistence.
// All operations are designed to be thread-safe and context-aware.
type Store interface {
	// Application operations

	// CreateApp creates a new application.
	CreateApp(ctx context.Context, app *model.Application) error
	// GetApp retrieves an application by ID.
	GetApp(ctx context.Context, appID string) (*model.Application, error)
	// UpdateApp updates an existing application.
	UpdateApp(ctx context.Context, app *model.Application) error
	// DeleteApp deletes an application.
	DeleteApp(ctx context.Context, appID string) error
	// ListApps returns all applications with optional pagination.
	ListApps(ctx context.Context, page, pageSize int) ([]*model.Application, int, error)

	// Configuration item operations

	// CreateConfig creates a new config item.
	CreateConfig(ctx context.Context, config *model.ConfigItem) error
	// GetConfig retrieves a config item by app, environment, and key.
	GetConfig(ctx context.Context, appID, env, key string) (*model.ConfigItem, error)
	// UpdateConfig updates an existing config item.
	UpdateConfig(ctx context.Context, config *model.ConfigItem) error
	// DeleteConfig deletes a config item.
	DeleteConfig(ctx context.Context, appID, env, key string) error
	// ListConfigs returns all config items for an app and environment.
	ListConfigs(ctx context.Context, appID, env string) ([]*model.ConfigItem, error)
	// BatchCreateConfigs creates multiple config items at once.
	BatchCreateConfigs(ctx context.Context, configs []*model.ConfigItem) error
	// BatchDeleteConfigs deletes multiple config items by keys.
	BatchDeleteConfigs(ctx context.Context, appID, env string, keys []string) error
	// GetConfigMap returns the full config as a key-value map.
	GetConfigMap(ctx context.Context, appID, env string) (map[string]string, error)
	// ReplaceConfigMap replaces the entire config for an app and environment.
	ReplaceConfigMap(ctx context.Context, appID, env string, configs []*model.ConfigItem) error

	// Version operations

	// CreateVersion creates a new version snapshot.
	CreateVersion(ctx context.Context, version *model.Version) error
	// GetVersion retrieves a specific version.
	GetVersion(ctx context.Context, appID, env string, versionNumber int) (*model.Version, error)
	// GetLatestVersionNumber returns the latest version number.
	GetLatestVersionNumber(ctx context.Context, appID, env string) (int, error)
	// ListVersions returns version history for an app and environment.
	ListVersions(ctx context.Context, appID, env string, page, pageSize int) ([]model.VersionInfo, int, error)
	// DeleteVersionsBefore removes versions older than the specified version number.
	DeleteVersionsBefore(ctx context.Context, appID, env string, beforeVersion int) error

	// Audit log operations

	// CreateAuditLog stores a new audit log entry.
	CreateAuditLog(ctx context.Context, log *model.AuditLog) error
	// ListAuditLogs returns audit logs matching the filter.
	ListAuditLogs(ctx context.Context, filter model.AuditLogFilter) ([]model.AuditLog, int, error)

	// Lifecycle operations

	// Close releases any resources held by the store.
	Close() error
	// HealthCheck verifies the store is functioning properly.
	HealthCheck(ctx context.Context) error
}

// ImportExportStore extends Store with import/export capabilities.
type ImportExportStore interface {
	Store
	// Export writes the entire store contents to a writer.
	Export(ctx context.Context, w io.Writer) error
	// Import reads store contents from a reader.
	Import(ctx context.Context, r io.Reader) error
}
