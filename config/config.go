// Package config provides configuration management for the URL shortener service.
package config

import (
	"sync"
	"time"
)

// Storage holds storage-related configuration.
type Storage struct {
	mu         sync.RWMutex
	urlFilePath string
	logFilePath string
	syncInterval time.Duration
	flushOnWrite bool
}

// URLFilePath sets the file path for URL data persistence.
func (s *Storage) URLFilePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urlFilePath = path
}

// LogFilePath sets the file path for access log persistence.
func (s *Storage) LogFilePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logFilePath = path
}

// SyncInterval sets the interval for background sync operations.
func (s *Storage) SyncInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncInterval = d
}

// FlushOnWrite sets whether data is flushed to disk on every write.
func (s *Storage) FlushOnWrite(b bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushOnWrite = b
}

// GetURLFilePath returns the configured URL data file path.
func (s *Storage) GetURLFilePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.urlFilePath
}

// GetLogFilePath returns the configured access log file path.
func (s *Storage) GetLogFilePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logFilePath
}

// GetSyncInterval returns the configured sync interval.
func (s *Storage) GetSyncInterval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncInterval
}

// GetFlushOnWrite returns whether flush-on-write is enabled.
func (s *Storage) GetFlushOnWrite() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flushOnWrite
}

// Config is the top-level configuration for the URL shortener service.
type Config struct {
	Storage Storage
}

// Default returns a Config with sensible default values.
func Default() *Config {
	return &Config{
		Storage: Storage{
			urlFilePath:  "./urls.json",
			logFilePath:  "./access.log",
			syncInterval: 10 * time.Second,
			flushOnWrite: true,
		},
	}
}
