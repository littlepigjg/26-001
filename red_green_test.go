package config_center_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	appSvc := service.NewAppService(s)
	cfgSvc := service.NewConfigService(s, appSvc)

	appID := "test-app"
	env := "dev"

	// Create the app first
	_, err := appSvc.CreateApp(ctx, appID, "Test App", "Test Description", "tester")
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	t.Run("bypass_stores_invalid_long_key", func(t *testing.T) {
		longKey := strings.Repeat("x", 300)
		config := model.NewConfigItem(
			fmt.Sprintf("cfg-%s-%s-%s", appID, env, longKey),
			appID, env, longKey,
			"test-value", "", "string", "tester",
		)

		err := cfgSvc.CreateConfigDirect(ctx, config)
		if err == nil {
			t.Log("RED (红灯，缺陷未修复): 绕过服务层校验后，超长 key 被直接入库，validation 未生效")
			// Verify the config was actually stored
			stored, getErr := s.GetConfig(ctx, appID, env, longKey)
			if getErr != nil {
				t.Fatalf("config should have been stored but GetConfig returned error: %v", getErr)
			}
			if len(stored.Key) != 300 {
				t.Fatalf("stored key length = %d, want 300", len(stored.Key))
			}
			t.Fatalf("RED (红灯，缺陷未修复): 绕过服务层校验后，超长 key 被直接入库，validation 未生效")
		} else {
			t.Logf("GREEN (绿灯，缺陷已修复): CreateConfigDirect returned error: %v", err)
		}
	})

	t.Run("normal_path_rejects_invalid_long_key", func(t *testing.T) {
		longKey := strings.Repeat("y", 300)
		_, err := cfgSvc.CreateConfig(ctx, appID, env, longKey, "test-value", "", "string", "tester")
		if err == nil {
			t.Fatal("RED (红灯): 正常路径应该拒绝超长 key，但却成功了")
		}
		t.Logf("GREEN (绿灯): 正常路径正确拒绝超长 key: %v", err)
	})

	t.Run("context_cancellation_propagates", func(t *testing.T) {
		longKey := strings.Repeat("z", 300)
		config := model.NewConfigItem(
			fmt.Sprintf("cfg-%s-%s-%s", appID, env, longKey),
			appID, env, longKey,
			"test-value", "", "string", "tester",
		)

		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := cfgSvc.CreateConfigDirect(cancelCtx, config)
		if err == nil {
			t.Fatal("RED (红灯): context 已取消但 CreateConfigDirect 没有返回错误")
		}
		t.Logf("GREEN (绿灯): context 取消后返回错误: %v", err)
	})

	t.Run("panic_guard_prevents_duplicate_write", func(t *testing.T) {
		s2 := store.NewMemoryStore()
		guardCalled := false
		s2.SetPanicGuard(func(appID, key string) bool {
			guardCalled = true
			return false // block the write
		})

		appSvc2 := service.NewAppService(s2)
		cfgSvc2 := service.NewConfigService(s2, appSvc2)
		_, _ = appSvc2.CreateApp(ctx, "guard-app", "Guard App", "Test", "tester")

		config := model.NewConfigItem(
			"cfg-guard-app-dev-blocked",
			"guard-app", "dev", "blocked-key",
			"value", "", "string", "tester",
		)

		_ = cfgSvc2.CreateConfigDirect(ctx, config)
		if !guardCalled {
			t.Fatal("RED (红灯): PanicGuard 未被调用")
		}
		t.Log("GREEN (绿灯): PanicGuard 正确阻止了写入")
	})

	t.Run("snapshot_includes_stored_invalid_data", func(t *testing.T) {
		snap := s.RawSnapshot()
		count := len(snap)
		t.Logf("snapshot contains %d entries", count)
		if count < 1 {
			t.Fatal("RED (红灯): snapshot 应该包含至少1条记录")
		}
		t.Log("GREEN (绿灯): snapshot 包含已存储的非法数据")
	})
}
