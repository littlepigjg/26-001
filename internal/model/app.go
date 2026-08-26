// Package model defines the data structures used throughout the configuration center.
// It includes entities for applications, configurations, versions, audit logs, and errors.
package model

import (
	"fmt"
	"time"
)

// Application represents a managed application in the configuration center.
// Applications are the top-level organizational unit for configurations.
type Application struct {
	// ID is the unique identifier for the application.
	ID string `json:"id"`
	// Name is the display name of the application.
	Name string `json:"name"`
	// Description provides a human-readable description of the application.
	Description string `json:"description"`
	// Owner is the person or team responsible for this application.
	Owner string `json:"owner"`
	// Environments lists the environments this application supports.
	Environments []string `json:"environments"`
	// CreatedAt records when the application was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt records when the application was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewApplication creates a new Application with default values.
func NewApplication(id, name, description, owner string) *Application {
	now := time.Now()
	return &Application{
		ID:           id,
		Name:         name,
		Description:  description,
		Owner:        owner,
		Environments: []string{"dev", "test", "prod"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Validate checks if the application fields are valid.
func (a *Application) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("application ID is required")
	}
	if a.Name == "" {
		return fmt.Errorf("application name is required")
	}
	if len(a.Name) > 64 {
		return fmt.Errorf("application name must be at most 64 characters")
	}
	if a.Owner == "" {
		return fmt.Errorf("application owner is required")
	}
	return nil
}

// ContainsEnvironment checks if the application supports a given environment.
func (a *Application) ContainsEnvironment(env string) bool {
	for _, e := range a.Environments {
		if e == env {
			return true
		}
	}
	return false
}

// AddEnvironment adds an environment to the application if not already present.
func (a *Application) AddEnvironment(env string) {
	if !a.ContainsEnvironment(env) {
		a.Environments = append(a.Environments, env)
		a.UpdatedAt = time.Now()
	}
}

// RemoveEnvironment removes an environment from the application.
func (a *Application) RemoveEnvironment(env string) {
	for i, e := range a.Environments {
		if e == env {
			a.Environments = append(a.Environments[:i], a.Environments[i+1:]...)
			a.UpdatedAt = time.Now()
			return
		}
	}
}
