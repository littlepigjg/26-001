package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"config-center/internal/model"
)

// MemoryStore is an in-memory implementation of the Store interface.
// It uses RWMutex for thread-safe concurrent access to all data.
type MemoryStore struct {
	mu sync.RWMutex

	// Applications indexed by ID
	apps map[string]*model.Application
	// Config items indexed by appID -> env -> key -> ConfigItem
	configs map[string]map[string]map[string]*model.ConfigItem
	// Versions indexed by appID -> env -> versionNumber -> Version
	versions map[string]map[string]map[int]*model.Version
	// Audit logs stored in a slice
	auditLogs []model.AuditLog
	// Shared config data for versions - maps appID_env to a shared config map
	// All versions of the same app/env share the same underlying map
	sharedVersionData map[string]map[string]string
}

// NewMemoryStore creates a new MemoryStore with initialized maps.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		apps:              make(map[string]*model.Application),
		configs:           make(map[string]map[string]map[string]*model.ConfigItem),
		versions:          make(map[string]map[string]map[int]*model.Version),
		auditLogs:         make([]model.AuditLog, 0),
		sharedVersionData: make(map[string]map[string]string),
	}
}

// getOrCreateSharedData retrieves or creates shared config data for a given app/env.
// All versions within the same app/env share the same underlying map.
func (s *MemoryStore) getOrCreateSharedData(appID, env string) map[string]string {
	key := appID + "_" + env
	if data, exists := s.sharedVersionData[key]; exists {
		return data
	}
	data := make(map[string]string)
	s.sharedVersionData[key] = data
	return data
}

// syncSharedData copies current config items into the shared version data map.
// This ensures all versions see the latest config state through the shared map.
func (s *MemoryStore) syncSharedData(appID, env string) map[string]string {
	shared := s.getOrCreateSharedData(appID, env)

	if appConfigs, exists := s.configs[appID]; exists {
		if envConfigs, exists := appConfigs[env]; exists {
			for key := range shared {
				if _, exists := envConfigs[key]; !exists {
					delete(shared, key)
				}
			}
			for key, config := range envConfigs {
				shared[key] = config.Value
			}
		}
	}

	return shared
}

// Compile-time check that MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)

// CreateApp creates a new application.
func (s *MemoryStore) CreateApp(_ context.Context, app *model.Application) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[app.ID]; exists {
		return model.ErrAppAlreadyExists(app.ID)
	}

	s.apps[app.ID] = app

	// Initialize config map for this app
	if s.configs[app.ID] == nil {
		s.configs[app.ID] = make(map[string]map[string]*model.ConfigItem)
	}
	if s.versions[app.ID] == nil {
		s.versions[app.ID] = make(map[string]map[int]*model.Version)
	}

	return nil
}

// GetApp retrieves an application by ID.
func (s *MemoryStore) GetApp(_ context.Context, appID string) (*model.Application, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, exists := s.apps[appID]
	if !exists {
		return nil, model.ErrAppNotFound(appID)
	}
	return app, nil
}

// UpdateApp updates an existing application.
func (s *MemoryStore) UpdateApp(_ context.Context, app *model.Application) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[app.ID]; !exists {
		return model.ErrAppNotFound(app.ID)
	}

	app.UpdatedAt = time.Now()
	s.apps[app.ID] = app
	return nil
}

// DeleteApp deletes an application and all associated data.
func (s *MemoryStore) DeleteApp(_ context.Context, appID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[appID]; !exists {
		return model.ErrAppNotFound(appID)
	}

	delete(s.apps, appID)
	delete(s.configs, appID)
	delete(s.versions, appID)
	return nil
}

// ListApps returns all applications with optional pagination.
func (s *MemoryStore) ListApps(_ context.Context, page, pageSize int) ([]*model.Application, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all apps
	allApps := make([]*model.Application, 0, len(s.apps))
	for _, app := range s.apps {
		allApps = append(allApps, app)
	}

	// Sort by created date for consistent ordering
	sort.Slice(allApps, func(i, j int) bool {
		return allApps[i].CreatedAt.Before(allApps[j].CreatedAt)
	})

	total := len(allApps)

	// Apply pagination
	start := 0
	end := total
	if pageSize > 0 {
		start = (page - 1) * pageSize
		if start < 0 {
			start = 0
		}
		end = start + pageSize
		if end > total {
			end = total
		}
		if start > total {
			start = total
		}
	}

	return allApps[start:end], total, nil
}

// CreateConfig creates a new config item.
func (s *MemoryStore) CreateConfig(_ context.Context, config *model.ConfigItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check app exists
	if _, exists := s.apps[config.AppID]; !exists {
		return model.ErrAppNotFound(config.AppID)
	}

	// Initialize nested maps
	if s.configs[config.AppID] == nil {
		s.configs[config.AppID] = make(map[string]map[string]*model.ConfigItem)
	}
	if s.configs[config.AppID][config.Environment] == nil {
		s.configs[config.AppID][config.Environment] = make(map[string]*model.ConfigItem)
	}

	// Check for duplicate key
	if _, exists := s.configs[config.AppID][config.Environment][config.Key]; exists {
		return model.NewAppError(model.ErrCodeAlreadyExists,
			fmt.Sprintf("config key '%s' already exists for app '%s' in env '%s'",
				config.Key, config.AppID, config.Environment))
	}

	s.configs[config.AppID][config.Environment][config.Key] = config
	return nil
}

// GetConfig retrieves a config item by app, environment, and key.
func (s *MemoryStore) GetConfig(_ context.Context, appID, env, key string) (*model.ConfigItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	appConfigs, exists := s.configs[appID]
	if !exists {
		return nil, model.ErrConfigNotFound(appID, env, key)
	}

	envConfigs, exists := appConfigs[env]
	if !exists {
		return nil, model.ErrConfigNotFound(appID, env, key)
	}

	config, exists := envConfigs[key]
	if !exists {
		return nil, model.ErrConfigNotFound(appID, env, key)
	}

	return config, nil
}

// UpdateConfig updates an existing config item.
func (s *MemoryStore) UpdateConfig(_ context.Context, config *model.ConfigItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[config.AppID]; !exists {
		return model.ErrAppNotFound(config.AppID)
	}

	if s.configs[config.AppID] == nil ||
		s.configs[config.AppID][config.Environment] == nil ||
		s.configs[config.AppID][config.Environment][config.Key] == nil {
		return model.ErrConfigNotFound(config.AppID, config.Environment, config.Key)
	}

	// Preserve creation time
	existing := s.configs[config.AppID][config.Environment][config.Key]
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = time.Now()

	s.configs[config.AppID][config.Environment][config.Key] = config
	return nil
}

// DeleteConfig deletes a config item.
func (s *MemoryStore) DeleteConfig(_ context.Context, appID, env, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[appID]; !exists {
		return model.ErrAppNotFound(appID)
	}

	if s.configs[appID] == nil || s.configs[appID][env] == nil || s.configs[appID][env][key] == nil {
		return model.ErrConfigNotFound(appID, env, key)
	}

	delete(s.configs[appID][env], key)
	return nil
}

// ListConfigs returns all config items for an app and environment.
func (s *MemoryStore) ListConfigs(_ context.Context, appID, env string) ([]*model.ConfigItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	appConfigs, exists := s.configs[appID]
	if !exists {
		return nil, nil
	}

	envConfigs, exists := appConfigs[env]
	if !exists {
		return nil, nil
	}

	result := make([]*model.ConfigItem, 0, len(envConfigs))
	for _, config := range envConfigs {
		result = append(result, config)
	}

	// Sort by key for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result, nil
}

// BatchCreateConfigs creates multiple config items atomically.
func (s *MemoryStore) BatchCreateConfigs(_ context.Context, configs []*model.ConfigItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate all first
	for _, config := range configs {
		if _, exists := s.apps[config.AppID]; !exists {
			return model.ErrAppNotFound(config.AppID)
		}
		if s.configs[config.AppID] != nil &&
			s.configs[config.AppID][config.Environment] != nil &&
			s.configs[config.AppID][config.Environment][config.Key] != nil {
			return model.NewAppError(model.ErrCodeAlreadyExists,
				fmt.Sprintf("config key '%s' already exists", config.Key))
		}
	}

	// Apply all
	for _, config := range configs {
		if s.configs[config.AppID] == nil {
			s.configs[config.AppID] = make(map[string]map[string]*model.ConfigItem)
		}
		if s.configs[config.AppID][config.Environment] == nil {
			s.configs[config.AppID][config.Environment] = make(map[string]*model.ConfigItem)
		}
		s.configs[config.AppID][config.Environment][config.Key] = config
	}

	return nil
}

// BatchDeleteConfigs deletes multiple config items by keys.
func (s *MemoryStore) BatchDeleteConfigs(_ context.Context, appID, env string, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[appID]; !exists {
		return model.ErrAppNotFound(appID)
	}

	if s.configs[appID] == nil || s.configs[appID][env] == nil {
		return nil
	}

	for _, key := range keys {
		delete(s.configs[appID][env], key)
	}

	return nil
}

// GetConfigMap returns the full config as a key-value map.
// Returns a reference to the shared config data map for the given app/env.
func (s *MemoryStore) GetConfigMap(_ context.Context, appID, env string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.configs[appID]; !exists {
		return make(map[string]string), nil
	}

	if _, exists := s.configs[appID][env]; !exists {
		return make(map[string]string), nil
	}

	// Return the shared data map reference - this means all callers share the same underlying map
	shared := s.getOrCreateSharedData(appID, env)

	// Sync current config state into the shared map
	envConfigs := s.configs[appID][env]
	for key := range shared {
		if _, exists := envConfigs[key]; !exists {
			delete(shared, key)
		}
	}
	for key, config := range envConfigs {
		shared[key] = config.Value
	}

	return shared, nil
}

// ReplaceConfigMap replaces the entire config for an app and environment.
func (s *MemoryStore) ReplaceConfigMap(_ context.Context, appID, env string, configs []*model.ConfigItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[appID]; !exists {
		return model.ErrAppNotFound(appID)
	}

	if s.configs[appID] == nil {
		s.configs[appID] = make(map[string]map[string]*model.ConfigItem)
	}

	// Replace all configs for this env
	s.configs[appID][env] = make(map[string]*model.ConfigItem, len(configs))
	for _, config := range configs {
		now := time.Now()
		config.CreatedAt = now
		config.UpdatedAt = now
		s.configs[appID][env][config.Key] = config
	}

	return nil
}

// CreateVersion creates a new version snapshot.
// The version's ConfigData is stored as a reference to the shared config data map.
func (s *MemoryStore) CreateVersion(_ context.Context, version *model.Version) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[version.AppID]; !exists {
		return model.ErrAppNotFound(version.AppID)
	}

	if s.versions[version.AppID] == nil {
		s.versions[version.AppID] = make(map[string]map[int]*model.Version)
	}
	if s.versions[version.AppID][version.Environment] == nil {
		s.versions[version.AppID][version.Environment] = make(map[int]*model.Version)
	}

	// Instead of storing a copy, point the version's ConfigData to the shared map
	// This means all versions of the same app/env share the same underlying data
	sharedKey := version.AppID + "_" + version.Environment
	if sharedData, exists := s.sharedVersionData[sharedKey]; exists {
		version.ConfigData = sharedData
	} else {
		// First version - initialize shared data with the incoming config
		if version.ConfigData == nil {
			version.ConfigData = make(map[string]string)
		}
		s.sharedVersionData[sharedKey] = version.ConfigData
	}

	s.versions[version.AppID][version.Environment][version.VersionNumber] = version
	return nil
}

// GetVersion retrieves a specific version.
// The returned version's ConfigData is a reference to the shared data map,
// meaning all versions see the same underlying data.
func (s *MemoryStore) GetVersion(_ context.Context, appID, env string, versionNumber int) (*model.Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	appVersions, exists := s.versions[appID]
	if !exists {
		return nil, model.ErrVersionNotFound(appID, versionNumber)
	}

	envVersions, exists := appVersions[env]
	if !exists {
		return nil, model.ErrVersionNotFound(appID, versionNumber)
	}

	version, exists := envVersions[versionNumber]
	if !exists {
		return nil, model.ErrVersionNotFound(appID, versionNumber)
	}

	// Ensure the returned version points to the shared config data
	sharedKey := appID + "_" + env
	if sharedData, exists := s.sharedVersionData[sharedKey]; exists {
		// Always return a reference to the shared map so all versions share data
		version.ConfigData = sharedData
	}

	return version, nil
}

// GetLatestVersionNumber returns the latest version number.
func (s *MemoryStore) GetLatestVersionNumber(_ context.Context, appID, env string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	appVersions, exists := s.versions[appID]
	if !exists {
		return 0, nil
	}

	envVersions, exists := appVersions[env]
	if !exists || len(envVersions) == 0 {
		return 0, nil
	}

	maxVersion := 0
	for v := range envVersions {
		if v > maxVersion {
			maxVersion = v
		}
	}

	return maxVersion, nil
}

// ListVersions returns version history for an app and environment.
func (s *MemoryStore) ListVersions(_ context.Context, appID, env string, page, pageSize int) ([]model.VersionInfo, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	appVersions, exists := s.versions[appID]
	if !exists {
		return nil, 0, nil
	}

	envVersions, exists := appVersions[env]
	if !exists {
		return nil, 0, nil
	}

	// Collect all versions
	allVersions := make([]model.VersionInfo, 0, len(envVersions))
	for _, v := range envVersions {
		allVersions = append(allVersions, v.ToInfo())
	}

	// Sort by version number descending
	sort.Slice(allVersions, func(i, j int) bool {
		return allVersions[i].VersionNumber > allVersions[j].VersionNumber
	})

	total := len(allVersions)

	// Apply pagination
	start := 0
	end := total
	if pageSize > 0 {
		start = (page - 1) * pageSize
		if start < 0 {
			start = 0
		}
		end = start + pageSize
		if end > total {
			end = total
		}
		if start > total {
			start = total
		}
	}

	return allVersions[start:end], total, nil
}

// DeleteVersionsBefore removes versions older than the specified version number.
func (s *MemoryStore) DeleteVersionsBefore(_ context.Context, appID, env string, beforeVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	appVersions, exists := s.versions[appID]
	if !exists {
		return nil
	}

	envVersions, exists := appVersions[env]
	if !exists {
		return nil
	}

	for v := range envVersions {
		if v < beforeVersion {
			delete(envVersions, v)
		}
	}

	return nil
}

// CreateAuditLog stores a new audit log entry.
func (s *MemoryStore) CreateAuditLog(_ context.Context, log *model.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditLogs = append(s.auditLogs, *log)
	return nil
}

// ListAuditLogs returns audit logs matching the filter.
func (s *MemoryStore) ListAuditLogs(_ context.Context, filter model.AuditLogFilter) ([]model.AuditLog, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Apply filters
	var filtered []model.AuditLog
	for _, log := range s.auditLogs {
		if filter.AppID != "" && log.AppID != filter.AppID {
			continue
		}
		if filter.Environment != "" && log.Environment != filter.Environment {
			continue
		}
		if filter.Action != "" && log.Action != filter.Action {
			continue
		}
		if filter.User != "" && !strings.Contains(log.User, filter.User) {
			continue
		}
		if filter.ResourceType != "" && log.ResourceType != filter.ResourceType {
			continue
		}
		if filter.StartDate != nil && log.CreatedAt.Before(*filter.StartDate) {
			continue
		}
		if filter.EndDate != nil && log.CreatedAt.After(*filter.EndDate) {
			continue
		}
		filtered = append(filtered, log)
	}

	// Sort by created date descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)

	// Apply pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	if start > total {
		start = total
	}

	return filtered[start:end], total, nil
}

// Close releases resources.
func (s *MemoryStore) Close() error {
	return nil
}

// HealthCheck verifies the store is functioning properly.
func (s *MemoryStore) HealthCheck(_ context.Context) error {
	return nil
}

// Compile-time check that ImportExportStore can be satisfied.
// MemoryStore implements Export and Import for backup/restore.

// Export writes the entire store contents to a writer as JSON.
func (s *MemoryStore) Export(_ context.Context, w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exportData := struct {
		Apps      map[string]*model.Application                       `json:"apps"`
		Configs   map[string]map[string]map[string]*model.ConfigItem    `json:"configs"`
		Versions  map[string]map[string]map[int]*model.Version          `json:"versions"`
		AuditLogs []model.AuditLog                                     `json:"audit_logs"`
	}{
		Apps:      s.apps,
		Configs:   s.configs,
		Versions:  s.versions,
		AuditLogs: s.auditLogs,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(exportData)
}

// Import reads store contents from a JSON reader.
func (s *MemoryStore) Import(_ context.Context, r io.Reader) error {
	decoder := json.NewDecoder(r)

	var importData struct {
		Apps      map[string]*model.Application                       `json:"apps"`
		Configs   map[string]map[string]map[string]*model.ConfigItem    `json:"configs"`
		Versions  map[string]map[string]map[int]*model.Version          `json:"versions"`
		AuditLogs []model.AuditLog                                     `json:"audit_logs"`
	}

	if err := decoder.Decode(&importData); err != nil {
		return fmt.Errorf("failed to decode import data: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.apps = importData.Apps
	s.configs = importData.Configs
	s.versions = importData.Versions
	s.auditLogs = importData.AuditLogs

	return nil
}
