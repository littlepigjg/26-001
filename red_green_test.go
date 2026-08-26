package config_center

import (
	"context"
	"fmt"
	"testing"

	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	ctx := context.Background()

	t.Run("audit_service_log_config_change", func(t *testing.T) {
		st := store.NewMemoryStore()
		st.SetAuditLogCapacity(1)

		auditSvc := service.NewAuditService(st)

		err := auditSvc.LogConfigChange(ctx, "app1", "dev", "key1", "user1", "127.0.0.1", "old1", "new1")
		if err != nil {
			t.Fatalf("unexpected error filling audit log store: %v", err)
		}

		if st.AuditLogCount() != 1 {
			t.Fatalf("expected 1 audit log after filling, got %d", st.AuditLogCount())
		}

		err = auditSvc.LogConfigChange(ctx, "app1", "dev", "key2", "user1", "127.0.0.1", "old2", "new2")

		if err == nil {
			fmt.Println("RED (红灯，缺陷未修复)")
			t.Errorf("expected error when audit log storage is full, but got nil — audit logs are being silently lost without any error returned to the caller")
		} else {
			fmt.Println("GREEN (绿灯，缺陷已修复)")
		}
	})

	t.Run("config_service_update_config", func(t *testing.T) {
		st := store.NewMemoryStore()
		st.SetAuditLogCapacity(1)

		appSvc := service.NewAppService(st)
		auditSvc := service.NewAuditService(st)
		configSvc := service.NewConfigService(st, appSvc)
		configSvc.AttachAuditService(auditSvc)

		_, err := appSvc.CreateApp(ctx, "test-app", "Test App", "Test Description", "test-owner")
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}

		_, err = configSvc.CreateConfig(ctx, "test-app", "dev", "db.host", "localhost", "database host", "string", "admin")
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		err = auditSvc.LogConfigChange(ctx, "test-app", "dev", "init.key", "admin", "127.0.0.1", "", "init")
		if err != nil {
			t.Fatalf("unexpected error filling audit log store: %v", err)
		}

		if st.AuditLogCount() != 1 {
			t.Fatalf("expected 1 audit log after filling, got %d", st.AuditLogCount())
		}

		updatedCfg, err := configSvc.UpdateConfig(ctx, "test-app", "dev", "db.host", "production-db", "updated", "admin")
		if err != nil {
			t.Fatalf("unexpected error updating config: %v", err)
		}
		if updatedCfg.Value != "production-db" {
			t.Fatalf("expected value to be 'production-db', got '%s'", updatedCfg.Value)
		}

		countAfter := st.AuditLogCount()
		if countAfter == 1 {
			fmt.Println("RED (红灯，缺陷未修复)")
			t.Errorf("expected audit log count to be 2 (audit log should have been recorded), but got 1 — audit log was silently lost due to ignored error")
		} else {
			fmt.Println("GREEN (绿灯，缺陷已修复)")
		}
	})
}
