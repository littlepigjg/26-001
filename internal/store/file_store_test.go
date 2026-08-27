package store

import (
	"path/filepath"
	"testing"
	"time"
)

// newTestFileStore creates a FileStore backed by a temp file with a short
// auto-save interval, suitable for lifecycle tests.
func newTestFileStore(t *testing.T, autoSave bool) *FileStore {
	t.Helper()
	dir := t.TempDir()
	fs, err := NewFileStore(filepath.Join(dir, "store.json"), autoSave, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	return fs
}

// TestFileStoreDoubleClose reproduces the "close of closed channel" panic that
// occurs when Close() is called twice on a FileStore. The second close must be
// safe, not fatal.
func TestFileStoreDoubleClose(t *testing.T) {
	fs := newTestFileStore(t, true)
	defer fs.Close()

	if err := fs.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}

	// Second close must not panic ("close of closed channel").
	if err := fs.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}

	// Third close should also be a safe no-op.
	if err := fs.Close(); err != nil {
		t.Fatalf("third Close failed: %v", err)
	}
}

// TestFileStoreCloseWithoutAutoSave verifies closing a FileStore that never
// started the auto-save loop is also safe to call repeatedly.
func TestFileStoreCloseWithoutAutoSave(t *testing.T) {
	fs := newTestFileStore(t, false)
	if err := fs.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

// TestFileStoreAutoSaveLoopDoesNotDoubleClose ensures the auto-save goroutine's
// internal close of the quit channel does not race with an external Close(),
// which previously surfaced as a panic from the loop's own recover().
func TestFileStoreAutoSaveLoopDoesNotDoubleClose(t *testing.T) {
	for i := 0; i < 20; i++ {
		fs := newTestFileStore(t, true)
		// Give the auto-save loop time to be actively selecting on the quit channel.
		time.Sleep(80 * time.Millisecond)
		if err := fs.Close(); err != nil {
			t.Fatalf("Close failed on iteration %d: %v", i, err)
		}
	}
}
