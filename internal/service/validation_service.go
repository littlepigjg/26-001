package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// ValidationService validates configuration values against defined rules.
// It supports format-specific validation (JSON, YAML, number, boolean, etc.).
type ValidationService struct {
	store  store.Store
	appSvc *AppService
	logger *logger.Logger
}

// NewValidationService creates a new ValidationService.
func NewValidationService(s store.Store, appSvc *AppService) *ValidationService {
	return &ValidationService{
		store:  s,
		appSvc: appSvc,
		logger: logger.WithField("service", "validation"),
	}
}

// ValidationResult contains the results of a validation operation.
type ValidationResult struct {
	// Valid indicates if all validations passed.
	Valid bool `json:"valid"`
	// Errors contains validation errors by field.
	Errors map[string][]string `json:"errors,omitempty"`
	// Warnings contains non-fatal warnings.
	Warnings map[string][]string `json:"warnings,omitempty"`
	// Summary provides a human-readable summary.
	Summary string `json:"summary"`
}

// NewValidationResult creates a new ValidationResult.
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:    true,
		Errors:   make(map[string][]string),
		Warnings: make(map[string][]string),
	}
}

// AddError adds a validation error for a specific field.
func (r *ValidationResult) AddError(field, message string) {
	r.Valid = false
	r.Errors[field] = append(r.Errors[field], message)
}

// AddWarning adds a warning for a specific field.
func (r *ValidationResult) AddWarning(field, message string) {
	r.Warnings[field] = append(r.Warnings[field], message)
}

// Finalize sets the summary message.
func (r *ValidationResult) Finalize() {
	if r.Valid {
		if len(r.Warnings) > 0 {
			r.Summary = fmt.Sprintf("validation passed with %d warning(s)", len(r.Warnings))
		} else {
			r.Summary = "validation passed"
		}
	} else {
		r.Summary = fmt.Sprintf("validation failed with %d error(s)", len(r.Errors))
	}
}

// ValidateConfigItem validates a single configuration item.
func (s *ValidationService) ValidateConfigItem(ctx context.Context, appID, env, key, value, format string) *ValidationResult {
	result := NewValidationResult()

	// Validate key
	if err := validateConfigKey(key); err != nil {
		result.AddError("key", err.Error())
	}

	// Validate format-specific value
	if format != "" {
		if err := validateValueFormat(value, format); err != nil {
			result.AddError("value", err.Error())
		}
	}

	// Validate required fields
	if strings.TrimSpace(value) == "" {
		result.AddWarning("value", "value is empty")
	}

	result.Finalize()
	return result
}

// ValidateConfigMap validates an entire configuration map.
func (s *ValidationService) ValidateConfigMap(ctx context.Context, appID, env string, configs map[string]string) *ValidationResult {
	result := NewValidationResult()

	// Check for required keys
	requiredKeys := s.getRequiredKeys(ctx, appID, env)
	for _, reqKey := range requiredKeys {
		if _, exists := configs[reqKey]; !exists {
			result.AddError(reqKey, fmt.Sprintf("required key '%s' is missing", reqKey))
		}
	}

	// Validate each key-value pair
	for key, value := range configs {
		if err := validateConfigKey(key); err != nil {
			result.AddError(key, err.Error())
		}
		// Check value length
		if len(value) > 1048576 {
			result.AddError(key, "value exceeds maximum size of 1MB")
		}
	}

	// Check for empty keys or values
	for key, value := range configs {
		if key == "" {
			result.AddError("_empty", "config key cannot be empty")
		}
		if strings.TrimSpace(value) == "" {
			result.AddWarning(key, "value is empty")
		}
	}

	result.Finalize()
	return result
}

// ValidateConfigValue validates a configuration value against common patterns.
func (s *ValidationService) ValidateConfigValue(ctx context.Context, value, format string) error {
	return validateValueFormat(value, format)
}

// getRequiredKeys returns the list of required keys for an app/env.
func (s *ValidationService) getRequiredKeys(_ context.Context, appID, env string) []string {
	// In a full implementation, this would be stored in the app model.
	// For now, return an empty list (no required keys by default).
	return []string{}
}

// validateConfigKey validates a configuration key format.
func validateConfigKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if len(key) > 255 {
		return fmt.Errorf("key must be at most 255 characters")
	}
	validKey := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._\-]*$`)
	if !validKey.MatchString(key) {
		return fmt.Errorf("key must start with a letter and contain only letters, digits, dots, underscores, and hyphens")
	}
	return nil
}

// validateValueFormat validates a value against a specific format.
func validateValueFormat(value, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return validateJSON(value)
	case "number":
		return validateNumber(value)
	case "boolean":
		return validateBoolean(value)
	case "url":
		return validateURL(value)
	case "email":
		return validateEmail(value)
	case "regex":
		return validateRegex(value)
	case "string", "":
		// String format is always valid
		return nil
	default:
		// Unknown format, treat as string
		return nil
	}
}

// validateJSON checks if a value is valid JSON.
func validateJSON(value string) error {
	if value == "" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// validateNumber checks if a value is a valid number.
func validateNumber(value string) error {
	if value == "" {
		return nil
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			if ch == '.' || ch == '-' || ch == '+' || ch == 'e' || ch == 'E' {
				continue
			}
			return fmt.Errorf("value '%s' is not a valid number", value)
		}
	}
	return nil
}

// validateBoolean checks if a value is a valid boolean.
func validateBoolean(value string) error {
	valid := map[string]bool{
		"true": true, "false": true,
		"1": true, "0": true,
		"yes": true, "no": true,
		"on": true, "off": true,
	}
	if !valid[strings.ToLower(strings.TrimSpace(value))] {
		return fmt.Errorf("value '%s' is not a valid boolean", value)
	}
	return nil
}

// validateURL checks if a value is a valid URL.
func validateURL(value string) error {
	if value == "" {
		return nil
	}
	urlPattern := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	if !urlPattern.MatchString(value) {
		return fmt.Errorf("value '%s' is not a valid URL", value)
	}
	return nil
}

// validateEmail checks if a value is a valid email address.
func validateEmail(value string) error {
	if value == "" {
		return nil
	}
	emailPattern := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailPattern.MatchString(value) {
		return fmt.Errorf("value '%s' is not a valid email", value)
	}
	return nil
}

// validateRegex checks if a value is a valid regex pattern.
func validateRegex(value string) error {
	if value == "" {
		return nil
	}
	if _, err := regexp.Compile(value); err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	return nil
}

// FormatDescriptions returns descriptions for all supported formats.
func FormatDescriptions() map[string]string {
	return map[string]string{
		"string":  "Plain text string",
		"json":    "JSON object or array",
		"yaml":    "YAML document",
		"number":  "Numeric value (integer or decimal)",
		"boolean": "Boolean value (true/false, yes/no, 1/0)",
		"xml":     "XML document",
		"url":     "URL (http/https)",
		"email":   "Email address",
		"regex":   "Regular expression pattern",
	}
}

// ValidateApp validates an application's configuration integrity.
func (s *ValidationService) ValidateApp(ctx context.Context, appID, env string) *ValidationResult {
	result := NewValidationResult()

	// Check app exists
	app, err := s.appSvc.GetApp(ctx, appID)
	if err != nil {
		result.AddError("app", fmt.Sprintf("application '%s' not found", appID))
		return result
	}

	// Check environment
	if !app.ContainsEnvironment(env) {
		result.AddError("environment", fmt.Sprintf("environment '%s' not supported for app '%s'", env, appID))
		return result
	}

	// Get config and validate
	configData, err := s.store.GetConfigMap(ctx, appID, env)
	if err != nil {
		result.AddError("config", fmt.Sprintf("failed to load config: %v", err))
		return result
	}

	innerResult := s.ValidateConfigMap(ctx, appID, env, configData)
	for k, v := range innerResult.Errors {
		for _, msg := range v {
			result.AddError(k, msg)
		}
	}
	for k, v := range innerResult.Warnings {
		for _, msg := range v {
			result.AddWarning(k, msg)
		}
	}

	result.Finalize()
	return result
}

// Ensure model import is used
var _ = model.ErrValidationFailed
