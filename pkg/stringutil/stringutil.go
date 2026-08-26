// Package stringutil provides string utility functions.
package stringutil

import (
	"regexp"
	"strings"
)

// Truncate truncates a string to the specified length, adding an ellipsis if truncated.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Contains checks if s contains substr (case-insensitive).
func Contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// HasPrefix checks if s starts with prefix (case-insensitive).
func HasPrefix(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}

// HasSuffix checks if s ends with suffix (case-insensitive).
func HasSuffix(s, suffix string) bool {
	return strings.HasSuffix(strings.ToLower(s), strings.ToLower(suffix))
}

// CamelToSnake converts a CamelCase string to snake_case.
func CamelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// SnakeToCamel converts a snake_case string to CamelCase.
func SnakeToCamel(s string) string {
	parts := strings.Split(strings.ToLower(s), "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// KebabToCamel converts a kebab-case string to CamelCase.
func KebabToCamel(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	return SnakeToCamel(s)
}

// CamelToKebab converts a CamelCase string to kebab-case.
func CamelToKebab(s string) string {
	s = CamelToSnake(s)
	return strings.ReplaceAll(s, "_", "-")
}

// RemoveWhitespace removes all whitespace characters from a string.
func RemoveWhitespace(s string) string {
	reg := regexp.MustCompile(`\s+`)
	return reg.ReplaceAllString(s, "")
}

// NormalizeSpace normalizes whitespace in a string (trims and collapses multiple spaces).
func NormalizeSpace(s string) string {
	reg := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(reg.ReplaceAllString(s, " "))
}

// IsEmpty checks if a string is empty or contains only whitespace.
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// DefaultIfEmpty returns the default value if the string is empty.
func DefaultIfEmpty(s, defaultVal string) string {
	if IsEmpty(s) {
		return defaultVal
	}
	return s
}

// PadLeft pads a string on the left to the specified length.
func PadLeft(s string, padChar string, length int) string {
	if len(s) >= length {
		return s
	}
	padLen := length - len(s)
	pad := strings.Repeat(padChar, padLen/len(padChar))
	if padLen%len(padChar) > 0 {
		pad += padChar[:padLen%len(padChar)]
	}
	return pad + s
}

// PadRight pads a string on the right to the specified length.
func PadRight(s string, padChar string, length int) string {
	if len(s) >= length {
		return s
	}
	padLen := length - len(s)
	pad := strings.Repeat(padChar, padLen/len(padChar))
	if padLen%len(padChar) > 0 {
		pad += padChar[:padLen%len(padChar)]
	}
	return s + pad
}

// PadCenter pads a string on both sides to the specified length.
func PadCenter(s string, padChar string, length int) string {
	if len(s) >= length {
		return s
	}
	totalPad := length - len(s)
	leftPad := totalPad / 2
	rightPad := totalPad - leftPad
	return PadLeft(PadRight(s, padChar, len(s)+rightPad), padChar, length)
}

// Reverse reverses a string.
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// IsPalindrome checks if a string is a palindrome (case-insensitive, ignoring non-alphanumeric).
func IsPalindrome(s string) bool {
	s = strings.ToLower(RemoveNonAlphaNumeric(s))
	return s == Reverse(s)
}

// RemoveNonAlphaNumeric removes all non-alphanumeric characters.
func RemoveNonAlphaNumeric(s string) string {
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		}
	}
	return result.String()
}
