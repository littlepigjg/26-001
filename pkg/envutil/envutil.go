// Package envutil provides environment variable utilities.
package envutil

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Get returns the value of an environment variable, or the default value if not set.
func Get(key, defaultVal string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	return v
}

// GetBool returns a boolean environment variable.
func GetBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

// GetInt returns an integer environment variable.
func GetInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return i
}

// GetFloat64 returns a float64 environment variable.
func GetFloat64(key string, defaultVal float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// GetDuration returns a time.Duration environment variable.
func GetDuration(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultVal
	}
	return d
}

// GetList returns a comma-separated list of strings from an environment variable.
func GetList(key string, defaultVal []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// GetRequired returns the value of a required environment variable.
// It panics if the variable is not set.
func GetRequired(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required environment variable " + key + " is not set")
	}
	return v
}

// Set sets an environment variable and returns a restore function.
func Set(key, value string) func() {
	oldVal, existed := os.LookupEnv(key)
	os.Setenv(key, value)
	return func() {
		if existed {
			os.Setenv(key, oldVal)
		} else {
			os.Unsetenv(key)
		}
	}
}

// IsSet checks if an environment variable is set.
func IsSet(key string) bool {
	_, exists := os.LookupEnv(key)
	return exists
}

// Clear removes an environment variable.
func Clear(key string) {
	os.Unsetenv(key)
}
