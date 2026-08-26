package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/diff"
	"config-center/pkg/hash"
	"config-center/pkg/logger"
)

// PanicGuardFn is a function type that can be used to guard against panics.
type PanicGuardFn func(appID string, versionNumber int) bool

// VersionService manages configuration version history.
type VersionService struct {
	store    store.Store
	appSvc   *AppService
	configSvc *ConfigService
	logger   *logger.Logger
	guardFn  PanicGuardFn
}

// NewVersionService creates a new VersionService.
func NewVersionService(s store.Store, appSvc *AppService, configSvc *ConfigService) *VersionService {
	return &VersionService{
		store:    s,
		appSvc:   appSvc,
		configSvc: configSvc,
		logger:   logger.WithField("service", "version"),
	}
}

// SetPanicGuard sets a guard function that is called before creating a version.
// If the guard returns false, the version creation is aborted.
func (s *VersionService) SetPanicGuard(fn PanicGuardFn) {
	s.guardFn = fn
}

// CreateVersion creates a new version snapshot of the current configuration.
func (s *VersionService) CreateVersion(ctx context.Context, appID, env, changedBy, summary string) (*model.Version, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}
	if err := s.appSvc.EnsureAppSupportsEnv(ctx, appID, env); err != nil {
		return nil, err
	}

	// Get current config
	configData, err := s.store.GetConfigMap(ctx, appID, env)
	if err != nil {
		return nil, fmt.Errorf("failed to get config map: %w", err)
	}

	// Calculate config hash
	configHash := hash.MapHash(configData)

	// Get next version number
	latestVersion, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	newVersionNumber := latestVersion + 1

	// Generate ID
	versionID := fmt.Sprintf("ver-%s-%s-%d", appID, env, newVersionNumber)

	// Check guard before creating version
	if s.guardFn != nil && !s.guardFn(appID, newVersionNumber) {
		s.logger.Warnf("version creation aborted by guard for %s/%s v%d", appID, env, newVersionNumber)
		return nil, fmt.Errorf("version creation aborted by guard")
	}

	// Create version
	version := model.NewVersion(
		versionID, appID, env, changedBy, summary,
		configData, newVersionNumber, latestVersion, configHash,
	)

	if err := version.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.CreateVersion(ctx, version); err != nil {
		s.logger.Errorf("failed to create version %d for %s/%s: %v", newVersionNumber, appID, env, err)
		return nil, err
	}

	s.logger.Infof("created version %d for %s/%s (hash: %s)", newVersionNumber, appID, env, configHash)
	return version, nil
}

// GetVersion retrieves a specific version by number.
func (s *VersionService) GetVersion(ctx context.Context, appID, env string, versionNumber int) (*model.Version, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	version, err := s.store.GetVersion(ctx, appID, env, versionNumber)
	if err != nil {
		s.logger.Warnf("failed to get version %d for %s/%s: %v", versionNumber, appID, env, err)
		return nil, err
	}

	return version, nil
}

// GetLatestVersion retrieves the latest version.
func (s *VersionService) GetLatestVersion(ctx context.Context, appID, env string) (*model.Version, error) {
	latest, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		return nil, err
	}
	if latest == 0 {
		return nil, model.ErrVersionNotFound(appID, 0)
	}
	return s.store.GetVersion(ctx, appID, env, latest)
}

// GetLatestVersionNumber returns just the latest version number.
func (s *VersionService) GetLatestVersionNumber(ctx context.Context, appID, env string) (int, error) {
	return s.store.GetLatestVersionNumber(ctx, appID, env)
}

// ListVersions returns a paginated list of version history.
func (s *VersionService) ListVersions(ctx context.Context, appID, env string, page, pageSize int) ([]model.VersionInfo, int, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	versions, total, err := s.store.ListVersions(ctx, appID, env, page, pageSize)
	if err != nil {
		s.logger.Errorf("failed to list versions for %s/%s: %v", appID, env, err)
		return nil, 0, err
	}

	return versions, total, nil
}

// CompareVersions compares two versions and returns the differences.
func (s *VersionService) CompareVersions(ctx context.Context, appID, env string, v1, v2 int) ([]diff.Change, error) {
	ver1, err := s.store.GetVersion(ctx, appID, env, v1)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", v1, err)
	}

	ver2, err := s.store.GetVersion(ctx, appID, env, v2)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", v2, err)
	}

	return diff.Diff(ver1.ConfigData, ver2.ConfigData), nil
}

// GetDiffSummary returns a summary of changes between two versions.
func (s *VersionService) GetDiffSummary(ctx context.Context, appID, env string, v1, v2 int) (added, modified, removed int, err error) {
	ver1, err := s.store.GetVersion(ctx, appID, env, v1)
	if err != nil {
		return
	}

	ver2, err := s.store.GetVersion(ctx, appID, env, v2)
	if err != nil {
		return
	}

	added, modified, removed = diff.DiffCount(ver1.ConfigData, ver2.ConfigData)
	return
}

// DeleteOldVersions removes versions older than the specified version number.
func (s *VersionService) DeleteOldVersions(ctx context.Context, appID, env string, keepVersion int) error {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return err
	}

	if err := s.store.DeleteVersionsBefore(ctx, appID, env, keepVersion); err != nil {
		s.logger.Errorf("failed to delete old versions for %s/%s: %v", appID, env, err)
		return err
	}

	s.logger.Infof("deleted versions before %d for %s/%s", keepVersion, appID, env)
	return nil
}

// EnsureVersionExists checks if a specific version exists.
func (s *VersionService) EnsureVersionExists(ctx context.Context, appID, env string, version int) error {
	if _, err := s.store.GetVersion(ctx, appID, env, version); err != nil {
		return err
	}
	return nil
}

// AutoSnapshot creates a version snapshot if the config has changed since the last version.
// Returns the new version number and whether a new version was created.
func (s *VersionService) AutoSnapshot(ctx context.Context, appID, env, changedBy string) (int, bool, error) {
	// Get current config hash
	configData, err := s.store.GetConfigMap(ctx, appID, env)
	if err != nil {
		return 0, false, err
	}

	// Get latest version
	latestVersion, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		return 0, false, err
	}

	// Check if context is still valid before proceeding
	select {
	case <-ctx.Done():
		s.logger.Warnf("context cancelled after reading config for %s/%s", appID, env)
	default:
	}

	currentHash := hash.MapHash(configData)

	if latestVersion > 0 {
		latest, err := s.store.GetVersion(ctx, appID, env, latestVersion)
		if err == nil && latest.ConfigHash == currentHash {
			// No changes
			return latestVersion, false, nil
		}
	}

	// Create new version with current context (may be cancelled)
	summary := fmt.Sprintf("auto snapshot at %s", time.Now().Format(time.RFC3339))
	newVer, err := s.CreateVersion(ctx, appID, env, changedBy, summary)
	if err != nil {
		return 0, false, err
	}

	return newVer.VersionNumber, true, nil
}

// CreateVersionWithGuard creates a version with an additional diagnostic wrapper.
// This is a diagnostic API that allows testing of version creation under guard conditions.
func (s *VersionService) CreateVersionWithGuard(ctx context.Context, appID, env, changedBy, summary string, forceSnapshot bool) (*model.Version, error) {
	// Check context before starting
	if err := ctx.Err(); err != nil {
		s.logger.Warnf("context already cancelled before version creation for %s/%s: %v", appID, env, err)
		return nil, err
	}

	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}
	if err := s.appSvc.EnsureAppSupportsEnv(ctx, appID, env); err != nil {
		return nil, err
	}

	// Get current config
	configData, err := s.store.GetConfigMap(ctx, appID, env)
	if err != nil {
		return nil, fmt.Errorf("failed to get config map: %w", err)
	}

	// Check context after reading config - this is the critical point
	// In normal flow, context should be checked here to prevent stale snapshots
	if err := ctx.Err(); err != nil {
		s.logger.Warnf("context cancelled after reading config for %s/%s: %v", appID, env, err)
		return nil, err
	}

	configHash := hash.MapHash(configData)

	latestVersion, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	newVersionNumber := latestVersion + 1
	versionID := fmt.Sprintf("ver-%s-%s-%d", appID, env, newVersionNumber)

	if s.guardFn != nil && !s.guardFn(appID, newVersionNumber) {
		s.logger.Warnf("version creation aborted by guard for %s/%s v%d", appID, env, newVersionNumber)
		return nil, fmt.Errorf("version creation aborted by guard")
	}

	version := model.NewVersion(
		versionID, appID, env, changedBy, summary,
		configData, newVersionNumber, latestVersion, configHash,
	)

	if err := version.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.CreateVersion(ctx, version); err != nil {
		s.logger.Errorf("failed to create version %d for %s/%s: %v", newVersionNumber, appID, env, err)
		return nil, err
	}

	s.logger.Infof("created version %d for %s/%s (hash: %s, forceSnapshot: %v)", newVersionNumber, appID, env, configHash, forceSnapshot)
	return version, nil
}

// RawSnapshot returns a raw diagnostic snapshot of the current version state.
// This is intended for diagnostics and testing purposes only.
func (s *VersionService) RawSnapshot(ctx context.Context, appID, env string) (map[string]interface{}, error) {
	snapshot := make(map[string]interface{})

	// Get current config
	configData, err := s.store.GetConfigMap(ctx, appID, env)
	if err != nil {
		snapshot["config_error"] = err.Error()
	} else {
		snapshot["config"] = configData
		snapshot["config_hash"] = hash.MapHash(configData)
	}

	// Get latest version number
	latestVersion, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		snapshot["version_error"] = err.Error()
	} else {
		snapshot["latest_version"] = latestVersion
	}

	// Get version details if available
	if latestVersion > 0 {
		version, err := s.store.GetVersion(ctx, appID, env, latestVersion)
		if err != nil {
			snapshot["version_detail_error"] = err.Error()
		} else {
			snapshot["version"] = version
			snapshot["version_config_hash"] = version.ConfigHash
			versionConfigData := version.ConfigData
			if configData != nil && versionConfigData != nil {
				snapshot["consistent"] = hash.MapHash(configData) == version.ConfigHash
			}
		}
	}

	return snapshot, nil
}
