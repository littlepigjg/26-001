// Package logger provides a structured logging utility for the configuration center.
// It supports multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL) and outputs
// structured log entries with timestamps, caller information, and contextual fields.
package logger

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents the severity level of a log message.
type Level int

const (
	// LevelDebug is for development and debugging messages.
	LevelDebug Level = iota
	// LevelInfo is for general operational messages.
	LevelInfo
	// LevelWarn is for warning conditions that may need attention.
	LevelWarn
	// LevelError is for error conditions that need immediate attention.
	LevelError
	// LevelFatal is for critical errors that cause program termination.
	LevelFatal
)

// String returns the string representation of a log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel converts a string to a Level value.
func ParseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR", "ERR":
		return LevelError
	case "FATAL":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Logger is a structured logger with configurable output and level filtering.
type Logger struct {
	mu       sync.Mutex
	output   io.Writer
	level    Level
	prefix   string
	fields   map[string]interface{}
	showCall bool
}

// defaultLogger is the package-level default logger instance.
var defaultLogger = NewLogger(os.Stdout, LevelInfo, "config-center", true)

// NewLogger creates a new Logger instance.
// Parameters:
//   - output: the writer to write log output to
//   - level: the minimum log level to output
//   - prefix: a prefix string added to each log message
//   - showCall: whether to include caller file/line information
func NewLogger(output io.Writer, level Level, prefix string, showCall bool) *Logger {
	return &Logger{
		output:   output,
		level:    level,
		prefix:   prefix,
		fields:   make(map[string]interface{}),
		showCall: showCall,
	}
}

// Default returns the package-level default logger.
func Default() *Logger {
	return defaultLogger
}

// SetDefault replaces the package-level default logger.
func SetDefault(l *Logger) {
	if l != nil {
		defaultLogger = l
	}
}

// WithField returns a new Logger with the given field added to its context.
// The original logger is not modified.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	newLogger := &Logger{
		output:   l.output,
		level:    l.level,
		prefix:   l.prefix,
		fields:   make(map[string]interface{}, len(l.fields)+1),
		showCall: l.showCall,
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

// WithFields returns a new Logger with the given fields added to its context.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	newLogger := &Logger{
		output:   l.output,
		level:    l.level,
		prefix:   l.prefix,
		fields:   make(map[string]interface{}, len(l.fields)+len(fields)),
		showCall: l.showCall,
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// SetLevel changes the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput changes the output writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}

// Info logs a message at INFO level.
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(LevelError, msg, args...)
}

// Fatal logs a message at FATAL level and exits with code 1.
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(LevelFatal, msg, args...)
	os.Exit(1)
}

// Debugf logs a formatted message at DEBUG level.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(LevelDebug, fmt.Sprintf(format, args...))
}

// Infof logs a formatted message at INFO level.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(LevelInfo, fmt.Sprintf(format, args...))
}

// Warnf logs a formatted message at WARN level.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(LevelWarn, fmt.Sprintf(format, args...))
}

// Errorf logs a formatted message at ERROR level.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(LevelError, fmt.Sprintf(format, args...))
}

// Fatalf logs a formatted message at FATAL level and exits.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(LevelFatal, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// log performs the actual logging. It formats and writes the log entry.
func (l *Logger) log(level Level, msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	// Build the structured log entry
	timestamp := time.Now().Format(time.RFC3339)
	var caller string
	if l.showCall {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			// Shorten the file path
			parts := strings.Split(file, "/")
			if len(parts) > 2 {
				file = strings.Join(parts[len(parts)-2:], "/")
			}
			caller = fmt.Sprintf("%s:%d", file, line)
		}
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("[%s] [%s] [%s] ", timestamp, level.String(), l.prefix))

	if caller != "" {
		buf.WriteString(fmt.Sprintf("[%s] ", caller))
	}

	// Add structured fields
	if len(l.fields) > 0 {
		buf.WriteString("{")
		first := true
		for k, v := range l.fields {
			if !first {
				buf.WriteString(", ")
			}
			buf.WriteString(fmt.Sprintf("%s=%v", k, v))
			first = false
		}
		buf.WriteString("} ")
	}

	buf.WriteString(msg)
	if len(args) > 0 {
		buf.WriteString(" | ")
		buf.WriteString(fmt.Sprint(args...))
	}
	buf.WriteString("\n")

	// Best-effort write, ignore errors
	_, _ = l.output.Write([]byte(buf.String()))
}

// Package-level convenience functions that use the default logger.

// Debugf logs using the default logger at DEBUG level.
func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

// Infof logs using the default logger at INFO level.
func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

// Warnf logs using the default logger at WARN level.
func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

// Errorf logs using the default logger at ERROR level.
func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

// Fatalf logs using the default logger at FATAL level.
func Fatalf(format string, args ...interface{}) {
	defaultLogger.Fatalf(format, args...)
}

// WithField returns a child logger of the default logger with a single field.
func WithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

// WithFields returns a child logger of the default logger with multiple fields.
func WithFields(fields map[string]interface{}) *Logger {
	return defaultLogger.WithFields(fields)
}
