package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"config-center/internal/model"
	"config-center/pkg/logger"
)

// FileStore is a file-based persistent implementation of the Store interface.
// It stores all configuration data in a JSON file and supports periodic auto-save.
type FileStore struct {
	mu       sync.RWMutex
	filePath string
	autoSave bool
	saveInterval time.Duration
	lastSave time.Time

	// Applications indexed by ID
	apps map[string]*model.Application
	// Config items indexed by appID -> env -> key -> ConfigItem
	configs map[string]map[string]map[string]*model.ConfigItem
	// Versions indexed by appID -> env -> versionNumber -> Version
	versions map[string]map[string]map[int]*model.Version
	// Audit logs
	auditLogs []model.AuditLog

	// quit channel for auto-save goroutine
	quit       chan struct{}
	// loopWg tracks the auto-save goroutine so Close can wait for it to
	// fully exit before returning.
	loopWg     sync.WaitGroup
	// closeOnce guarantees the quit channel is closed at most once,
	// so repeated Close calls are safe and never panic with
	// "close of closed channel".
	closeOnce  sync.Once
	panicGuard PanicGuardFn
	logger     *logger.Logger
}

// NewFileStore creates a new FileStore.
func NewFileStore(filePath string, autoSave bool, saveInterval time.Duration) (*FileStore, error) {
	fs := &FileStore{
		filePath:    filePath,
		autoSave:    autoSave,
		saveInterval: saveInterval,
		apps:        make(map[string]*model.Application),
		configs:     make(map[string]map[string]map[string]*model.ConfigItem),
		versions:    make(map[string]map[string]map[int]*model.Version),
		auditLogs:   make([]model.AuditLog, 0),
		quit:        make(chan struct{}),
		logger:      logger.WithField("store", "file"),
	}

	// Load existing data if file exists
	if _, err := os.Stat(filePath); err == nil {
		if err := fs.load(); err != nil {
			return nil, fmt.Errorf("failed to load data from file: %w", err)
		}
		fs.logger.Infof("Loaded data from %s", filePath)
	}

	// Start auto-save goroutine if enabled
	if autoSave {
		fs.loopWg.Add(1)
		go func() {
			defer fs.loopWg.Done()
			fs.autoSaveLoop()
		}()
	}

	return fs, nil
}

// Compile-time check that FileStore implements Store.
var _ Store = (*FileStore)(nil)

// load reads the store data from the JSON file.
func (fs *FileStore) load() error {
	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return err
	}

	var exportData struct {
		Apps      map[string]*model.Application                    `json:"apps"`
		Configs   map[string]map[string]map[string]*model.ConfigItem `json:"configs"`
		Versions  map[string]map[string]map[int]*model.Version       `json:"versions"`
		AuditLogs []model.AuditLog                                  `json:"audit_logs"`
	}

	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	fs.apps = exportData.Apps
	fs.configs = exportData.Configs
	fs.versions = exportData.Versions
	fs.auditLogs = exportData.AuditLogs

	return nil
}

// save writes the store data to the JSON file.
func (fs *FileStore) save() error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	exportData := struct {
		Apps      map[string]*model.Application                    `json:"apps"`
		Configs   map[string]map[string]map[string]*model.ConfigItem `json:"configs"`
		Versions  map[string]map[string]map[int]*model.Version       `json:"versions"`
		AuditLogs []model.AuditLog                                  `json:"audit_logs"`
	}{
		Apps:      fs.apps,
		Configs:   fs.configs,
		Versions:  fs.versions,
		AuditLogs: fs.auditLogs,
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tmpFile := fs.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	fs.lastSave = time.Now()
	fs.logger.Debugf("Saved data to %s", fs.filePath)
	return nil
}

// autoSaveLoop periodically saves data. It exits when the quit channel is
// autoSaveLoop periodically saves data. It exits when the quit channel is
// closed (by Close) and is tracked by loopWg so Close can wait for it to fully
// exit. It never closes the quit channel itself — that is the sole
// responsibility of Close, guarded by closeOnce — so there is no double-close.
func (fs *FileStore) autoSaveLoop() {
	ticker := time.NewTicker(fs.saveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := fs.save(); err != nil {
				fs.logger.Errorf("Auto-save failed: %v", err)
			}
		case <-fs.quit:
			if err := fs.save(); err != nil {
				fs.logger.Errorf("Auto-save on shutdown failed: %v", err)
			}
			return
		}
	}
}

// Close stops the auto-save loop and performs a final save.
// It is safe to call multiple times: closeOnce ensures quit is closed at most
// once, and loopWg.Wait() is a no-op after the first Close once the goroutine
// has exited. Subsequent calls only re-run the final save (idempotent).
func (fs *FileStore) Close() error {
	fs.closeOnce.Do(func() {
		close(fs.quit)
	})
	// Wait for the auto-save goroutine to fully exit so its final save cannot
	// race with the one below and so it has stopped touching the file before
	// Close returns. When autoSave is disabled the goroutine never started
	// and Wait returns immediately.
	fs.loopWg.Wait()
	return fs.save()
}

// SetPanicGuard sets a guard function for controlling panic behavior.
func (fs *FileStore) SetPanicGuard(fn PanicGuardFn) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.panicGuard = fn
}

// RawSnapshot returns a diagnostic snapshot of all stored data.
func (fs *FileStore) RawSnapshot() map[string]interface{} {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	snapshot := make(map[string]interface{})
	snapshot["apps"] = len(fs.apps)
	snapshot["configs"] = len(fs.configs)
	snapshot["versions"] = len(fs.versions)
	snapshot["audit_logs"] = len(fs.auditLogs)
	snapshot["auto_save"] = fs.autoSave
	return snapshot
}

// HealthCheck verifies the store is functioning properly.
func (fs *FileStore) HealthCheck(_ context.Context) error {
	return nil
}

// CreateApp creates a new application.
func (fs *FileStore) CreateApp(_ context.Context, app *model.Application) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[app.ID]; exists {
		return model.ErrAppAlreadyExists(app.ID)
	}

	fs.apps[app.ID] = app
	if fs.configs[app.ID] == nil {
		fs.configs[app.ID] = make(map[string]map[string]*model.ConfigItem)
	}
	if fs.versions[app.ID] == nil {
		fs.versions[app.ID] = make(map[string]map[int]*model.Version)
	}

	return nil
}

// GetApp retrieves an application by ID.
func (fs *FileStore) GetApp(_ context.Context, appID string) (*model.Application, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	app, exists := fs.apps[appID]
	if !exists {
		return nil, model.ErrAppNotFound(appID)
	}
	return app, nil
}

// UpdateApp updates an existing application.
func (fs *FileStore) UpdateApp(_ context.Context, app *model.Application) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[app.ID]; !exists {
		return model.ErrAppNotFound(app.ID)
	}

	app.UpdatedAt = time.Now()
	fs.apps[app.ID] = app
	return nil
}

// DeleteApp deletes an application and all associated data.
func (fs *FileStore) DeleteApp(_ context.Context, appID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[appID]; !exists {
		return model.ErrAppNotFound(appID)
	}

	delete(fs.apps, appID)
	delete(fs.configs, appID)
	delete(fs.versions, appID)
	return nil
}

// ListApps returns all applications with pagination.
func (fs *FileStore) ListApps(_ context.Context, page, pageSize int) ([]*model.Application, int, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	allApps := make([]*model.Application, 0, len(fs.apps))
	for _, app := range fs.apps {
		allApps = append(allApps, app)
	}

	// Sort by creation date
	sortApps(allApps)

	total := len(allApps)

	start, end := paginate(total, page, pageSize)
	return allApps[start:end], total, nil
}

// sortApps sorts applications by creation date.
func sortApps(apps []*model.Application) {
	for i := 1; i < len(apps); i++ {
		for j := i; j > 0; j-- {
			if apps[j].CreatedAt.Before(apps[j-1].CreatedAt) {
				apps[j-1], apps[j] = apps[j], apps[j-1]
			}
		}
	}
}

// paginate calculates pagination offsets.
func paginate(total, page, pageSize int) (int, int) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
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
	return start, end
}

// CreateConfig creates a new config item.
func (fs *FileStore) CreateConfig(_ context.Context, config *model.ConfigItem) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[config.AppID]; !exists {
		return model.ErrAppNotFound(config.AppID)
	}

	if fs.configs[config.AppID] == nil {
		fs.configs[config.AppID] = make(map[string]map[string]*model.ConfigItem)
	}
	if fs.configs[config.AppID][config.Environment] == nil {
		fs.configs[config.AppID][config.Environment] = make(map[string]*model.ConfigItem)
	}

	if _, exists := fs.configs[config.AppID][config.Environment][config.Key]; exists {
		return model.NewAppError(model.ErrCodeAlreadyExists,
			fmt.Sprintf("config key '%s' already exists", config.Key))
	}

	fs.configs[config.AppID][config.Environment][config.Key] = config
	return nil
}

// GetConfig retrieves a config item.
func (fs *FileStore) GetConfig(_ context.Context, appID, env, key string) (*model.ConfigItem, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	appConfigs, exists := fs.configs[appID]
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
func (fs *FileStore) UpdateConfig(_ context.Context, config *model.ConfigItem) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[config.AppID]; !exists {
		return model.ErrAppNotFound(config.AppID)
	}

	existing := fs.configs[config.AppID][config.Environment][config.Key]
	if existing == nil {
		return model.ErrConfigNotFound(config.AppID, config.Environment, config.Key)
	}

	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = time.Now()
	fs.configs[config.AppID][config.Environment][config.Key] = config
	return nil
}

// DeleteConfig deletes a config item.
func (fs *FileStore) DeleteConfig(_ context.Context, appID, env, key string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[appID]; !exists {
		return model.ErrAppNotFound(appID)
	}

	if fs.configs[appID] == nil || fs.configs[appID][env] == nil || fs.configs[appID][env][key] == nil {
		return model.ErrConfigNotFound(appID, env, key)
	}

	delete(fs.configs[appID][env], key)
	return nil
}

// ListConfigs returns all config items for an app and environment.
func (fs *FileStore) ListConfigs(_ context.Context, appID, env string) ([]*model.ConfigItem, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	appConfigs, exists := fs.configs[appID]
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
	sortConfigsByKey(result)
	return result, nil
}

// sortConfigsByKey sorts config items by key.
func sortConfigsByKey(configs []*model.ConfigItem) {
	for i := 1; i < len(configs); i++ {
		for j := i; j > 0; j-- {
			if configs[j].Key < configs[j-1].Key {
				configs[j-1], configs[j] = configs[j], configs[j-1]
			}
		}
	}
}

// BatchCreateConfigs creates multiple config items.
func (fs *FileStore) BatchCreateConfigs(_ context.Context, configs []*model.ConfigItem) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	for _, config := range configs {
		if _, exists := fs.apps[config.AppID]; !exists {
			return model.ErrAppNotFound(config.AppID)
		}
		if fs.configs[config.AppID] == nil {
			fs.configs[config.AppID] = make(map[string]map[string]*model.ConfigItem)
		}
		if fs.configs[config.AppID][config.Environment] == nil {
			fs.configs[config.AppID][config.Environment] = make(map[string]*model.ConfigItem)
		}
		fs.configs[config.AppID][config.Environment][config.Key] = config
	}
	return nil
}

// BatchDeleteConfigs deletes multiple config items.
func (fs *FileStore) BatchDeleteConfigs(_ context.Context, appID, env string, keys []string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[appID]; !exists {
		return model.ErrAppNotFound(appID)
	}
	if fs.configs[appID] == nil || fs.configs[appID][env] == nil {
		return nil
	}

	for _, key := range keys {
		delete(fs.configs[appID][env], key)
	}
	return nil
}

// GetConfigMap returns the full config as a key-value map.
func (fs *FileStore) GetConfigMap(_ context.Context, appID, env string) (map[string]string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	result := make(map[string]string)
	appConfigs, exists := fs.configs[appID]
	if !exists {
		return result, nil
	}
	envConfigs, exists := appConfigs[env]
	if !exists {
		return result, nil
	}
	for key, config := range envConfigs {
		result[key] = config.Value
	}
	return result, nil
}

// ReplaceConfigMap replaces the entire config for an app and environment.
func (fs *FileStore) ReplaceConfigMap(_ context.Context, appID, env string, configs []*model.ConfigItem) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[appID]; !exists {
		return model.ErrAppNotFound(appID)
	}

	if fs.configs[appID] == nil {
		fs.configs[appID] = make(map[string]map[string]*model.ConfigItem)
	}
	fs.configs[appID][env] = make(map[string]*model.ConfigItem, len(configs))

	now := time.Now()
	for _, config := range configs {
		config.CreatedAt = now
		config.UpdatedAt = now
		fs.configs[appID][env][config.Key] = config
	}
	return nil
}

// CreateVersion creates a new version snapshot.
func (fs *FileStore) CreateVersion(_ context.Context, version *model.Version) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.apps[version.AppID]; !exists {
		return model.ErrAppNotFound(version.AppID)
	}

	if fs.versions[version.AppID] == nil {
		fs.versions[version.AppID] = make(map[string]map[int]*model.Version)
	}
	if fs.versions[version.AppID][version.Environment] == nil {
		fs.versions[version.AppID][version.Environment] = make(map[int]*model.Version)
	}

	fs.versions[version.AppID][version.Environment][version.VersionNumber] = version
	return nil
}

// GetVersion retrieves a specific version.
func (fs *FileStore) GetVersion(_ context.Context, appID, env string, versionNumber int) (*model.Version, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	appVersions, exists := fs.versions[appID]
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
	return version, nil
}

// GetLatestVersionNumber returns the latest version number.
func (fs *FileStore) GetLatestVersionNumber(_ context.Context, appID, env string) (int, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	appVersions, exists := fs.versions[appID]
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

// ListVersions returns version history.
func (fs *FileStore) ListVersions(_ context.Context, appID, env string, page, pageSize int) ([]model.VersionInfo, int, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	appVersions, exists := fs.versions[appID]
	if !exists {
		return nil, 0, nil
	}
	envVersions, exists := appVersions[env]
	if !exists {
		return nil, 0, nil
	}

	allVersions := make([]model.VersionInfo, 0, len(envVersions))
	for _, v := range envVersions {
		allVersions = append(allVersions, v.ToInfo())
	}

	// Sort by version number descending
	sortVersions(allVersions)

	total := len(allVersions)
	start, end := paginate(total, page, pageSize)
	return allVersions[start:end], total, nil
}

// sortVersions sorts version info by version number descending.
func sortVersions(versions []model.VersionInfo) {
	for i := 1; i < len(versions); i++ {
		for j := i; j > 0; j-- {
			if versions[j].VersionNumber > versions[j-1].VersionNumber {
				versions[j-1], versions[j] = versions[j], versions[j-1]
			}
		}
	}
}

// DeleteVersionsBefore removes versions older than the specified version.
func (fs *FileStore) DeleteVersionsBefore(_ context.Context, appID, env string, beforeVersion int) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	appVersions, exists := fs.versions[appID]
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
func (fs *FileStore) CreateAuditLog(_ context.Context, log *model.AuditLog) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.auditLogs = append(fs.auditLogs, *log)
	return nil
}

// ListAuditLogs returns audit logs matching the filter.
func (fs *FileStore) ListAuditLogs(_ context.Context, filter model.AuditLogFilter) ([]model.AuditLog, int, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var filtered []model.AuditLog
	for _, log := range fs.auditLogs {
		if filter.AppID != "" && log.AppID != filter.AppID {
			continue
		}
		if filter.Environment != "" && log.Environment != filter.Environment {
			continue
		}
		if filter.Action != "" && log.Action != filter.Action {
			continue
		}
		if filter.User != "" && !containsSubstring(log.User, filter.User) {
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
	sortAuditLogs(filtered)

	total := len(filtered)
	start, end := paginate(total, filter.Page, filter.PageSize)
	return filtered[start:end], total, nil
}

// sortAuditLogs sorts audit logs by creation date descending.
func sortAuditLogs(logs []model.AuditLog) {
	for i := 1; i < len(logs); i++ {
		for j := i; j > 0; j-- {
			if logs[j].CreatedAt.After(logs[j-1].CreatedAt) {
				logs[j-1], logs[j] = logs[j], logs[j-1]
			}
		}
	}
}

// containsSubstring checks if s contains substr (case-insensitive).
func containsSubstring(s, substr string) bool {
	s = toLowerStr(s)
	substr = toLowerStr(substr)
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

// toLowerStr converts a string to lowercase.
func toLowerStr(s string) string {
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

// searchSubstring checks if s contains substr.
func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
