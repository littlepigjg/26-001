package logger

import (
	"encoding/json"
	"fmt"
	"time"
)

// LogEntry represents a structured log entry for JSON output.
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Prefix    string                 `json:"prefix,omitempty"`
	Caller    string                 `json:"caller,omitempty"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// JSONFormatter formats log entries as JSON strings.
type JSONFormatter struct{}

// Format converts a LogEntry to a JSON string.
func (f *JSONFormatter) Format(entry *LogEntry) string {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Sprintf(`{"timestamp":"%s","level":"ERROR","message":"Failed to format log entry: %s"}`,
			time.Now().Format(time.RFC3339), err.Error())
	}
	return string(data) + "\n"
}

// TextFormatter formats log entries as human-readable text.
type TextFormatter struct {
	// Color enables ANSI color codes in output.
	Color bool
}

// NewTextFormatter creates a new TextFormatter with the specified color setting.
func NewTextFormatter(color bool) *TextFormatter {
	return &TextFormatter{Color: color}
}

// Format converts a LogEntry to a formatted text string.
func (f *TextFormatter) Format(entry *LogEntry) string {
	var result string

	if f.Color {
		result = fmt.Sprintf("\x1b[36m[%s]\x1b[0m \x1b[32m[%s]\x1b[0m ",
			entry.Timestamp, entry.Level)
	} else {
		result = fmt.Sprintf("[%s] [%s] ", entry.Timestamp, entry.Level)
	}

	if entry.Prefix != "" {
		result += fmt.Sprintf("[%s] ", entry.Prefix)
	}

	if entry.Caller != "" {
		if f.Color {
			result += fmt.Sprintf("\x1b[33m[%s]\x1b[0m ", entry.Caller)
		} else {
			result += fmt.Sprintf("[%s] ", entry.Caller)
		}
	}

	result += entry.Message

	if len(entry.Fields) > 0 {
		result += " {"
		first := true
		for k, v := range entry.Fields {
			if !first {
				result += ", "
			}
			result += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		result += "}"
	}

	result += "\n"
	return result
}

// Formatter is the interface for log formatters.
type Formatter interface {
	Format(entry *LogEntry) string
}
