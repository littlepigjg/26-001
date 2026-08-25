package config_center_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	fmt.Println("=== 缺陷检测测试开始 ===")
	fmt.Println()

	cfg := config.Default()
	if cfg == nil {
		t.Fatal("FAIL: config.Default() 返回 nil")
	}

	cfg.Storage.URLFilePath("/tmp/test_urls.dat")
	cfg.Storage.LogFilePath("/tmp/test_logs.dat")
	cfg.Storage.SyncInterval(30 * time.Second)
	cfg.Storage.FlushOnWrite(true)

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("FAIL: NewURLStore 创建失败: %v", err)
	}
	defer urlStore.Close()

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("FAIL: NewAccessLogStore 创建失败: %v", err)
	}
	defer logStore.Close()

	loadCtx, loadCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer loadCancel()
	if err := urlStore.Load(loadCtx); err != nil {
		t.Fatalf("FAIL: URLStore.Load 失败: %v", err)
	}

	openCtx, openCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer openCancel()
	if err := logStore.Open(openCtx); err != nil {
		t.Fatalf("FAIL: AccessLogStore.Open 失败: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("FAIL: NewURLService 创建失败: %v", err)
	}
	urlSvc.SetLogStore(logStore)

	fmt.Println("步骤1: 使用正常 context 创建短链（基线操作）")
	createCtx, createCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer createCancel()

	req := &model.CreateReq{
		RawURL:     "https://example.com/very/long/path",
		CustomCode: "test1",
		MaxVisits:  0,
	}

	startCreate := time.Now()
	su, err := urlSvc.Create(createCtx, req)
	createDuration := time.Since(startCreate)
	if err != nil {
		t.Fatalf("FAIL: 基线创建失败: %v", err)
	}
	fmt.Printf("  基线创建完成: code=%s, 耗时=%v\n", su.Code, createDuration)

	fmt.Println()
	fmt.Println("步骤2: 使用500毫秒超时 context 创建短链（测试 context 超时传播）")
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer timeoutCancel()

	req2 := &model.CreateReq{
		RawURL:     "https://example.com/another/path",
		CustomCode: "test2",
		MaxVisits:  0,
	}

	start := time.Now()
	su2, err := urlSvc.Create(timeoutCtx, req2)
	elapsed := time.Since(start)

	fmt.Printf("  操作耗时: %v\n", elapsed)

	if err != nil {
		fmt.Printf("  返回错误: %v\n", err)
	} else {
		fmt.Printf("  返回成功: code=%s\n", su2.Code)
	}

	fmt.Println()

	if err != nil && elapsed < 1*time.Second {
		fmt.Println("结论: GREEN（绿灯，缺陷已修复）")
		fmt.Println("  Context 超时正确传播，在超时后操作被取消")
		t.Log("GREEN（绿灯，缺陷已修复）: Context 超时正确传播")
	} else {
		fmt.Println("结论: RED（红灯，缺陷未修复）")
		fmt.Printf("  Context 超时未传播（elapsed=%v, err=%v），操作忽略超时并执行完整I/O\n", elapsed, err)
		t.Error("RED（红灯，缺陷未修复）: Context 超时未正确传播")
	}

	fmt.Println()
	fmt.Println("=== 缺陷检测测试结束 ===")
}

func TestRedirectContext(t *testing.T) {
	fmt.Println("=== 重定向 Context 超时测试 ===")

	cfg := config.Default()
	cfg.Storage.URLFilePath("/tmp/test_urls2.dat")
	cfg.Storage.LogFilePath("/tmp/test_logs2.dat")

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("FAIL: NewURLStore 创建失败: %v", err)
	}
	defer urlStore.Close()

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("FAIL: NewAccessLogStore 创建失败: %v", err)
	}
	defer logStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := urlStore.Load(ctx); err != nil {
		t.Fatalf("FAIL: URLStore.Load 失败: %v", err)
	}
	if err := logStore.Open(ctx); err != nil {
		t.Fatalf("FAIL: AccessLogStore.Open 失败: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("FAIL: NewURLService 创建失败: %v", err)
	}

	createCtx, createCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer createCancel()

	req := &model.CreateReq{
		RawURL: "https://example.com/redirect/test",
	}
	su, err := urlSvc.Create(createCtx, req)
	if err != nil {
		t.Fatalf("FAIL: 创建短链失败: %v", err)
	}
	fmt.Printf("  已创建短链: %s -> %s\n", su.Code, su.RawURL)

	redirectSvc, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("FAIL: NewRedirectService 创建失败: %v", err)
	}

	fmt.Println("  使用500毫秒超时 context 调用 HandleRedirect")
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer timeoutCancel()

	redirectReq := &service.RedirectRequest{
		Code:      su.Code,
		Timestamp: time.Now(),
	}

	start := time.Now()
	result, err := redirectSvc.HandleRedirect(timeoutCtx, redirectReq)
	elapsed := time.Since(start)

	fmt.Printf("  操作耗时: %v\n", elapsed)
	if err != nil {
		fmt.Printf("  返回错误: %v\n", err)
	} else {
		fmt.Printf("  返回结果: status=%d, url=%s\n", result.Status, result.RawURL)
	}

	if err != nil && elapsed < 1*time.Second {
		fmt.Println("  结论: GREEN（绿灯）")
		t.Log("GREEN（绿灯，缺陷已修复）")
	} else {
		fmt.Println("  结论: RED（红灯）")
		t.Error("RED（红灯，缺陷未修复）: HandleRedirect 中 context 超时未传播")
	}

	fmt.Println()
}

func TestSnapshotConsistency(t *testing.T) {
	fmt.Println("=== 快照一致性测试 ===")

	cfg := config.Default()
	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("FAIL: NewURLStore 创建失败: %v", err)
	}
	defer urlStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := urlStore.Load(ctx); err != nil {
		t.Fatalf("FAIL: Load 失败: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("FAIL: NewURLService 创建失败: %v", err)
	}

	for i := 0; i < 3; i++ {
		req := &model.CreateReq{
			RawURL: fmt.Sprintf("https://example.com/item/%d", i),
		}
		createCtx, createCancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, err := urlSvc.Create(createCtx, req)
		createCancel()
		if err != nil {
			t.Fatalf("FAIL: 创建第%d项失败: %v", i, err)
		}
	}

	snapshot := urlStore.RawSnapshot()
	fmt.Printf("  快照中的条目数: %d\n", len(snapshot))
	if len(snapshot) != 3 {
		t.Errorf("  期望3个条目，实际%d个", len(snapshot))
	}

	for code, su := range snapshot {
		if su.Code != code {
			t.Errorf("  快照不一致: key=%s, Code=%s", code, su.Code)
		}
		if su.CreatedAt.IsZero() {
			t.Errorf("  条目 %s 的 CreatedAt 为零值", code)
		}
	}

	if len(snapshot) == 3 {
		fmt.Println("  结论: GREEN（绿灯）- 快照一致")
	} else {
		fmt.Println("  结论: RED（红灯）- 快照不一致")
		t.Error("RED（红灯，缺陷未修复）: 快照中的条目数不匹配")
	}

	fmt.Println()
}
