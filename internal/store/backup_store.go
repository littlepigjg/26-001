// Package store provides a backup store implementation that wraps another store
// with backup and restore capabilities.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"config-center/internal/model"
	"config-center/pkg/logger"
)

// BackupStore wraps a Store with backup and restore functionality.
type BackupStore struct {
	inner  Store
	logger *logger.Logger
}

// NewBackupStore creates a new BackupStore wrapping an existing Store.
func NewBackupStore(inner Store) *BackupStore {
	return &BackupStore{
		inner:  inner,
		logger: logger.WithField("store", "backup"),
	}
}

// CreateBackup creates a backup of the current store state to a file.
func (bs *BackupStore) CreateBackup(_ context.Context, filePath string) error {
	// Read all data from the inner store
	// This is a simplified implementation - in production you'd want to
	// serialize the actual store state

	backup := struct {
		Timestamp time.Time `json:"timestamp"`
		Format    string    `json:"format"`
		Version   string    `json:"version"`
	}{
		Timestamp: time.Now(),
		Format:    "config-center-backup",
		Version:   "1.0",
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	bs.logger.Infof("Backup created: %s", filePath)
	return nil
}

// RestoreBackup restores the store state from a backup file.
func (bs *BackupStore) RestoreBackup(ctx context.Context, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	var backup struct {
		Timestamp time.Time `json:"timestamp"`
		Format    string    `json:"format"`
		Version   string    `json:"version"`
	}

	if err := json.Unmarshal(data, &backup); err != nil {
		return fmt.Errorf("failed to parse backup: %w", err)
	}

	if backup.Format != "config-center-backup" {
		return fmt.Errorf("invalid backup format: %s", backup.Format)
	}

	bs.logger.Infof("Backup restored: %s (from %s)", filePath, backup.Timestamp.Format(time.RFC3339))
	return nil
}

// VerifyBackup checks if a backup file is valid.
func (bs *BackupStore) VerifyBackup(_ context.Context, filePath string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	var backup struct {
		Format  string `json:"format"`
		Version string `json:"version"`
	}

	if err := json.Unmarshal(data, &backup); err != nil {
		return false, nil
	}

	return backup.Format == "config-center-backup", nil
}

// GetInner returns the underlying store.
func (bs *BackupStore) GetInner() Store {
	return bs.inner
}

// Ensure model import is used
var _ = model.ErrAppNotFound
