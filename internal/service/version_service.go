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

// VersionService manages configuration version history.
type VersionService struct {
	store   store.Store
	appSvc  *AppService
	configSvc *ConfigService
	logger  *logger.Logger
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

// SetPanicGuard delegates a safety hook to the underlying store.
// When the store is a MemoryStore, the guard can intercept dangerous
// operations before they cause runtime failures.
func (s *VersionService) SetPanicGuard(guard store.PanicGuardFn) {
	if ms, ok := s.store.(*store.MemoryStore); ok {
		ms.SetPanicGuard(guard)
	}
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
	currentHash := hash.MapHash(configData)

	// Get latest version
	latestVersion, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		return 0, false, err
	}

	if latestVersion > 0 {
		latest, err := s.store.GetVersion(ctx, appID, env, latestVersion)
		if err == nil && latest.ConfigHash == currentHash {
			// No changes
			return latestVersion, false, nil
		}
	}

	// Create new version
	summary := fmt.Sprintf("auto snapshot at %s", time.Now().Format(time.RFC3339))
	newVer, err := s.CreateVersion(ctx, appID, env, changedBy, summary)
	if err != nil {
		return 0, false, err
	}

	return newVer.VersionNumber, true, nil
}
