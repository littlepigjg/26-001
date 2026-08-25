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

	s := store.NewMemoryStore()
	appSvc := service.NewAppService(s)
	configSvc := service.NewConfigService(s, appSvc)
	versionSvc := service.NewVersionService(s, appSvc, configSvc)

	t.Run("Setup", func(t *testing.T) {
		_, err := appSvc.CreateApp(ctx, "my-app", "My Application", "Test app", "admin")
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		err = appSvc.AddEnvironment(ctx, "my-app", "dev")
		if err != nil {
			t.Fatalf("failed to add environment: %v", err)
		}
		_, err = configSvc.CreateConfig(ctx, "my-app", "dev", "db.host", "localhost", "Database host", "string", "admin")
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}
		_, err = configSvc.CreateConfig(ctx, "my-app", "dev", "db.port", "5432", "Database port", "number", "admin")
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}
	})

	t.Run("CreateFirstVersion", func(t *testing.T) {
		ver, created, err := versionSvc.AutoSnapshot(ctx, "my-app", "dev", "admin")
		if err != nil {
			t.Fatalf("first AutoSnapshot failed: %v", err)
		}
		if !created {
			t.Fatalf("expected first version to be created, but created=%v", created)
		}
		if ver != 1 {
			t.Fatalf("expected version 1, got %d", ver)
		}
	})

	t.Run("DeleteVersionCreatesInconsistency", func(t *testing.T) {
		err := versionSvc.DeleteOldVersions(ctx, "my-app", "dev", 2)
		if err != nil {
			t.Fatalf("failed to delete old versions: %v", err)
		}

		snapshot := s.RawSnapshot()
		t.Logf("After delete: version_count=%v, index_count=%v, versions_index=%v",
			snapshot["version_count"], snapshot["index_count"], snapshot["versions_index"])
	})

	t.Run("AutoSnapshotShouldReturnError", func(t *testing.T) {
		_, _, err := versionSvc.AutoSnapshot(ctx, "my-app", "dev", "admin")
		if err == nil {
			t.Errorf("expected error due to inconsistent version state, but got nil error")
			fmt.Println("RED（红灯，缺陷未修复）")
			t.Fail()
		} else {
			fmt.Println("GREEN（绿灯，缺陷已修复）")
		}
	})
}

func TestErrorPropagationWithSyncSnapshot(t *testing.T) {
	ctx := context.Background()

	s := store.NewMemoryStore()
	appSvc := service.NewAppService(s)
	configSvc := service.NewConfigService(s, appSvc)
	versionSvc := service.NewVersionService(s, appSvc, configSvc)

	_, err := appSvc.CreateApp(ctx, "test-sync", "Test Sync", "", "dev")
	if err != nil {
		t.Fatal(err)
	}
	appSvc.AddEnvironment(ctx, "test-sync", "staging")

	_, err = configSvc.CreateConfig(ctx, "test-sync", "staging", "timeout", "30s", "", "string", "tester")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = versionSvc.AutoSnapshot(ctx, "test-sync", "staging", "tester")
	if err != nil {
		t.Fatal(err)
	}

	err = versionSvc.DeleteOldVersions(ctx, "test-sync", "staging", 2)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = versionSvc.SyncSnapshotIfChanged(ctx, "test-sync", "staging", "tester")
	if err == nil {
		t.Errorf("SyncSnapshotIfChanged should detect inconsistency and return error")
		fmt.Println("RED（红灯，缺陷未修复）")
		t.Fail()
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
	}
}

func TestErrorPropagationPanicGuard(t *testing.T) {
	ctx := context.Background()

	s := store.NewMemoryStore()
	appSvc := service.NewAppService(s)
	configSvc := service.NewConfigService(s, appSvc)
	versionSvc := service.NewVersionService(s, appSvc, configSvc)

	_, err := appSvc.CreateApp(ctx, "guard-app", "Guard App", "", "ops")
	if err != nil {
		t.Fatal(err)
	}
	appSvc.AddEnvironment(ctx, "guard-app", "prod")

	_, err = configSvc.CreateConfig(ctx, "guard-app", "prod", "feature.flag", "true", "", "boolean", "ops")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = versionSvc.AutoSnapshot(ctx, "guard-app", "prod", "ops")
	if err != nil {
		t.Fatal(err)
	}

	s.SetPanicGuard(func(appID, env string) bool {
		return appID == "guard-app" && env == "prod"
	})

	_, _, err = versionSvc.AutoSnapshot(ctx, "guard-app", "prod", "ops")
	if err == nil {
		t.Errorf("expected error from GetVersion due to PanicGuard, but error was swallowed")
		fmt.Println("RED（红灯，缺陷未修复）")
		t.Fail()
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
	}
}

func TestErrorPropagationFull(t *testing.T) {
	ctx := context.Background()

	s := store.NewMemoryStore()
	appSvc := service.NewAppService(s)
	configSvc := service.NewConfigService(s, appSvc)
	versionSvc := service.NewVersionService(s, appSvc, configSvc)

	_, err := appSvc.CreateApp(ctx, "full-app", "Full App", "Integration test", "sre")
	if err != nil {
		t.Fatal(err)
	}
	appSvc.AddEnvironment(ctx, "full-app", "canary")

	configs := []struct{ key, value string }{
		{"server.port", "8080"},
		{"server.host", "0.0.0.0"},
		{"db.max_connections", "100"},
	}
	for _, c := range configs {
		_, err = configSvc.CreateConfig(ctx, "full-app", "canary", c.key, c.value, "", "string", "sre")
		if err != nil {
			t.Fatal(err)
		}
	}

	_, created, err := versionSvc.AutoSnapshot(ctx, "full-app", "canary", "sre")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("first snapshot should create version")
	}

	_, err = configSvc.CreateConfig(ctx, "full-app", "canary", "new.key", "new_value", "", "string", "sre")
	if err != nil {
		t.Fatal(err)
	}

	err = versionSvc.DeleteOldVersions(ctx, "full-app", "canary", 3)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = versionSvc.AutoSnapshot(ctx, "full-app", "canary", "sre")
	if err == nil {
		t.Errorf("AutoSnapshot should fail when version index is inconsistent with actual versions")
		fmt.Println("RED（红灯，缺陷未修复）")
		t.Fail()
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	fmt.Printf("exit_code=%d\n", code)
}
