package service

import (
	"context"
	"errors"
	"testing"

	"config-center/internal/model"
	"config-center/internal/store"
)

// newAuditTestHarness wires up a MemoryStore-backed AuditService so tests can
// exercise the audit log capacity path via SetStoreAuditLogCapacity.
func newAuditTestHarness(capacity int) (*AuditService, *store.MemoryStore) {
	ms := store.NewMemoryStore()
	auditSvc := NewAuditService(ms)
	if capacity > 0 {
		auditSvc.SetStoreAuditLogCapacity(capacity)
	}
	return auditSvc, ms
}

// TestLogReturnsErrorWhenCapacityReached reproduces the reported bug: with the
// store at capacity, a second audit write must surface an error instead of a
// silent success. Previously AuditService.Log swallowed the store error and
// returned nil, so the caller believed the audit record was persisted.
func TestLogReturnsErrorWhenCapacityReached(t *testing.T) {
	auditSvc, ms := newAuditTestHarness(1)

	// First write succeeds (capacity 1 -> 1/1 used).
	if err := auditSvc.LogSuccess(
		context.Background(),
		model.ActionCreate, "config", "key1",
		"app1", "dev", "tester", "127.0.0.1",
		"created key1",
	); err != nil {
		t.Fatalf("first audit write should succeed, got: %v", err)
	}
	if got := ms.AuditLogCount(); got != 1 {
		t.Fatalf("audit count after first write = %d, want 1", got)
	}

	// Second write is now over capacity and MUST report the failure.
	err := auditSvc.LogSuccess(
		context.Background(),
		model.ActionUpdate, "config", "key1",
		"app1", "dev", "tester", "127.0.0.1",
		"updated key1",
	)
	if err == nil {
		t.Fatalf("second audit write at capacity must return an error, got nil")
	}

	// The store must not have grown: the over-capacity record was rejected.
	if got := ms.AuditLogCount(); got != 1 {
		t.Fatalf("audit count after rejected write = %d, want 1 (no silent drop)", got)
	}
}

// TestLogConfigChangeReturnsErrorWhenCapacityReached verifies the config-change
// helper propagates the store failure rather than masking it as success.
func TestLogConfigChangeReturnsErrorWhenCapacityReached(t *testing.T) {
	auditSvc, ms := newAuditTestHarness(1)

	// Fill the single slot.
	if err := auditSvc.LogConfigCreate(
		context.Background(), "app1", "dev", "key1", "tester", "127.0.0.1",
	); err != nil {
		t.Fatalf("seed audit write should succeed, got: %v", err)
	}

	// Now at capacity: a config-change audit must surface the failure.
	err := auditSvc.LogConfigChange(
		context.Background(), "app1", "dev", "key1",
		"tester", "127.0.0.1", "old", "new",
	)
	if err == nil {
		t.Fatalf("LogConfigChange at capacity must return an error, got nil")
	}
	if got := ms.AuditLogCount(); got != 1 {
		t.Fatalf("audit count = %d, want 1 (over-capacity record must not be stored)", got)
	}
}

// TestLogInvalidRecordStillValidates guards that the validation path (which
// returns an error before reaching the store) is unaffected by the fix.
func TestLogInvalidRecordStillValidates(t *testing.T) {
	auditSvc, _ := newAuditTestHarness(0)

	// An empty action should fail validation, not be silently accepted.
	err := auditSvc.Log(
		context.Background(),
		"", "config", "key1", "app1", "dev", "tester", "127.0.0.1",
		"summary", "details", "success",
	)
	if err == nil {
		t.Fatalf("invalid audit log should return a validation error, got nil")
	}
}

// TestLogFailurePathErrorsUnwrapped ensures the wrapped error is inspectable
// via errors.As so callers can branch on the underlying AppError code.
func TestLogFailurePathErrorsUnwrapped(t *testing.T) {
	auditSvc, ms := newAuditTestHarness(1)

	_ = auditSvc.LogSuccess(
		context.Background(), model.ActionCreate, "config", "k",
		"app1", "dev", "tester", "127.0.0.1", "seed",
	)

	err := auditSvc.LogFailure(
		context.Background(), model.ActionDelete, "config", "k",
		"app1", "dev", "tester", "127.0.0.1", "delete", "boom",
	)
	if err == nil {
		t.Fatalf("over-capacity LogFailure must error")
	}

	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error should wrap a *model.AppError, got %T: %v", err, err)
	}
	if appErr.Code != model.ErrCodeInternal {
		t.Fatalf("wrapped error code = %d, want %d", appErr.Code, model.ErrCodeInternal)
	}
	if got := ms.AuditLogCount(); got != 1 {
		t.Fatalf("audit count = %d, want 1", got)
	}
}
