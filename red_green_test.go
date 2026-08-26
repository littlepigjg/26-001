package main

import (
	"context"
	"fmt"
	"strings"
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

	_, err := appSvc.CreateApp(ctx, "order-service", "Order Service", "order management", "team-alpha")
	if err != nil {
		t.Fatalf("CreateApp failed: %v", err)
	}
	appSvc.AddEnvironment(ctx, "order-service", "production")

	cfgItems := []*model.ConfigItem{
		model.NewConfigItem("cfg-1", "order-service", "production", "database.url", "postgres://localhost:5432/orders", "Database connection", "string", "admin"),
		model.NewConfigItem("cfg-2", "order-service", "production", "database.pool_size", "20", "Connection pool size", "number", "admin"),
		model.NewConfigItem("cfg-3", "order-service", "production", "cache.ttl", "3600", "Cache TTL in seconds", "number", "admin"),
		model.NewConfigItem("cfg-4", "order-service", "production", "feature.enable_v2", "true", "Enable v2 feature", "boolean", "admin"),
	}
	for _, item := range cfgItems {
		if err := memStore.CreateConfig(ctx, item); err != nil {
			t.Fatalf("CreateConfig failed: %v", err)
		}
	}

	clientSvc := service.NewClientService(memStore, appSvc, true)
	defer clientSvc.Close()

	callCount := 0
	transform := func(m map[string]string) map[string]string {
		callCount++
		key := fmt.Sprintf("_transform_pass_%d", callCount)
		m[key] = time.Now().UTC().Format(time.RFC3339Nano)
		return m
	}
	clientSvc.SetConfigTransform(transform)

	result1, err := clientSvc.PullConfig(ctx, "order-service", "production", "")
	if err != nil {
		t.Fatalf("PullConfig first call failed: %v", err)
	}
	if result1.Config == nil {
		t.Fatal("First PullConfig returned nil Config")
	}
	if !result1.Modified {
		t.Fatal("First PullConfig should indicate Modified=true")
	}

	result1.Config["_client_side_mutation"] = "injected-by-test"

	result2, err := clientSvc.PullConfig(ctx, "order-service", "production", result1.ETag)
	if err != nil {
		t.Fatalf("PullConfig second call failed: %v", err)
	}
	if result2.Config == nil {
		t.Fatal("Second PullConfig returned nil Config")
	}

	foundMutation := false
	foundPass2 := false
	foundPass1 := false
	for k := range result2.Config {
		if k == "_client_side_mutation" {
			foundMutation = true
		}
		if k == "_transform_pass_1" {
			foundPass1 = true
		}
		if k == "_transform_pass_2" {
			foundPass2 = true
		}
	}

	if foundMutation {
		fmt.Println("RED（红灯，缺陷未修复）")
		fmt.Println("缺陷表现: 第二次 PullConfig 返回的 config 被第一次调用方的修改污染，说明缓存中的 map 是共享引用而非副本")
		fmt.Printf("  - _client_side_mutation 出现在第二次返回结果中: %v\n", foundMutation)
		fmt.Printf("  - _transform_pass_1 仍存在于第二次结果中: %v\n", foundPass1)
		fmt.Printf("  - _transform_pass_2 被追加到同一 map 中: %v\n", foundPass2)
		t.Errorf("State pollution detected: config map returned by second PullConfig contains caller-injected key '_client_side_mutation'")
		return
	}

	transformedKeys := 0
	for k := range result2.Config {
		if strings.HasPrefix(k, "_transform_pass_") {
			transformedKeys++
		}
	}
	if transformedKeys > 1 {
		fmt.Println("RED（红灯，缺陷未修复）")
		fmt.Println("缺陷表现: config transform 在缓存命中时被重复执行，导致同一 map 上累积多个 _transform_pass_N 键")
		fmt.Printf("  - 检测到 %d 个 _transform_pass_* 键，预期仅 1 个\n", transformedKeys)
		t.Errorf("Config transform executed multiple times on same cached map: found %d _transform_pass_* keys", transformedKeys)
		return
	}

	fmt.Println("GREEN（绿灯，缺陷已修复）")
	fmt.Println("所有检查通过: config map 在 PullConfig 调用间正确隔离，缓存返回的是独立副本")
}
