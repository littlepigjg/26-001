package model

import (
	"fmt"
	"time"
)

// Version represents a snapshot of the configuration at a point in time.
// Each change to configuration creates a new version.
type Version struct {
	// ID is the unique identifier for this version.
	ID string `json:"id"`
	// AppID is the application this version belongs to.
	AppID string `json:"app_id"`
	// Environment is the target environment.
	Environment string `json:"environment"`
	// VersionNumber is the sequential version number (1-based).
	VersionNumber int `json:"version_number"`
	// ConfigHash is the hash of the configuration at this version.
	ConfigHash string `json:"config_hash"`
	// ConfigData is the full configuration snapshot.
	ConfigData map[string]string `json:"config_data"`
	// ChangeSummary describes what changed in this version.
	ChangeSummary string `json:"change_summary"`
	// ChangedBy is the user who created this version.
	ChangedBy string `json:"changed_by"`
	// ParentVersion is the previous version number (0 for initial).
	ParentVersion int `json:"parent_version"`
	// CreatedAt records when this version was created.
	CreatedAt time.Time `json:"created_at"`
}

// VersionInfo is a summary of a version (without the full config data).
type VersionInfo struct {
	// ID is the version identifier.
	ID string `json:"id"`
	// VersionNumber is the sequential version number.
	VersionNumber int `json:"version_number"`
	// ConfigHash is the hash of the configuration.
	ConfigHash string `json:"config_hash"`
	// ChangeSummary describes the changes.
	ChangeSummary string `json:"change_summary"`
	// ChangedBy is the user who made the change.
	ChangedBy string `json:"changed_by"`
	// ParentVersion is the previous version number.
	ParentVersion int `json:"parent_version"`
	// CreatedAt is when this version was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewVersion creates a new Version from the current configuration.
func NewVersion(id, appID, env, changedBy, summary string, configData map[string]string, versionNumber, parentVersion int, configHash string) *Version {
	return &Version{
		ID:            id,
		AppID:         appID,
		Environment:   env,
		VersionNumber: versionNumber,
		ConfigHash:    configHash,
		ConfigData:    configData,
		ChangeSummary: summary,
		ChangedBy:     changedBy,
		ParentVersion: parentVersion,
		CreatedAt:     time.Now(),
	}
}

// Validate checks if the version fields are valid.
func (v *Version) Validate() error {
	if v.AppID == "" {
		return fmt.Errorf("app_id is required")
	}
	if v.Environment == "" {
		return fmt.Errorf("environment is required")
	}
	if v.VersionNumber < 1 {
		return fmt.Errorf("version_number must be at least 1")
	}
	if v.ConfigData == nil {
		return fmt.Errorf("config_data cannot be nil")
	}
	return nil
}

// ToInfo converts a Version to a VersionInfo (without the full config data).
func (v *Version) ToInfo() VersionInfo {
	return VersionInfo{
		ID:            v.ID,
		VersionNumber: v.VersionNumber,
		ConfigHash:    v.ConfigHash,
		ChangeSummary: v.ChangeSummary,
		ChangedBy:     v.ChangedBy,
		ParentVersion: v.ParentVersion,
		CreatedAt:     v.CreatedAt,
	}
}

// VersionList is a list of versions with pagination.
type VersionList struct {
	// AppID is the application identifier.
	AppID string `json:"app_id"`
	// Environment is the target environment.
	Environment string `json:"environment"`
	// Versions is the list of version info summaries.
	Versions []VersionInfo `json:"versions"`
	// Total is the total number of versions.
	Total int `json:"total"`
}
