package main

import (
	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestRedGreen verifies that sentinel errors returned from the short URL
// service can be detected via errors.Is / errors.As across the full store ->
// service layer stack. This is a pure pass/fail check with an explicit
// RED / GREEN printout; it does not depend on any external environment.
func TestRedGreen(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shurl-test-*")
	if err != nil {
		t.Fatalf("mkdirtmp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.Default()
	cfg.Storage.URLFilePath(filepath.Join(tmpDir, "urls.json"))
	cfg.Storage.LogFilePath(filepath.Join(tmpDir, "access.log"))

	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	if err := us.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer us.Close()

	ls, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore: %v", err)
	}
	if err := ls.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer ls.Close()

	svc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("NewURLService: %v", err)
	}

	const missingCode = "does-not-exist"

	// 1) Direct store.GetVersion (version 0) - current snapshot miss.
	if _, err = us.GetVersion(missingCode, 0); err == nil {
		t.Fatalf("expected error for store.GetVersion(missing, 0), got nil")
	}
	if !errors.Is(err, model.ErrVersionNotFound) {
		emitRed(t, fmt.Sprintf("store.GetVersion(version=0): %s", err.Error()))
		return
	}

	// 2) Direct store.GetVersion (version 1) - historical snapshot miss.
	if _, err = us.GetVersion(missingCode, 1); err == nil {
		t.Fatalf("expected error for store.GetVersion(missing, 1), got nil")
	}
	if !errors.Is(err, model.ErrVersionNotFound) {
		emitRed(t, fmt.Sprintf("store.GetVersion(version=1): %s", err.Error()))
		return
	}

	// 3) Direct store.LookupHistoricalSnapshot - historical miss path.
	if _, err = us.LookupHistoricalSnapshot(missingCode, 1); err == nil {
		t.Fatalf("expected error for store.LookupHistoricalSnapshot(missing, 1), got nil")
	}
	if !errors.Is(err, model.ErrVersionNotFound) {
		emitRed(t, fmt.Sprintf("store.LookupHistoricalSnapshot: %s", err.Error()))
		return
	}

	// 4) Service layer -> store.GetVersion: cross-file error propagation.
	if _, err = svc.GetVersion(missingCode, 0); err == nil {
		t.Fatalf("expected error for svc.GetVersion(missing, 0), got nil")
	}
	if !errors.Is(err, model.ErrVersionNotFound) {
		emitRed(t, fmt.Sprintf("service.GetVersion: %s", err.Error()))
		return
	}

	// All four assertions green: the sentinel is preserved end-to-end.
	t.Logf("GREEN (绿灯，缺陷已修复)")
}

// emitRed prints the RED verdict and fails the test without a confusing
// stack trace.
func emitRed(t *testing.T, detail string) {
	t.Helper()
	t.Logf("RED (红灯，缺陷未修复)")
	t.Logf("  %s", detail)
	t.Logf("  errors.Is(err, model.ErrVersionNotFound) returned false")
	t.Logf("  This indicates the store or service layer is wrapping the")
	t.Logf("  original error with fmt.Errorf(\"%%v\", ...) instead of %%w,")
	t.Logf("  which breaks the errors.Is / errors.As detection chain.")
	t.FailNow()
}
