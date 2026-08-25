package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/pkg/logger"
)

// PanicGuardFn is a function that determines whether to panic on error.
type PanicGuardFn func(code, rawURL string) bool

// URLStore provides persistent storage for short URLs using a file-based backend.
type URLStore struct {
	mu         sync.RWMutex
	filePath   string
	fileStore  *FileStore
	urls       map[string]*model.ShortURL
	panicGuard PanicGuardFn
	logger     *logger.Logger
}

// NewURLStore creates a new URLStore with the given configuration.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	fs, err := NewFileStore(cfg.Storage.FilePath, true, cfg.Storage.AutoSaveInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to create file store: %w", err)
	}

	us := &URLStore{
		filePath:  cfg.Storage.FilePath,
		fileStore: fs,
		urls:      make(map[string]*model.ShortURL),
		logger:    logger.WithField("store", "url"),
	}

	if err := us.load(); err != nil {
		fs.Close()
		return nil, fmt.Errorf("failed to load URL store: %w", err)
	}

	return us, nil
}

// load reads the URL data from the JSON file.
func (us *URLStore) load() error {
	data, err := os.ReadFile(us.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var exportData struct {
		URLs map[string]*model.ShortURL `json:"urls"`
	}

	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("failed to unmarshal URL data: %w", err)
	}

	if exportData.URLs != nil {
		us.urls = exportData.URLs
	}

	return nil
}

// Save persists the URL data to the JSON file.
func (us *URLStore) save() error {
	us.mu.RLock()
	defer us.mu.RUnlock()

	exportData := struct {
		URLs map[string]*model.ShortURL `json:"urls"`
	}{
		URLs: us.urls,
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal URL data: %w", err)
	}

	tmpFile := us.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, us.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Load loads the URL store data.
func (us *URLStore) Load(_ context.Context) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.load()
}

// Close releases resources held by the URLStore.
func (us *URLStore) Close() error {
	if err := us.save(); err != nil {
		us.logger.Errorf("failed to save before close: %v", err)
	}
	return us.fileStore.Close()
}

// SetPanicGuard sets a guard function that determines whether to panic on certain errors.
func (us *URLStore) SetPanicGuard(fn PanicGuardFn) {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.panicGuard = fn
}

// Save stores a short URL.
func (us *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short URL is nil")
	}

	if !overwrite {
		us.mu.RLock()
		_, exists := us.urls[u.Code]
		us.mu.RUnlock()
		if exists {
			return model.NewAppError(model.ErrCodeAlreadyExists,
				fmt.Sprintf("short URL with code '%s' already exists", u.Code))
		}
	}

	if err := u.Validate(); err != nil {
		return model.ErrValidationFailed(err.Error())
	}

	us.mu.Lock()
	us.urls[u.Code] = u
	us.mu.Unlock()

	if err := us.save(); err != nil {
		return fmt.Errorf("failed to save short URL: %w", err)
	}

	return nil
}

// Get retrieves a short URL by its code.
func (us *URLStore) Get(code string) (*model.ShortURL, error) {
	us.mu.RLock()
	defer us.mu.RUnlock()

	u, exists := us.urls[code]
	if !exists {
		return nil, model.NewAppError(model.ErrCodeNotFound,
			fmt.Sprintf("short URL with code '%s' not found", code))
	}

	return u, nil
}

// RawSnapshot returns a snapshot of all short URLs for diagnostics.
func (us *URLStore) RawSnapshot() map[string]model.ShortURL {
	us.mu.RLock()
	defer us.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(us.urls))
	for code, u := range us.urls {
		snapshot[code] = *u
	}
	return snapshot
}

// AccessLogStore provides persistent storage for access logs.
type AccessLogStore struct {
	mu       sync.RWMutex
	filePath string
	logs     []AccessLogEntry
	logger   *logger.Logger
}

// AccessLogEntry represents a single access log entry.
type AccessLogEntry struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	Timestamp time.Time `json:"timestamp"`
	Status    int       `json:"status"`
	IPAddress string    `json:"ip_address"`
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	als := &AccessLogStore{
		filePath: cfg.Storage.FilePath + ".logs",
		logs:     make([]AccessLogEntry, 0),
		logger:   logger.WithField("store", "access_log"),
	}

	return als, nil
}

// Open opens the access log store and loads existing data.
func (als *AccessLogStore) Open(_ context.Context) error {
	als.mu.Lock()
	defer als.mu.Unlock()

	data, err := os.ReadFile(als.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var logs []AccessLogEntry
	if err := json.Unmarshal(data, &logs); err != nil {
		return fmt.Errorf("failed to unmarshal access logs: %w", err)
	}

	als.logs = logs
	return nil
}

// Close persists and closes the access log store.
func (als *AccessLogStore) Close() error {
	als.mu.RLock()
	defer als.mu.RUnlock()

	data, err := json.MarshalIndent(als.logs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal access logs: %w", err)
	}

	if err := os.WriteFile(als.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write access logs: %w", err)
	}

	return nil
}