// Package service implements the business logic layer for the configuration center.
// Services coordinate between the store layer and handler layer, implementing
// all configuration management operations.
package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// AppService manages application CRUD operations.
type AppService struct {
	store    store.Store
	logger   *logger.Logger
}

// NewAppService creates a new AppService.
func NewAppService(s store.Store) *AppService {
	return &AppService{
		store:  s,
		logger: logger.WithField("service", "app"),
	}
}

// CreateApp creates a new application.
func (s *AppService) CreateApp(ctx context.Context, id, name, description, owner string) (*model.Application, error) {
	if id == "" {
		return nil, model.ErrInvalidParam("id", "cannot be empty")
	}
	if name == "" {
		return nil, model.ErrInvalidParam("name", "cannot be empty")
	}

	app := model.NewApplication(id, name, description, owner)
	if err := app.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.CreateApp(ctx, app); err != nil {
		s.logger.Errorf("failed to create app %s: %v", id, err)
		return nil, err
	}

	s.logger.Infof("created app: %s (%s)", name, id)
	return app, nil
}

// GetApp retrieves an application by ID.
func (s *AppService) GetApp(ctx context.Context, id string) (*model.Application, error) {
	if id == "" {
		return nil, model.ErrInvalidParam("id", "cannot be empty")
	}

	app, err := s.store.GetApp(ctx, id)
	if err != nil {
		s.logger.Warnf("failed to get app %s: %v", id, err)
		return nil, err
	}

	return app, nil
}

// UpdateApp updates an existing application.
func (s *AppService) UpdateApp(ctx context.Context, id string, name, description, owner string) (*model.Application, error) {
	// Get existing app
	app, err := s.store.GetApp(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update fields
	if name != "" {
		app.Name = name
	}
	if description != "" {
		app.Description = description
	}
	if owner != "" {
		app.Owner = owner
	}

	if err := app.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.UpdateApp(ctx, app); err != nil {
		s.logger.Errorf("failed to update app %s: %v", id, err)
		return nil, err
	}

	s.logger.Infof("updated app: %s", id)
	return app, nil
}

// DeleteApp deletes an application and all its associated data.
func (s *AppService) DeleteApp(ctx context.Context, id string) error {
	if id == "" {
		return model.ErrInvalidParam("id", "cannot be empty")
	}

	// Get the app first to verify it exists
	app, err := s.store.GetApp(ctx, id)
	if err != nil {
		return err
	}

	// Delete the app
	if err := s.store.DeleteApp(ctx, id); err != nil {
		s.logger.Errorf("failed to delete app %s: %v", id, err)
		return err
	}

	s.logger.Infof("deleted app: %s (%s)", app.Name, id)
	return nil
}

// ListApps returns a paginated list of applications.
func (s *AppService) ListApps(ctx context.Context, page, pageSize int) ([]*model.Application, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	apps, total, err := s.store.ListApps(ctx, page, pageSize)
	if err != nil {
		s.logger.Errorf("failed to list apps: %v", err)
		return nil, 0, err
	}

	return apps, total, nil
}

// AddEnvironment adds an environment to an application.
func (s *AppService) AddEnvironment(ctx context.Context, appID, env string) error {
	app, err := s.store.GetApp(ctx, appID)
	if err != nil {
		return err
	}

	app.AddEnvironment(env)

	if err := s.store.UpdateApp(ctx, app); err != nil {
		return fmt.Errorf("failed to update app: %w", err)
	}

	return nil
}

// RemoveEnvironment removes an environment from an application.
func (s *AppService) RemoveEnvironment(ctx context.Context, appID, env string) error {
	app, err := s.store.GetApp(ctx, appID)
	if err != nil {
		return err
	}

	app.RemoveEnvironment(env)

	if err := s.store.UpdateApp(ctx, app); err != nil {
		return fmt.Errorf("failed to update app: %w", err)
	}

	return nil
}

// EnsureAppExists checks if an application exists and returns an error if not.
func (s *AppService) EnsureAppExists(ctx context.Context, appID string) error {
	if _, err := s.store.GetApp(ctx, appID); err != nil {
		return err
	}
	return nil
}

// EnsureAppSupportsEnv checks if an application supports a given environment.
func (s *AppService) EnsureAppSupportsEnv(ctx context.Context, appID, env string) error {
	app, err := s.store.GetApp(ctx, appID)
	if err != nil {
		return err
	}

	if !app.ContainsEnvironment(env) {
		return model.ErrEnvironmentNotSupported(appID, env)
	}
	return nil
}

// DefaultApps returns a list of default applications for initialization.
func (s *AppService) DefaultApps() []*model.Application {
	return []*model.Application{
		model.NewApplication("default-service", "Default Service",
			"Default service for general configuration", "system"),
	}
}

// TimeNow is a variable that returns the current time, allowing for testing.
var TimeNow = time.Now
