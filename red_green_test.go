package config_center_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	app := model.NewApplication("test-app", "Test Application", "Integration test app", "qa-team")
	if err := s.CreateApp(ctx, app); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	for i := 0; i < 8; i++ {
		cfg := model.NewConfigItem(
			fmt.Sprintf("cfg-%s-dev-key%d", app.ID, i),
			app.ID, "dev",
			fmt.Sprintf("key-%d", i),
			fmt.Sprintf("value-%d", i),
			fmt.Sprintf("Test config %d", i),
			"string",
			"test-runner",
		)
		if err := s.CreateConfig(ctx, cfg); err != nil {
			t.Fatalf("Failed to create config %d: %v", i, err)
		}
	}

	ver := model.NewVersion(
		"ver-1", app.ID, "dev",
		"test-runner", "Initial version",
		map[string]string{"key-0": "value-0", "key-1": "value-1"},
		1, 0, "hash-001",
	)
	if err := s.CreateVersion(ctx, ver); err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	hm := service.NewHealthMonitor(s, 10*time.Second)

	t.Run("normal_health_check", func(t *testing.T) {
		report := hm.CheckNow(context.Background())
		if report == nil {
			t.Fatal("Health report should not be nil")
		}
		if report.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s'", report.Status)
		}
		if len(report.Components) == 0 {
			t.Error("Expected at least one component in health report")
		}
		fmt.Printf("  Normal check: status=%s, components=%d, duration=%dms\n",
			report.Status, len(report.Components), report.Duration)
	})

	t.Run("cancelled_context_should_not_block", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		start := time.Now()
		report := hm.CheckNow(ctx)
		elapsed := time.Since(start)

		if report == nil {
			t.Fatal("Health report should not be nil even with cancelled context")
		}

		fmt.Printf("  Cancelled ctx check: status=%s, elapsed=%v, reportDuration=%dms\n",
			report.Status, elapsed, report.Duration)

		if elapsed > 150*time.Millisecond {
			t.Log("RED (红灯，缺陷未修复): 健康检查在 context 取消后仍继续执行，未能及时响应取消信号")
			t.FailNow()
		}

		t.Log("GREEN (绿灯，缺陷已修复): 健康检查正确响应了 context 取消信号，未阻塞关闭流程")
	})
}
