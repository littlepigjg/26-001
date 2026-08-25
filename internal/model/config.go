package model

import (
	"fmt"
	"time"
)

// ConfigItem represents a single configuration key-value pair for a specific application and environment.
type ConfigItem struct {
	// ID is the unique identifier for this config item.
	ID string `json:"id"`
	// AppID is the application this config belongs to.
	AppID string `json:"app_id"`
	// Environment is the target environment (dev, test, prod).
	Environment string `json:"environment"`
	// Key is the configuration key.
	Key string `json:"key"`
	// Value is the configuration value.
	Value string `json:"value"`
	// Description describes the purpose of this configuration.
	Description string `json:"description"`
	// Format specifies the expected format (string, json, yaml, number, etc.).
	Format string `json:"format"`
	// Required indicates if this config must be present.
	Required bool `json:"required"`
	// Version is the current version number of this config.
	Version int `json:"version"`
	// UpdatedBy records who last modified this config.
	UpdatedBy string `json:"updated_by"`
	// CreatedAt records when the config was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt records when the config was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// ConfigItemList is a list of ConfigItems for a specific app and environment.
type ConfigItemList struct {
	// AppID is the application identifier.
	AppID string `json:"app_id"`
	// Environment is the target environment.
	Environment string `json:"environment"`
	// Items contains all config items for this app/env.
	Items []ConfigItem `json:"items"`
	// Version is the current version hash.
	Version string `json:"version"`
	// UpdatedAt is the last update time.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewConfigItem creates a new ConfigItem.
func NewConfigItem(id, appID, env, key, value, description, format, updatedBy string) *ConfigItem {
	now := time.Now()
	return &ConfigItem{
		ID:          id,
		AppID:       appID,
		Environment: env,
		Key:         key,
		Value:       value,
		Description: description,
		Format:      format,
		Required:    false,
		Version:     1,
		UpdatedBy:   updatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Validate checks if the config item fields are valid.
func (c *ConfigItem) Validate() error {
	if c.AppID == "" {
		return fmt.Errorf("app_id is required")
	}
	if c.Environment == "" {
		return fmt.Errorf("environment is required")
	}
	if c.Key == "" {
		return fmt.Errorf("key is required")
	}
	if len(c.Key) > 255 {
		return fmt.Errorf("key must be at most 255 characters")
	}
	if len(c.Value) > 1048576 {
		return fmt.Errorf("value must be at most 1MB")
	}
	if c.Format != "" {
		validFormats := map[string]bool{
			"string": true, "json": true, "yaml": true,
			"number": true, "boolean": true, "xml": true,
		}
		if !validFormats[c.Format] {
			return fmt.Errorf("invalid format: %s", c.Format)
		}
	}
	return nil
}

// ToMap converts a ConfigItemList to a simple map of key-value pairs.
func (cl *ConfigItemList) ToMap() map[string]string {
	m := make(map[string]string, len(cl.Items))
	for _, item := range cl.Items {
		m[item.Key] = item.Value
	}
	return m
}

// Keys returns all keys in the config item list.
func (cl *ConfigItemList) Keys() []string {
	keys := make([]string, 0, len(cl.Items))
	for _, item := range cl.Items {
		keys = append(keys, item.Key)
	}
	return keys
}

// FindKey looks up a config item by key.
func (cl *ConfigItemList) FindKey(key string) (*ConfigItem, bool) {
	for i := range cl.Items {
		if cl.Items[i].Key == key {
			return &cl.Items[i], true
		}
	}
	return nil, false
}

// DeleteKey removes a config item by key.
func (cl *ConfigItemList) DeleteKey(key string) bool {
	for i, item := range cl.Items {
		if item.Key == key {
			cl.Items = append(cl.Items[:i], cl.Items[i+1:]...)
			return true
		}
	}
	return false
}
