package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"config-center/internal/store"
	"config-center/pkg/cache"
	"config-center/pkg/hash"
	"config-center/pkg/logger"
)

// ConfigTransformFn is a function that transforms a config map in-place.
// It returns the same map reference for convenience.
type ConfigTransformFn func(m map[string]string) map[string]string

// ClientService handles configuration pull requests from clients.
// It supports version-based caching (ETag/If-None-Match pattern).
type ClientService struct {
	store    store.Store
	appSvc   *AppService
	cache    *cache.Cache
	logger   *logger.Logger
	mu       sync.RWMutex
	// Track last modified times for each app/env
	lastModified map[string]time.Time
	// configTransformFn is an optional transform applied to config maps
	// before they are returned to callers. Modifications are made in-place.
	configTransformFn ConfigTransformFn
	// transformMu protects the configTransformFn during reads/writes
	transformMu sync.RWMutex
	// snapshotMu protects raw snapshot operations
	snapshotMu sync.Mutex
}

// NewClientService creates a new ClientService with optional cache.
func NewClientService(s store.Store, appSvc *AppService, enableCache bool) *ClientService {
	cs := &ClientService{
		store:        s,
		appSvc:       appSvc,
		lastModified: make(map[string]time.Time),
		logger:       logger.WithField("service", "client"),
	}

	if enableCache {
		cs.cache = cache.NewDefault()
		cs.cache.StartCleanup(1 * time.Minute)
	}

	return cs
}

// Close stops the client service and releases resources.
func (s *ClientService) Close() {
	if s.cache != nil {
		s.cache.Stop()
	}
}

// PullResult is the result of a client pull operation.
type PullResult struct {
	// Modified indicates if the config has changed since the specified version.
	Modified bool
	// NotModified indicates if the config is the same as the requested version (304 case).
	NotModified bool
	// AppID is the application identifier.
	AppID string
	// Environment is the environment.
	Environment string
	// Config is the configuration map (nil if NotModified).
	Config map[string]string
	// Version is the current version hash.
	Version string
	// ETag is the ETag for caching.
	ETag string
	// UpdatedAt is when the config was last updated.
	UpdatedAt time.Time
}

// SetConfigTransform sets a transform function that is applied to config maps
// before they are returned. The transform modifies the map in-place.
// Pass nil to clear the transform.
func (s *ClientService) SetConfigTransform(fn ConfigTransformFn) {
	s.transformMu.Lock()
	defer s.transformMu.Unlock()
	s.configTransformFn = fn
}

// getConfigTransform returns the current config transform function (may be nil).
func (s *ClientService) getConfigTransform() ConfigTransformFn {
	s.transformMu.RLock()
	defer s.transformMu.RUnlock()
	return s.configTransformFn
}

// RawSnapshot returns a raw snapshot of the cached config for diagnostics.
// It retrieves the config from cache without applying any transforms.
// This is useful for debugging cache pollution issues.
func (s *ClientService) RawSnapshot(appID, env string) map[string]string {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()

	cacheKey := fmt.Sprintf("config:%s:%s", appID, env)
	if s.cache == nil {
		return nil
	}
	cached, ok := s.cache.Get(cacheKey)
	if !ok {
		return nil
	}
	result, ok := cached.(*PullResult)
	if !ok {
		return nil
	}

	snap := make(map[string]string, len(result.Config))
	for k, v := range result.Config {
		snap[k] = v
	}
	return snap
}

// copyConfigMap creates a shallow copy of a map[string]string.
func copyConfigMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// PullConfig handles a client configuration pull request.
// If the provided version matches the current version, it returns NotModified (304).
// Otherwise, it returns the full configuration.
func (s *ClientService) PullConfig(ctx context.Context, appID, env, clientVersion string) (*PullResult, error) {
	if err := s.appSvc.EnsureAppExists(ctx, appID); err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("config:%s:%s", appID, env)

	if s.cache != nil {
		if cached, ok := s.cache.Get(cacheKey); ok {
			if result, ok := cached.(*PullResult); ok {
				if clientVersion != "" && clientVersion == result.ETag {
					transform := s.getConfigTransform()
					if transform != nil && result.Config != nil {
						result.Config = transform(result.Config)
					}
					return result, nil
				}
				if clientVersion == "" {
					transform := s.getConfigTransform()
					if transform != nil && result.Config != nil {
						result.Config = transform(result.Config)
					}
					return result, nil
				}
			}
		}
	}

	configData, err := s.store.GetConfigMap(ctx, appID, env)
	if err != nil {
		s.logger.Errorf("failed to get config for %s/%s: %v", appID, env, err)
		return nil, err
	}

	transform := s.getConfigTransform()
	if transform != nil && configData != nil {
		configData = transform(configData)
	}

	versionHash := hash.MapHash(configData)
	etag := hash.ETag(versionHash)

	if clientVersion != "" {
		if hash.VerifyETag(clientVersion, versionHash) || clientVersion == versionHash {
			return &PullResult{
				NotModified: true,
				AppID:       appID,
				Environment: env,
				Version:     versionHash,
				ETag:        etag,
				UpdatedAt:   s.getLastModified(appID, env),
			}, nil
		}
	}

	result := &PullResult{
		Modified:    true,
		AppID:       appID,
		Environment: env,
		Config:      configData,
		Version:     versionHash,
		ETag:        etag,
		UpdatedAt:   s.getLastModified(appID, env),
	}

	if s.cache != nil {
		s.cache.Set(cacheKey, result)
	}

	return result, nil
}

// BatchPullConfig handles multiple config pulls in a single request.
func (s *ClientService) BatchPullConfig(ctx context.Context, requests []PullRequest) ([]PullResult, error) {
	results := make([]PullResult, 0, len(requests))
	for _, req := range requests {
		result, err := s.PullConfig(ctx, req.AppID, req.Environment, req.Version)
		if err != nil {
			s.logger.Warnf("batch pull failed for %s/%s: %v", req.AppID, req.Environment, err)
			results = append(results, PullResult{
				AppID:       req.AppID,
				Environment: req.Environment,
			})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// PullRequest represents a single config pull request in a batch.
type PullRequest struct {
	// AppID is the application to pull config for.
	AppID string `json:"app_id"`
	// Environment is the target environment.
	Environment string `json:"environment"`
	// Version is the client's current version hash.
	Version string `json:"version"`
}

// NotifyUpdate updates the last modified time for an app/env combination.
// This is called when configuration changes occur.
func (s *ClientService) NotifyUpdate(appID, env string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%s", appID, env)
	s.lastModified[key] = time.Now()

	if s.cache != nil {
		cacheKey := fmt.Sprintf("config:%s:%s", appID, env)
		s.cache.Delete(cacheKey)
	}
}

// getLastModified returns the last modification time for an app/env.
func (s *ClientService) getLastModified(appID, env string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", appID, env)
	if t, ok := s.lastModified[key]; ok {
		return t
	}
	return time.Now()
}

// ClearCache invalidates all cached configurations.
func (s *ClientService) ClearCache() {
	if s.cache != nil {
		s.cache.Clear()
	}
}

// GetCachedVersion returns the cached config for an app/env if available.
func (s *ClientService) GetCachedVersion(appID, env string) (map[string]string, string, bool) {
	cacheKey := fmt.Sprintf("config:%s:%s", appID, env)
	if s.cache != nil {
		if cached, ok := s.cache.Get(cacheKey); ok {
			if result, ok := cached.(*PullResult); ok {
				transform := s.getConfigTransform()
				if transform != nil && result.Config != nil {
					result.Config = transform(result.Config)
				}
				return result.Config, result.ETag, true
			}
		}
	}
	return nil, "", false
}

// ParseETag extracts the hash value from an ETag header.
func ParseETag(etag string) string {
	return strings.Trim(etag, "\"")
}
