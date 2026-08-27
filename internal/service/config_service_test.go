package service

import (
	"context"
	"errors"
	"testing"

	"config-center/internal/model"
	"config-center/internal/store"
)

// newConfigTestHarness wires a MemoryStore-backed ConfigService with an attached
// AuditService, optionally constrained to a small audit log capacity.
func newConfigTestHarness(auditCapacity int) (*ConfigService, *AuditService, *store.MemoryStore) {
	ms := store.NewMemoryStore()
	appSvc := NewAppService(ms)
	configSvc := NewConfigService(ms, appSvc)
	auditSvc := NewAuditService(ms)
	configSvc.AttachAuditService(auditSvc)
	if auditCapacity > 0 {
		configSvc.SetAuditStorageCapacity(auditCapacity)
	}
	return configSvc, auditSvc, ms
}

// seedAppWithConfig creates an app and a single config item so UpdateConfig has
// an existing record to mutate.
func seedAppWithConfig(t *testing.T, configSvc *ConfigService, appID, env, key string) {
	t.Helper()
	ctx := context.Background()
	if _, err := configSvc.appSvc.CreateApp(ctx, appID, appID, "desc", "owner"); err != nil {
		t.Fatalf("seed app: %v", err)
	}
	if _, err := configSvc.CreateConfig(ctx, appID, env, key, "v0", "desc", "string", "tester"); err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

// TestUpdateConfigPropagatesAuditFailure verifies the reported bug: when the
// audit log store is full, updating a config must surface an error to the
// caller instead of returning success with a broken audit trail.
func TestUpdateConfigPropagatesAuditFailure(t *testing.T) {
	configSvc, auditSvc, ms := newConfigTestHarness(1)
	ctx := context.Background()
	appID, env, key := "app1", "dev", "k1"

	seedAppWithConfig(t, configSvc, appID, env, key)

	// Fill the single audit slot so the next write is over capacity. Seeding
	// via the service layer does not write audit rows (those are handler-side),
	// so explicitly occupy the slot here.
	if err := auditSvc.LogSuccess(ctx, model.ActionCreate, "config", "seed",
		appID, env, "tester", "127.0.0.1", "seed"); err != nil {
		t.Fatalf("seed audit write should succeed, got: %v", err)
	}
	if got := ms.AuditLogCount(); got != 1 {
		t.Fatalf("audit count after seed = %d, want 1", got)
	}

	// Updating the config value must fail because the audit write cannot persist.
	// Previously this returned success with a silently-lost audit row.
	_, err := configSvc.UpdateConfig(ctx, appID, env, key, "v1", "", "tester")
	if err == nil {
		t.Fatalf("UpdateConfig at audit capacity must return an error, got nil")
	}

	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != model.ErrCodeInternal {
		t.Fatalf("UpdateConfig error should wrap an internal AppError, got %v", err)
	}

	// The over-capacity audit row must not have been stored.
	if got := ms.AuditLogCount(); got != 1 {
		t.Fatalf("audit count after rejected update = %d, want 1", got)
	}
}

// TestUpdateConfigSucceedsWhenCapacityAvailable confirms the happy path is
// unaffected: a normal update still succeeds and records the audit log.
func TestUpdateConfigSucceedsWhenCapacityAvailable(t *testing.T) {
	configSvc, _, ms := newConfigTestHarness(0) // unlimited
	ctx := context.Background()
	appID, env, key := "app2", "dev", "k2"

	seedAppWithConfig(t, configSvc, appID, env, key)
	before := ms.AuditLogCount()

	cfg, err := configSvc.UpdateConfig(ctx, appID, env, key, "newval", "", "tester")
	if err != nil {
		t.Fatalf("UpdateConfig should succeed with capacity available, got: %v", err)
	}
	if cfg.Value != "newval" {
		t.Fatalf("updated value = %q, want %q", cfg.Value, "newval")
	}
	if got := ms.AuditLogCount(); got != before+1 {
		t.Fatalf("audit count after update = %d, want %d (audit row must be recorded)", got, before+1)
	}
}
