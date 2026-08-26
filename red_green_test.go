package main

import (
	"context"
	"fmt"
	"testing"

	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
	"config-center/pkg/hash"
)

// TestRedGreen 测试版本创建在 context 取消时是否会导致状态不一致
// RED = 存在缺陷（版本在 context 取消后仍被创建，导致状态污染）
// GREEN = 无缺陷（版本在 context 取消后不会被创建，状态一致）
func TestRedGreen(t *testing.T) {
	ms := store.NewMemoryStore()
	appSvc := service.NewAppService(ms)
	configSvc := service.NewConfigService(ms, appSvc)
	versionSvc := service.NewVersionService(ms, appSvc, configSvc)

	ctx := context.Background()

	_, err := appSvc.CreateApp(ctx, "test-app", "Test Application", "Test App Description", "admin")
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	err = appSvc.AddEnvironment(ctx, "test-app", "dev")
	if err != nil {
		t.Fatalf("Failed to add environment: %v", err)
	}

	_, err = configSvc.CreateConfig(ctx, "test-app", "dev", "key1", "value1", "test key", "string", "admin")
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	_, err = configSvc.CreateConfig(ctx, "test-app", "dev", "key2", "value2", "test key 2", "string", "admin")
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	t.Run("NormalVersionCreation", func(t *testing.T) {
		beforeVersion, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")

		_, err := versionSvc.CreateVersion(ctx, "test-app", "dev", "admin", "normal version")
		if err != nil {
			t.Fatalf("Failed to create version normally: %v", err)
		}

		afterVersion, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")
		if afterVersion != beforeVersion+1 {
			t.Errorf("Expected version %d, got %d", beforeVersion+1, afterVersion)
		}
	})

	t.Run("ContextCancellationCreateVersion", func(t *testing.T) {
		beforeVersion, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")

		cancelCtx, cancel := context.WithCancel(ctx)

		currentConfig, _ := ms.GetConfigMap(ctx, "test-app", "dev")
		latestVer, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")

		cancel()

		version := model.NewVersion(
			fmt.Sprintf("ver-test-app-dev-%d", latestVer+1),
			"test-app", "dev", "admin", "test with cancelled context",
			currentConfig, latestVer+1, latestVer, hash.MapHash(currentConfig),
		)

		_ = ms.CreateVersion(cancelCtx, version)

		afterVersion, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")

		if afterVersion > beforeVersion {
			t.Logf("RED: Version %d created despite cancelled context", afterVersion)
			t.Errorf("RED (红灯，缺陷未修复): Version was created after context cancellation. "+
				"Context cancellation is not properly checked. "+
				"Before: v%d, After: v%d", beforeVersion, afterVersion)
		} else {
			t.Logf("GREEN: Version not created after context cancellation")
		}
	})

	t.Run("AutoSnapshotWithCancelledContext", func(t *testing.T) {
		beforeVersion, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")

		_, err := configSvc.UpdateConfig(ctx, "test-app", "dev", "key1", "updated_value1", "", "admin")
		if err != nil {
			t.Fatalf("Failed to update config: %v", err)
		}

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		_, _, err = versionSvc.AutoSnapshot(cancelCtx, "test-app", "dev", "admin")

		afterVersion, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")

		if afterVersion > beforeVersion {
			t.Logf("RED: AutoSnapshot created version %d despite cancelled context", afterVersion)
			t.Errorf("RED (红灯，缺陷未修复): AutoSnapshot created a new version despite context cancellation. "+
				"Before: v%d, After: v%d", beforeVersion, afterVersion)
		} else {
			t.Logf("GREEN: AutoSnapshot correctly handled context cancellation")
		}
	})

	t.Run("StatePollutionDetection", func(t *testing.T) {
		version1, err := versionSvc.CreateVersion(ctx, "test-app", "dev", "admin", "v1")
		if err != nil {
			t.Fatalf("Failed to create version: %v", err)
		}

		versionConfig, _ := ms.GetConfigMap(ctx, "test-app", "dev")
		configHash := hash.MapHash(versionConfig)

		if version1.ConfigHash != configHash {
			t.Errorf("Version config hash mismatch: expected %s, got %s", configHash, version1.ConfigHash)
		}

		_, err = configSvc.UpdateConfig(ctx, "test-app", "dev", "key1", "polluted_value", "", "admin")
		if err != nil {
			t.Fatalf("Failed to update config: %v", err)
		}

		currentConfig, _ := ms.GetConfigMap(ctx, "test-app", "dev")
		currentHash := hash.MapHash(currentConfig)

		if version1.ConfigHash == currentHash {
			t.Logf("GREEN: Version data matches current config (no state pollution)")
		} else {
			t.Logf("State normal: Version hash differs from current config (config was updated)")
		}

		version2, err := versionSvc.CreateVersion(ctx, "test-app", "dev", "admin", "v2")
		if err != nil {
			t.Fatalf("Failed to create version 2: %v", err)
		}

		if version2.ConfigHash != currentHash {
			t.Errorf("Version 2 config hash mismatch: expected %s, got %s", currentHash, version2.ConfigHash)
		}
	})

	t.Run("FinalVerdict", func(t *testing.T) {
		beforeVersion, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")

		cancelCtx, cancel := context.WithCancel(ctx)
		currentConfig, _ := ms.GetConfigMap(ctx, "test-app", "dev")
		latestVer, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")
		cancel()

		version := model.NewVersion(
			fmt.Sprintf("ver-test-app-dev-%d", latestVer+100),
			"test-app", "dev", "admin", "final test",
			currentConfig, latestVer+100, latestVer, hash.MapHash(currentConfig),
		)

		_ = ms.CreateVersion(cancelCtx, version)
		afterVersion, _ := ms.GetLatestVersionNumber(ctx, "test-app", "dev")

		contextCancellationBug := (afterVersion > beforeVersion)

		if contextCancellationBug {
			fmt.Println("判定结果: RED（红灯，缺陷未修复）")
			fmt.Println("  - Context cancellation not respected in CreateVersion")
			t.Errorf("RED（红灯，缺陷未修复）: CreateVersion does not respect context cancellation")
		} else {
			fmt.Println("判定结果: GREEN（绿灯，缺陷已修复）")
			t.Log("GREEN（绿灯，缺陷已修复）")
		}
	})
}
