// Package diff provides utilities for computing differences between configuration
// versions. It supports line-by-line diffs and key-value comparison.
package diff

import (
	"fmt"
	"strings"
)

// ChangeType represents the type of change detected.
type ChangeType int

const (
	// ChangeAdded indicates a key-value pair was added.
	ChangeAdded ChangeType = iota
	// ChangeModified indicates a key-value pair was modified.
	ChangeModified
	// ChangeRemoved indicates a key-value pair was removed.
	ChangeRemoved
	// ChangeUnchanged indicates no change.
	ChangeUnchanged
)

// String returns the string representation of a ChangeType.
func (ct ChangeType) String() string {
	switch ct {
	case ChangeAdded:
		return "added"
	case ChangeModified:
		return "modified"
	case ChangeRemoved:
		return "removed"
	default:
		return "unchanged"
	}
}

// Change represents a single change between two versions of a config entry.
type Change struct {
	// Key is the configuration key that changed.
	Key string `json:"key"`
	// Type is the type of change (added, modified, removed).
	Type ChangeType `json:"type"`
	// OldValue is the previous value (empty for added).
	OldValue string `json:"old_value,omitempty"`
	// NewValue is the new value (empty for removed).
	NewValue string `json:"new_value,omitempty"`
}

// String returns a human-readable description of the change.
func (c Change) String() string {
	switch c.Type {
	case ChangeAdded:
		return fmt.Sprintf("+ %s = %s", c.Key, c.NewValue)
	case ChangeModified:
		return fmt.Sprintf("~ %s: %s -> %s", c.Key, c.OldValue, c.NewValue)
	case ChangeRemoved:
		return fmt.Sprintf("- %s = %s", c.Key, c.OldValue)
	default:
		return fmt.Sprintf("  %s", c.Key)
	}
}

// Diff compares two maps of configuration key-value pairs and returns the list of changes.
// oldConfig is the previous version, newConfig is the current version.
func Diff(oldConfig, newConfig map[string]string) []Change {
	changes := make([]Change, 0)

	// Check for added and modified keys
	for key, newValue := range newConfig {
		oldValue, exists := oldConfig[key]
		if !exists {
			changes = append(changes, Change{
				Key:      key,
				Type:     ChangeAdded,
				NewValue: newValue,
			})
		} else if oldValue != newValue {
			changes = append(changes, Change{
				Key:      key,
				Type:     ChangeModified,
				OldValue: oldValue,
				NewValue: newValue,
			})
		}
	}

	// Check for removed keys
	for key, oldValue := range oldConfig {
		if _, exists := newConfig[key]; !exists {
			changes = append(changes, Change{
				Key:      key,
				Type:     ChangeRemoved,
				OldValue: oldValue,
			})
		}
	}

	return changes
}

// DiffString produces a unified diff string between two config maps.
func DiffString(oldConfig, newConfig map[string]string) string {
	changes := Diff(oldConfig, newConfig)
	if len(changes) == 0 {
		return "(no changes)"
	}
	var sb strings.Builder
	for _, c := range changes {
		sb.WriteString(c.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// DiffCount returns the number of changes between two config maps, grouped by type.
func DiffCount(oldConfig, newConfig map[string]string) (added, modified, removed int) {
	changes := Diff(oldConfig, newConfig)
	for _, c := range changes {
		switch c.Type {
		case ChangeAdded:
			added++
		case ChangeModified:
			modified++
		case ChangeRemoved:
			removed++
		}
	}
	return
}

// PatchResult represents the result of applying a patch.
type PatchResult struct {
	// Success indicates whether the patch was applied successfully.
	Success bool
	// Changes contains the list of changes made.
	Changes []Change
	// Error contains any error message.
	Error string
}

// ApplyDiff applies a list of changes to a config map, returning the updated map.
func ApplyDiff(config map[string]string, changes []Change) map[string]string {
	result := make(map[string]string, len(config))
	for k, v := range config {
		result[k] = v
	}
	for _, change := range changes {
		switch change.Type {
		case ChangeAdded, ChangeModified:
			result[change.Key] = change.NewValue
		case ChangeRemoved:
			delete(result, change.Key)
		}
	}
	return result
}

// LineDiff computes a simple line-by-line diff between two strings.
// It returns the lines that were added, removed, and unchanged.
func LineDiff(oldStr, newStr string) (added, removed, unchanged []string) {
	oldLines := splitLines(oldStr)
	newLines := splitLines(newStr)

	// Build a set of old lines
	oldSet := make(map[string]int)
	for _, line := range oldLines {
		oldSet[line]++
	}

	// Find added and removed lines
	newSet := make(map[string]int)
	for _, line := range newLines {
		newSet[line]++
	}

	for _, line := range newLines {
		if oldSet[line] > 0 {
			oldSet[line]--
		} else {
			added = append(added, line)
		}
	}

	for _, line := range oldLines {
		if newSet[line] > 0 {
			newSet[line]--
		} else {
			removed = append(removed, line)
		}
	}

	for _, line := range oldLines {
		if newSet[line] > 0 {
			unchanged = append(unchanged, line)
			newSet[line]--
		}
	}

	return
}

// splitLines splits a string into lines, preserving the original content.
func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// HasChanges checks if there are any differences between two config maps.
func HasChanges(oldConfig, newConfig map[string]string) bool {
	added, modified, removed := DiffCount(oldConfig, newConfig)
	return added > 0 || modified > 0 || removed > 0
}

// DiffCountFromChanges counts changes from a pre-computed change list.
func DiffCountFromChanges(changes []Change) (added, modified, removed int) {
	for _, c := range changes {
		switch c.Type {
		case ChangeAdded:
			added++
		case ChangeModified:
			modified++
		case ChangeRemoved:
			removed++
		}
	}
	return
}
