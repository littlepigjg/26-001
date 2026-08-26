package service

import (
	"context"
	"fmt"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/diff"
	"config-center/pkg/hash"
	"config-center/pkg/logger"
)

// DiffService provides configuration comparison and change tracking.
type DiffService struct {
	store  store.Store
	appSvc *AppService
	logger *logger.Logger
}

// NewDiffService creates a new DiffService.
func NewDiffService(s store.Store, appSvc *AppService) *DiffService {
	return &DiffService{
		store:  s,
		appSvc: appSvc,
		logger: logger.WithField("service", "diff"),
	}
}

// DiffResult contains the comparison result between two configurations.
type DiffResult struct {
	// AppID is the application identifier.
	AppID string `json:"app_id"`
	// Environment is the environment.
	Environment string `json:"environment"`
	// LeftLabel is the label for the left side (e.g., "version 1").
	LeftLabel string `json:"left_label"`
	// RightLabel is the label for the right side (e.g., "version 2").
	RightLabel string `json:"right_label"`
	// Changes is the list of differences.
	Changes []diff.Change `json:"changes"`
	// AddedCount is the number of added keys.
	AddedCount int `json:"added_count"`
	// ModifiedCount is the number of modified keys.
	ModifiedCount int `json:"modified_count"`
	// RemovedCount is the number of removed keys.
	RemovedCount int `json:"removed_count"`
	// HasChanges indicates if there are any differences.
	HasChanges bool `json:"has_changes"`
}

// DiffVersions compares two historical versions of a configuration.
func (s *DiffService) DiffVersions(ctx context.Context, appID, env string, v1, v2 int) (*DiffResult, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	ver1, err := s.store.GetVersion(ctx, appID, env, v1)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", v1, err)
	}

	ver2, err := s.store.GetVersion(ctx, appID, env, v2)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", v2, err)
	}

	changes := diff.Diff(ver1.ConfigData, ver2.ConfigData)
	added, modified, removed := diff.DiffCount(ver1.ConfigData, ver2.ConfigData)

	return &DiffResult{
		AppID:         appID,
		Environment:   env,
		LeftLabel:     fmt.Sprintf("version %d", v1),
		RightLabel:    fmt.Sprintf("version %d", v2),
		Changes:       changes,
		AddedCount:    added,
		ModifiedCount: modified,
		RemovedCount:  removed,
		HasChanges:    len(changes) > 0,
	}, nil
}

// DiffCurrentVersion compares the current configuration with a historical version.
func (s *DiffService) DiffCurrentVersion(ctx context.Context, appID, env string, historicalVersion int) (*DiffResult, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	// Get historical version
	historical, err := s.store.GetVersion(ctx, appID, env, historicalVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical version %d: %w", historicalVersion, err)
	}

	// Get current config
	currentConfig, err := s.store.GetConfigMap(ctx, appID, env)
	if err != nil {
		return nil, fmt.Errorf("failed to get current config: %w", err)
	}

	changes := diff.Diff(historical.ConfigData, currentConfig)
	added, modified, removed := diff.DiffCount(historical.ConfigData, currentConfig)

	return &DiffResult{
		AppID:         appID,
		Environment:   env,
		LeftLabel:     fmt.Sprintf("version %d", historicalVersion),
		RightLabel:    "current",
		Changes:       changes,
		AddedCount:    added,
		ModifiedCount: modified,
		RemovedCount:  removed,
		HasChanges:    len(changes) > 0,
	}, nil
}

// DiffCurrentWithConfig compares the current configuration with a provided config map.
func (s *DiffService) DiffCurrentWithConfig(ctx context.Context, appID, env string, newConfig map[string]string) (*DiffResult, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	currentConfig, err := s.store.GetConfigMap(ctx, appID, env)
	if err != nil {
		return nil, fmt.Errorf("failed to get current config: %w", err)
	}

	changes := diff.Diff(currentConfig, newConfig)
	added, modified, removed := diff.DiffCount(currentConfig, newConfig)

	return &DiffResult{
		AppID:         appID,
		Environment:   env,
		LeftLabel:     "current",
		RightLabel:    "proposed",
		Changes:       changes,
		AddedCount:    added,
		ModifiedCount: modified,
		RemovedCount:  removed,
		HasChanges:    len(changes) > 0,
	}, nil
}

// GenerateChangeSummary creates a human-readable summary of changes.
func (s *DiffService) GenerateChangeSummary(changes []diff.Change) string {
	if len(changes) == 0 {
		return "no changes"
	}

	added, modified, removed := diff.DiffCountFromChanges(changes)
	parts := make([]string, 0, 3)
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", modified))
	}
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", removed))
	}
	return fmt.Sprintf("changes: %s", parts)
}

// ComputeConfigHash computes a hash for a configuration map.
func (s *DiffService) ComputeConfigHash(config map[string]string) string {
	return hash.MapHash(config)
}

// DiffConfigs compares two config maps and returns the changes.
func (s *DiffService) DiffConfigs(oldConfig, newConfig map[string]string) []diff.Change {
	return diff.Diff(oldConfig, newConfig)
}

// Ensure imports are used
var _ = model.ErrAppNotFound
