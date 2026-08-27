package config_center

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
	ctx := context.Background()

	memStore := store.NewMemoryStore()
	appSvc := service.NewAppService(memStore)

	app := model.NewApplication("test-app", "Test Application", "For goroutine leak testing", "test-owner")
	if err := memStore.CreateApp(ctx, app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	cs := service.NewClientService(memStore, appSvc, true)

	for i := 0; i < 5; i++ {
		cs.RestartCacheCleanup(50 * time.Millisecond)
		time.Sleep(20 * time.Millisecond)
	}

	cs.Close()

	time.Sleep(500 * time.Millisecond)

	activeGoroutines := cs.GetActiveGoroutines()

	if activeGoroutines > 0 {
		fmt.Printf("RED (红灯，缺陷未修复)\n")
		fmt.Printf("检测到 %d 个泄漏的 goroutine，Close 后缓存清理 goroutine 未正确停止\n", activeGoroutines)
		t.FailNow()
	} else {
		fmt.Printf("GREEN (绿灯，缺陷已修复)\n")
		fmt.Printf("所有 goroutine 已正确清理，无泄漏\n")
	}
}
