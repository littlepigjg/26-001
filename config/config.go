// Package config provides configuration management for the URL shortener service.
package config

import (
	"time"
)

// Config holds the complete application configuration.
type Config struct {
	// Storage holds storage backend settings.
	Storage Storage
}

// Storage holds storage-related configuration.
type Storage struct {
	urlFilePath    string
	logFilePath    string
	syncInterval   time.Duration
	flushOnWrite   bool
}

// URLFilePath sets the file path for URL data persistence.
func (s *Storage) URLFilePath(path string) {
	s.urlFilePath = path
}

// LogFilePath sets the file path for access log persistence.
func (s *Storage) LogFilePath(path string) {
	s.logFilePath = path
}

// SyncInterval sets the interval for syncing data to disk.
func (s *Storage) SyncInterval(d time.Duration) {
	s.syncInterval = d
}

// FlushOnWrite sets whether to flush data to disk on every write.
func (s *Storage) FlushOnWrite(b bool) {
	s.flushOnWrite = b
}

// GetURLFilePath returns the configured URL file path.
func (s *Storage) GetURLFilePath() string {
	return s.urlFilePath
}

// GetLogFilePath returns the configured log file path.
func (s *Storage) GetLogFilePath() string {
	return s.logFilePath
}

// GetSyncInterval returns the configured sync interval.
func (s *Storage) GetSyncInterval() time.Duration {
	return s.syncInterval
}

// GetFlushOnWrite returns whether flush-on-write is enabled.
func (s *Storage) GetFlushOnWrite() bool {
	return s.flushOnWrite
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Storage: Storage{
			urlFilePath:  "./data/urls.db",
			logFilePath:  "./data/access.log",
			syncInterval: 30 * time.Second,
			flushOnWrite: false,
		},
	}
}
