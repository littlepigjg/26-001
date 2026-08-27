package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/diff"
	"config-center/pkg/logger"
)

// RollbackService handles configuration rollback operations.
// It allows restoring configuration to a previous version.
type RollbackService struct {
	store      store.Store
	appSvc     *AppService
	configSvc  *ConfigService
	versionSvc *VersionService
	auditSvc   *AuditService
	logger     *logger.Logger
}

// NewRollbackService creates a new RollbackService.
func NewRollbackService(
	s store.Store,
	appSvc *AppService,
	configSvc *ConfigService,
	versionSvc *VersionService,
	auditSvc *AuditService,
) *RollbackService {
	return &RollbackService{
		store:      s,
		appSvc:     appSvc,
		configSvc:  configSvc,
		versionSvc: versionSvc,
		auditSvc:   auditSvc,
		logger:     logger.WithField("service", "rollback"),
	}
}

// RollbackResult contains the result of a rollback operation.
type RollbackResult struct {
	// AppID is the application identifier.
	AppID string
	// Environment is the environment.
	Environment string
	// FromVersion is the version being rolled back from.
	FromVersion int
	// ToVersion is the version being rolled back to.
	ToVersion int
	// Changes describes what changed during rollback.
	Changes []diff.Change
	// NewVersion is the version number created after rollback.
	NewVersion int
}

// Rollback restores configuration to a specific historical version.
// It creates a new version snapshot rather than modifying existing ones.
func (s *RollbackService) Rollback(ctx context.Context, appID, env string, targetVersion int, user, ipAddress string) (*RollbackResult, error) {
	// Validate app and environment
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}
	if err := s.appSvc.EnsureAppSupportsEnv(ctx, appID, env); err != nil {
		return nil, err
	}

	// Get the target version
	target, err := s.store.GetVersion(ctx, appID, env, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get target version %d: %w", targetVersion, err)
	}

	// Get current version for diff calculation
	currentVersion, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	// Calculate diff between current and target
	var changes []diff.Change
	if currentVersion > 0 && currentVersion != targetVersion {
		current, err := s.store.GetVersion(ctx, appID, env, currentVersion)
		if err == nil {
			changes = diff.Diff(current.ConfigData, target.ConfigData)
		}
	}

	// Restore configuration to target version
	// Build config items from the target version's config data
	configItems := make([]*model.ConfigItem, 0, len(target.ConfigData))
	now := time.Now()
	for key, value := range target.ConfigData {
		configItems = append(configItems, &model.ConfigItem{
			ID:          fmt.Sprintf("cfg-%s-%s-%s", appID, env, key),
			AppID:       appID,
			Environment: env,
			Key:         key,
			Value:       value,
			Description: "",
			Format:      "string",
			Required:    false,
			Version:     targetVersion,
			UpdatedBy:   user,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	// Replace config map
	if err := s.store.ReplaceConfigMap(ctx, appID, env, configItems); err != nil {
		s.logger.Errorf("failed to restore config for %s/%s: %v", appID, env, err)
		return nil, fmt.Errorf("failed to restore config: %w", err)
	}

	// Create a new version snapshot
	summary := fmt.Sprintf("rollback to version %d", targetVersion)
	newVer, err := s.versionSvc.CreateVersion(ctx, appID, env, user, summary)
	if err != nil {
		return nil, fmt.Errorf("failed to create rollback version: %w", err)
	}

	// Log the rollback
	if err := s.auditSvc.LogRollback(ctx, appID, env, currentVersion, targetVersion, user, ipAddress); err != nil {
		s.logger.Errorf("failed to log rollback %s/%s: %v", appID, env, err)
		return nil, fmt.Errorf("audit log write failed: %w", err)
	}

	s.logger.Infof("rolled back %s/%s from v%d to v%d (new version: %d)",
		appID, env, currentVersion, targetVersion, newVer.VersionNumber)

	return &RollbackResult{
		AppID:       appID,
		Environment: env,
		FromVersion: currentVersion,
		ToVersion:   targetVersion,
		Changes:     changes,
		NewVersion:  newVer.VersionNumber,
	}, nil
}

// RollbackToPrevious restores configuration to the immediately previous version.
func (s *RollbackService) RollbackToPrevious(ctx context.Context, appID, env string, user, ipAddress string) (*RollbackResult, error) {
	latest, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		return nil, err
	}

	if latest <= 1 {
		return nil, model.NewAppError(model.ErrCodeNotFound,
			fmt.Sprintf("no previous version to rollback to for %s/%s", appID, env))
	}

	return s.Rollback(ctx, appID, env, latest-1, user, ipAddress)
}

// GetRollbackPreview shows what a rollback would change without actually performing it.
func (s *RollbackService) GetRollbackPreview(ctx context.Context, appID, env string, targetVersion int) ([]diff.Change, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	target, err := s.store.GetVersion(ctx, appID, env, targetVersion)
	if err != nil {
		return nil, err
	}

	currentVersion, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil || currentVersion == 0 {
		// No current version, so the diff is just the target
		return nil, nil
	}

	current, err := s.store.GetVersion(ctx, appID, env, currentVersion)
	if err != nil {
		return nil, err
	}

	return diff.Diff(current.ConfigData, target.ConfigData), nil
}

// CanRollback checks if a rollback to the target version is possible.
func (s *RollbackService) CanRollback(ctx context.Context, appID, env string, targetVersion int) (bool, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return false, err
	}

	// Check target version exists
	if _, err := s.store.GetVersion(ctx, appID, env, targetVersion); err != nil {
		return false, nil
	}

	// Check it's not the same as current
	current, err := s.store.GetLatestVersionNumber(ctx, appID, env)
	if err != nil {
		return false, err
	}

	if current == targetVersion {
		return false, nil
	}

	return true, nil
}
