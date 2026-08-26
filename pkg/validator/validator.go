// Package validator provides a configurable validation framework for request parameters.
// It supports multiple validation rules that can be composed together.
package validator

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationRule is a function that validates a value and returns an error if validation fails.
type ValidationRule func(value interface{}) error

// Validator holds a set of validation rules for a field.
type Validator struct {
	fieldName string
	rules     []ValidationRule
	optional  bool
}

// New creates a new Validator for the given field name.
func New(fieldName string) *Validator {
	return &Validator{
		fieldName: fieldName,
		rules:     make([]ValidationRule, 0),
	}
}

// Optional marks the field as optional. If the value is nil or empty, validation passes.
func (v *Validator) Optional() *Validator {
	v.optional = true
	return v
}

// Required adds a rule that requires the value to be non-nil and non-empty.
func (v *Validator) Required() *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if value == nil {
			return fmt.Errorf("field '%s' is required", v.fieldName)
		}
		if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
			return fmt.Errorf("field '%s' is required and cannot be empty", v.fieldName)
		}
		return nil
	})
	return v
}

// String adds a rule that validates the value is a string.
func (v *Validator) String() *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if value == nil {
			if v.optional {
				return nil
			}
			return fmt.Errorf("field '%s' must be a string", v.fieldName)
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field '%s' must be a string", v.fieldName)
		}
		return nil
	})
	return v
}

// MinLength adds a rule that validates the string has at least n characters.
func (v *Validator) MinLength(n int) *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok {
			if len(str) < n {
				return fmt.Errorf("field '%s' must be at least %d characters", v.fieldName, n)
			}
		}
		return nil
	})
	return v
}

// MaxLength adds a rule that validates the string has at most n characters.
func (v *Validator) MaxLength(n int) *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok {
			if len(str) > n {
				return fmt.Errorf("field '%s' must be at most %d characters", v.fieldName, n)
			}
		}
		return nil
	})
	return v
}

// Length adds a rule that validates the string length is between min and max.
func (v *Validator) Length(min, max int) *Validator {
	return v.MinLength(min).MaxLength(max)
}

// Pattern adds a rule that validates the string matches a regex pattern.
func (v *Validator) Pattern(pattern string) *Validator {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(fmt.Sprintf("invalid regex pattern for field '%s': %s", v.fieldName, err.Error()))
	}
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok {
			if !re.MatchString(str) {
				return fmt.Errorf("field '%s' has invalid format", v.fieldName)
			}
		}
		return nil
	})
	return v
}

// Email adds a rule that validates the string is a valid email address.
func (v *Validator) Email() *Validator {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok && str != "" {
			if !emailRegex.MatchString(str) {
				return fmt.Errorf("field '%s' must be a valid email address", v.fieldName)
			}
		}
		return nil
	})
	return v
}

// InList adds a rule that validates the string value is one of the allowed values.
func (v *Validator) InList(allowedValues ...string) *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok {
			for _, allowed := range allowedValues {
				if str == allowed {
					return nil
				}
			}
			return fmt.Errorf("field '%s' must be one of: %s", v.fieldName, strings.Join(allowedValues, ", "))
		}
		return nil
	})
	return v
}

// NotEmpty adds a rule that validates a string is not empty after trimming whitespace.
func (v *Validator) NotEmpty() *Validator {
	v.rules = append(v.rules, func(value interface{}) error {
		if str, ok := value.(string); ok {
			if strings.TrimSpace(str) == "" {
				return fmt.Errorf("field '%s' must not be empty", v.fieldName)
			}
		}
		return nil
	})
	return v
}

// Validate runs all validation rules against the given value.
func (v *Validator) Validate(value interface{}) error {
	if v.optional && isEmpty(value) {
		return nil
	}

	for _, rule := range v.rules {
		if err := rule(value); err != nil {
			return err
		}
	}
	return nil
}

// isEmpty checks if a value is considered empty (nil or empty string).
func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str) == ""
	}
	return false
}

// ValidateMap validates multiple fields in a map using the provided validators.
// It returns a map of field name to error message for all validation failures.
func ValidateMap(data map[string]interface{}, validators map[string]*Validator) map[string]string {
	errors := make(map[string]string)
	for field, validator := range validators {
		value, exists := data[field]
		if !exists {
			value = nil
		}
		if err := validator.Validate(value); err != nil {
			errors[field] = err.Error()
		}
	}
	return errors
}

// HasErrors checks if the validation errors map contains any errors.
func HasErrors(errors map[string]string) bool {
	return len(errors) > 0
}

// ErrorString formats the validation errors into a single string.
func ErrorString(errors map[string]string) string {
	if len(errors) == 0 {
		return ""
	}
	var parts []string
	for field, msg := range errors {
		parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(parts, "; ")
}
