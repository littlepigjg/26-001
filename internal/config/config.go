// Package config provides application configuration loading and management.
// It supports loading configuration from environment variables and config files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"config-center/pkg/logger"
)

// ServerConfig holds the server configuration.
type ServerConfig struct {
	// Host is the server hostname or IP to bind to.
	Host string `json:"host"`
	// Port is the port number to listen on.
	Port int `json:"port"`
	// ReadTimeout is the maximum duration for reading the request.
	ReadTimeout time.Duration `json:"read_timeout"`
	// WriteTimeout is the maximum duration for writing the response.
	WriteTimeout time.Duration `json:"write_timeout"`
	// IdleTimeout is the keep-alive timeout duration.
	IdleTimeout time.Duration `json:"idle_timeout"`
	// ShutdownTimeout is the graceful shutdown timeout.
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// StorageConfig holds storage-related configuration.
type StorageConfig struct {
	// Type is the storage backend type ("memory" or "file").
	Type string `json:"type"`
	// FilePath is the path to the storage file (for file storage).
	FilePath string `json:"file_path"`
	// AutoSave enables automatic persistence for file storage.
	AutoSave bool `json:"auto_save"`
	// AutoSaveInterval is how often to save (for file storage).
	AutoSaveInterval time.Duration `json:"auto_save_interval"`
	// LogPath is the path to the access log file.
	LogPath string `json:"log_path"`
	// SyncIntervalDuration is the interval for syncing data.
	SyncIntervalDuration time.Duration `json:"sync_interval_duration"`
	// FlushOnWriteEnabled controls whether to flush after each write.
	FlushOnWriteEnabled bool `json:"flush_on_write_enabled"`
}

// URLFilePath sets the file path for URL store persistence.
func (s *StorageConfig) URLFilePath(path string) {
	s.FilePath = path
}

// LogFilePath sets the file path for access log storage.
func (s *StorageConfig) LogFilePath(path string) {
	s.LogPath = path
}

// SyncInterval sets the synchronization interval.
func (s *StorageConfig) SyncInterval(d time.Duration) {
	s.SyncIntervalDuration = d
}

// FlushOnWrite enables or disables flush on write.
func (s *StorageConfig) FlushOnWrite(b bool) {
	s.FlushOnWriteEnabled = b
}

// LoggingConfig holds logging-related configuration.
type LoggingConfig struct {
	// Level is the minimum log level.
	Level string `json:"level"`
	// Output is the log output destination ("stdout", "stderr", or file path).
	Output string `json:"output"`
	// Format is the log format ("text" or "json").
	Format string `json:"format"`
	// ShowCaller includes caller information in logs.
	ShowCaller bool `json:"show_caller"`
}

// CacheConfig holds cache-related configuration.
type CacheConfig struct {
	// Enabled enables the cache layer.
	Enabled bool `json:"enabled"`
	// DefaultTTL is the default time-to-live for cache entries.
	DefaultTTL time.Duration `json:"default_ttl"`
	// MaxSize is the maximum number of cache entries.
	MaxSize int `json:"max_size"`
}

// Config is the complete application configuration.
type Config struct {
	// Server holds server-specific settings.
	Server ServerConfig `json:"server"`
	// Storage holds storage backend settings.
	Storage StorageConfig `json:"storage"`
	// Logging holds logging settings.
	Logging LoggingConfig `json:"logging"`
	// Cache holds cache settings.
	Cache CacheConfig `json:"cache"`
}

// defaultConfig returns the default configuration.
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Storage: StorageConfig{
			Type:             "memory",
			FilePath:         "",
			AutoSave:         false,
			AutoSaveInterval: 30 * time.Second,
		},
		Logging: LoggingConfig{
			Level:      "INFO",
			Output:     "stdout",
			Format:     "text",
			ShowCaller: true,
		},
		Cache: CacheConfig{
			Enabled:    true,
			DefaultTTL: 5 * time.Minute,
			MaxSize:    10000,
		},
	}
}

func Default() *Config {
	return defaultConfig()
}

// Load loads the configuration from the specified sources.
// It first loads defaults, then applies file overrides, then environment variable overrides.
func Load(configFile string) (*Config, error) {
	cfg := defaultConfig()

	// Load from config file if specified
	if configFile != "" {
		if err := loadFromFile(cfg, configFile); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with environment variables
	loadFromEnv(cfg)

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadFromFile loads configuration overrides from a JSON file.
func loadFromFile(cfg *Config, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// loadFromEnv overrides configuration with environment variables.
func loadFromEnv(cfg *Config) {
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = strings.ToUpper(v)
	}
	if v := os.Getenv("STORAGE_TYPE"); v != "" {
		cfg.Storage.Type = v
	}
	if v := os.Getenv("STORAGE_FILE_PATH"); v != "" {
		cfg.Storage.FilePath = v
	}
	if v := os.Getenv("CACHE_ENABLED"); v != "" {
		cfg.Cache.Enabled = strings.ToLower(v) == "true" || v == "1"
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}
	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("read_timeout must be positive")
	}
	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("write_timeout must be positive")
	}
	if c.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown_timeout must be positive")
	}
	validLevels := map[string]bool{
		"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true, "FATAL": true,
	}
	if !validLevels[strings.ToUpper(c.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}
	return nil
}

// GetLoggerLevel returns the parsed log level.
func (c *Config) GetLoggerLevel() logger.Level {
	return logger.ParseLevel(c.Logging.Level)
}

// String returns a human-readable representation of the configuration (without sensitive data).
func (c *Config) String() string {
	return fmt.Sprintf("Server: %s:%d, Storage: %s, LogLevel: %s, Cache: %v",
		c.Server.Host, c.Server.Port, c.Storage.Type, c.Logging.Level, c.Cache.Enabled)
}

// Provider is a thread-safe configuration provider that allows runtime configuration changes.
type Provider struct {
	mu     sync.RWMutex
	config *Config
}

// NewProvider creates a new Provider with the given initial configuration.
func NewProvider(cfg *Config) *Provider {
	return &Provider{config: cfg}
}

// Get returns the current configuration.
func (p *Provider) Get() *Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// Return a copy to prevent external modification
	cfg := *p.config
	return &cfg
}

// Update replaces the current configuration.
func (p *Provider) Update(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = cfg
	return nil
}
