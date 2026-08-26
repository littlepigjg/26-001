package model

import (
	"fmt"
	"strings"
)

// EnvironmentInfo describes an environment in the system.
type EnvironmentInfo struct {
	// Name is the environment identifier.
	Name string `json:"name"`
	// DisplayName is a human-readable name.
	DisplayName string `json:"display_name"`
	// Description describes the purpose of this environment.
	Description string `json:"description"`
	// IsDefault indicates if this is the default environment.
	IsDefault bool `json:"is_default"`
	// Order is the display ordering.
	Order int `json:"order"`
}

// Predefined environments.
var (
	// EnvDev is the development environment.
	EnvDev = EnvironmentInfo{
		Name:        "dev",
		DisplayName: "Development",
		Description: "Development environment for testing new features",
		IsDefault:   true,
		Order:       1,
	}
	// EnvTest is the testing environment.
	EnvTest = EnvironmentInfo{
		Name:        "test",
		DisplayName: "Testing",
		Description: "Testing environment for integration testing",
		IsDefault:   false,
		Order:       2,
	}
	// EnvStaging is the staging environment.
	EnvStaging = EnvironmentInfo{
		Name:        "staging",
		DisplayName: "Staging",
		Description: "Staging environment for pre-production verification",
		IsDefault:   false,
		Order:       3,
	}
	// EnvProd is the production environment.
	EnvProd = EnvironmentInfo{
		Name:        "prod",
		DisplayName: "Production",
		Description: "Production environment for live traffic",
		IsDefault:   false,
		Order:       4,
	}
)

// AllEnvironments returns all predefined environments.
func AllEnvironments() []EnvironmentInfo {
	return []EnvironmentInfo{EnvDev, EnvTest, EnvStaging, EnvProd}
}

// GetEnvironment returns environment info by name.
func GetEnvironment(name string) (EnvironmentInfo, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, env := range AllEnvironments() {
		if env.Name == name {
			return env, nil
		}
	}
	return EnvironmentInfo{}, fmt.Errorf("unknown environment: %s", name)
}

// ValidEnvironments returns a list of valid environment names.
func ValidEnvironments() []string {
	envs := AllEnvironments()
	names := make([]string, len(envs))
	for i, env := range envs {
		names[i] = env.Name
	}
	return names
}

// IsValidEnvironment checks if a string is a valid environment name.
func IsValidEnvironment(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, env := range AllEnvironments() {
		if env.Name == name {
			return true
		}
	}
	return false
}
