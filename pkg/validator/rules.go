package validator

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Numeric adds a rule that validates the value can be parsed as a number.
func (v *Validator) Numeric() *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if value == nil {
			if v.optional {
				return nil
			}
			return fmt.Errorf("field '%s' must be a number", v.fieldName)
		}
		switch val := value.(type) {
		case string:
			if _, err := strconv.ParseFloat(val, 64); err != nil {
				return fmt.Errorf("field '%s' must be a valid number", v.fieldName)
			}
		case float64, float32, int, int64, int32:
			// Already numeric
		default:
			return fmt.Errorf("field '%s' must be a number", v.fieldName)
		}
		return nil
	})
	return v
}

// Min adds a rule that validates a numeric value is at least min.
func (v *Validator) Min(min float64) *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if value == nil {
			return nil
		}
		var num float64
		switch val := value.(type) {
		case float64:
			num = val
		case int:
			num = float64(val)
		case int64:
			num = float64(val)
		case string:
			var err error
			num, err = strconv.ParseFloat(val, 64)
			if err != nil {
				return nil
			}
		default:
			return nil
		}
		if num < min {
			return fmt.Errorf("field '%s' must be at least %v", v.fieldName, min)
		}
		return nil
	})
	return v
}

// Max adds a rule that validates a numeric value is at most max.
func (v *Validator) Max(max float64) *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if value == nil {
			return nil
		}
		var num float64
		switch val := value.(type) {
		case float64:
			num = val
		case int:
			num = float64(val)
		case int64:
			num = float64(val)
		case string:
			var err error
			num, err = strconv.ParseFloat(val, 64)
			if err != nil {
				return nil
			}
		default:
			return nil
		}
		if num > max {
			return fmt.Errorf("field '%s' must be at most %v", v.fieldName, max)
		}
		return nil
	})
	return v
}

// URL adds a rule that validates the string is a valid URL.
func (v *Validator) URL() *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok && str != "" {
			if _, err := url.ParseRequestURI(str); err != nil {
				return fmt.Errorf("field '%s' must be a valid URL", v.fieldName)
			}
		}
		return nil
	})
	return v
}

// HasPrefix adds a rule that validates the string starts with the given prefix.
func (v *Validator) HasPrefix(prefix string) *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok {
			if !strings.HasPrefix(str, prefix) {
				return fmt.Errorf("field '%s' must start with '%s'", v.fieldName, prefix)
			}
		}
		return nil
	})
	return v
}

// HasSuffix adds a rule that validates the string ends with the given suffix.
func (v *Validator) HasSuffix(suffix string) *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok {
			if !strings.HasSuffix(str, suffix) {
				return fmt.Errorf("field '%s' must end with '%s'", v.fieldName, suffix)
			}
		}
		return nil
	})
	return v
}

// Custom adds a custom validation rule provided by the caller.
func (v *Validator) Custom(rule ValidationRule) *Validator {
	v.rules = append(v.rules, rule)
	return v
}

// ValidateConfigKey validates a configuration key format.
// Keys must be non-empty strings of letters, digits, dots, underscores, and hyphens.
func ValidateConfigKey(key string) error {
	if key == "" {
		return fmt.Errorf("config key cannot be empty")
	}
	if len(key) > 255 {
		return fmt.Errorf("config key must be at most 255 characters")
	}
	for _, ch := range key {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '.' || ch == '_' || ch == '-') {
			return fmt.Errorf("config key contains invalid character: %c", ch)
		}
	}
	return nil
}

// ValidateConfigValue validates a configuration value.
func ValidateConfigValue(value string) error {
	if len(value) > 1048576 { // 1MB limit
		return fmt.Errorf("config value must be at most 1MB")
	}
	return nil
}

// ValidateAppName validates an application name.
func ValidateAppName(name string) error {
	if name == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("app name must be at most 64 characters")
	}
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.') {
			return fmt.Errorf("app name contains invalid character: %c", ch)
		}
	}
	return nil
}

// ValidateEnvironment validates an environment name.
func ValidateEnvironment(env string) error {
	allowed := map[string]bool{
		"dev": true, "test": true, "staging": true, "prod": true,
		"development": true, "testing": true, "production": true,
	}
	if !allowed[strings.ToLower(env)] {
		return fmt.Errorf("invalid environment: %s (allowed: dev, test, staging, prod)", env)
	}
	return nil
}
