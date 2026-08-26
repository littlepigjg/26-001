package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

// TestRedGreen 验证配置回滚和批量更新操作中的状态污染缺陷。
// 当 ReplaceConfigMap 操作失败时，旧配置已被清空但新配置未完全写入，
// 导致配置状态被污染。修复后，应该能够正确回滚或报告错误。
func TestRedGreen(t *testing.T) {
	ctx := context.Background()

	// 初始化存储和服务
	memStore := store.NewMemoryStore()
	appSvc := service.NewAppService(memStore)
	configSvc := service.NewConfigService(memStore, appSvc)
	versionSvc := service.NewVersionService(memStore, appSvc, configSvc)
	auditSvc := service.NewAuditService(memStore)
	rollbackSvc := service.NewRollbackService(memStore, appSvc, configSvc, versionSvc, auditSvc)

	// 创建应用
	_, err := appSvc.CreateApp(ctx, "test-app", "Test App", "Test Application", "admin")
	if err != nil {
		t.Fatalf("创建应用失败: %v", err)
	}

	// 创建初始配置
	now := time.Now()
	configItems := []*model.ConfigItem{
		{
			ID: "cfg-test-app-dev-db_host", AppID: "test-app", Environment: "dev",
			Key: "db.host", Value: "localhost", Format: "string", Version: 1,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "cfg-test-app-dev-db_port", AppID: "test-app", Environment: "dev",
			Key: "db.port", Value: "5432", Format: "number", Version: 1,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "cfg-test-app-dev-redis_host", AppID: "test-app", Environment: "dev",
			Key: "redis.host", Value: "127.0.0.1", Format: "string", Version: 1,
			CreatedAt: now, UpdatedAt: now,
		},
	}

	// 写入初始配置
	for _, item := range configItems {
		_, err := configSvc.CreateConfig(ctx, item.AppID, item.Environment, item.Key, item.Value, "", item.Format, "admin")
		if err != nil {
			t.Fatalf("创建配置 %s 失败: %v", item.Key, err)
		}
	}

	// 创建第一个版本快照
	_, err = versionSvc.CreateVersion(ctx, "test-app", "dev", "admin", "initial version")
	if err != nil {
		t.Fatalf("创建版本快照失败: %v", err)
	}

	// 修改配置（模拟版本2）
	_, err = configSvc.UpdateConfig(ctx, "test-app", "dev", "db.host", "10.0.0.1", "", "admin")
	if err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}
	_, err = configSvc.UpdateConfig(ctx, "test-app", "dev", "db.port", "3306", "", "admin")
	if err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	// 创建第二个版本快照
	_, err = versionSvc.CreateVersion(ctx, "test-app", "dev", "admin", "second version")
	if err != nil {
		t.Fatalf("创建第二个版本快照失败: %v", err)
	}

	// 验证当前配置
	currentConfig, err := configSvc.GetConfigMap(ctx, "test-app", "dev")
	if err != nil {
		t.Fatalf("获取当前配置失败: %v", err)
	}
	if currentConfig["db.host"] != "10.0.0.1" {
		t.Fatalf("当前 db.host 应该是 10.0.0.1，实际是 %s", currentConfig["db.host"])
	}

	// 设置故障注入钩子：当回滚到版本1时触发失败
	// 这模拟了 ReplaceConfigMap 在清空旧配置后写入新配置时发生异常
	rollbackSvc.SetReplaceFailureGuard(func(appID, env string, configs []*model.ConfigItem) bool {
		// 当配置项中包含 db.host 且值为 localhost 时触发（版本1的特征）
		for _, cfg := range configs {
			if cfg != nil && cfg.Key == "db.host" && cfg.Value == "localhost" {
				return true
			}
		}
		return false
	})

	// 执行回滚到版本1 - 这会触发状态污染
	result, err := rollbackSvc.Rollback(ctx, "test-app", "dev", 1, "admin", "127.0.0.1")

	// 检查结果 - 修复后应该返回错误而不是成功
	// 缺陷存在时：rollback 返回成功（即使配置已被污染），config map 为空
	// 修复后：rollback 应该返回错误，或者正确恢复配置
	_ = result

	// 检查回滚后的配置状态
	postRollbackConfig, err := configSvc.GetConfigMap(ctx, "test-app", "dev")
	if err != nil {
		t.Fatalf("获取回滚后配置失败: %v", err)
	}

	// 缺陷存在时：配置被清空（旧配置已删除，新配置未写入）
	// 修复后：配置应该包含版本1的值，或者返回了错误
	configIsEmpty := len(postRollbackConfig) == 0
	configIsCorrect := postRollbackConfig["db.host"] == "localhost" &&
		postRollbackConfig["db.port"] == "5432" &&
		postRollbackConfig["redis.host"] == "127.0.0.1"

	// 判断：如果配置为空（状态污染），说明缺陷存在
	if configIsEmpty && err == nil {
		// 配置被清空了，这是状态污染的表现
		// 但我们还需要检查 rollback 返回的错误
		// 如果 rollback 返回了错误但配置仍被清空，说明修复不完整
		fmt.Println("RED（红灯，缺陷未修复）：回滚操作导致配置状态被污染，旧配置已丢失，新配置未写入")
		t.Errorf("状态污染缺陷：回滚后配置为空，但应该包含版本1的配置数据")
		return
	}

	// 如果配置正确恢复了，检查 rollback 是否正确报告了错误
	// 修复后：rollback 应该在 ReplaceConfigMap 失败时返回错误
	if configIsEmpty {
		fmt.Println("RED（红灯，缺陷未修复）：配置被清空且未返回错误")
		t.Errorf("状态污染缺陷：配置被清空，应返回错误并保留原配置")
		return
	}

	// 如果配置正确且 rollback 正确处理了错误
	if err == nil && !configIsEmpty && !configIsCorrect {
		// 配置存在但不正确
		fmt.Println("RED（红灯，缺陷未修复）：配置状态不一致，可能部分写入")
		t.Errorf("状态污染缺陷：配置部分写入，状态不一致")
		return
	}

	// 检查是否正确处理了错误返回
	// 修复后：当 ReplaceConfigMap 失败时，Rollback 应该返回错误
	// 如果 Rollback 返回了错误且配置正确保留，那就是修复正确的
	if err != nil && configIsCorrect {
		fmt.Println("GREEN（绿灯，缺陷已修复）：Rollback 正确返回错误，原配置被保留")
		return
	}

	// 如果配置正确且没有错误（修复后的正常情况）
	if configIsCorrect {
		fmt.Println("GREEN（绿灯，缺陷已修复）：配置正确恢复到版本1")
		return
	}

	// 其他异常情况
	fmt.Printf("RED（红灯，缺陷未修复）：未知状态，配置=%v, err=%v\n", postRollbackConfig, err)
	t.Errorf("状态污染缺陷：未预期的状态")
}

// TestBatchUpdateStatePollution 验证批量更新中的状态污染缺陷。
func TestBatchUpdateStatePollution(t *testing.T) {
	ctx := context.Background()

	// 初始化存储和服务
	memStore := store.NewMemoryStore()
	appSvc := service.NewAppService(memStore)
	configSvc := service.NewConfigService(memStore, appSvc)

	// 创建应用
	_, err := appSvc.CreateApp(ctx, "batch-app", "Batch App", "Test", "admin")
	if err != nil {
		t.Fatalf("创建应用失败: %v", err)
	}

	// 创建初始配置
	now := time.Now()
	initialItems := []*model.ConfigItem{
		{
			ID: "cfg-batch-app-dev-key1", AppID: "batch-app", Environment: "dev",
			Key: "key1", Value: "value1", Format: "string", Version: 1,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "cfg-batch-app-dev-key2", AppID: "batch-app", Environment: "dev",
			Key: "key2", Value: "value2", Format: "string", Version: 1,
			CreatedAt: now, UpdatedAt: now,
		},
	}

	for _, item := range initialItems {
		_, err := configSvc.CreateConfig(ctx, item.AppID, item.Environment, item.Key, item.Value, "", item.Format, "admin")
		if err != nil {
			t.Fatalf("创建配置失败: %v", err)
		}
	}

	// 验证初始配置
	initialConfig, _ := configSvc.GetConfigMap(ctx, "batch-app", "dev")
	if len(initialConfig) != 2 {
		t.Fatalf("初始配置应该有2项，实际有 %d 项", len(initialConfig))
	}

	// 设置故障注入钩子
	configSvc.SetReplaceFailureGuard(func(appID, env string, configs []*model.ConfigItem) bool {
		// 当有多个配置项时触发失败
		return len(configs) > 1
	})

	// 准备批量更新的新项目
	batchItems := []*model.ConfigItem{
		{
			ID: "cfg-batch-app-dev-new1", AppID: "batch-app", Environment: "dev",
			Key: "newkey1", Value: "newvalue1", Format: "string", Version: 2,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "cfg-batch-app-dev-new2", AppID: "batch-app", Environment: "dev",
			Key: "newkey2", Value: "newvalue2", Format: "string", Version: 2,
			CreatedAt: now, UpdatedAt: now,
		},
	}

	// 执行批量更新 - 这会触发状态污染
	err = configSvc.BatchUpdateConfig(ctx, "batch-app", "dev", batchItems, "admin")

	// 缺陷存在时：BatchUpdateConfig 返回成功（即使配置已被污染）
	// 修复后：应该返回错误
	_ = err

	// 检查批量更新后的配置状态
	postBatchConfig, _ := configSvc.GetConfigMap(ctx, "batch-app", "dev")

	// 缺陷存在时：配置被清空
	if len(postBatchConfig) == 0 {
		fmt.Println("RED（红灯，缺陷未修复）：批量更新导致配置状态被污染，配置被清空")
		t.Errorf("状态污染缺陷：批量更新后配置为空")
		return
	}

	// 如果配置仍包含旧值（因为失败钩子触发，旧配置被清空，新配置未写入）
	if _, exists := postBatchConfig["key1"]; exists {
		// 旧配置还在，但这不正常
		fmt.Println("RED（红灯，缺陷未修复）：配置状态不一致")
		t.Errorf("状态污染缺陷：批量更新后配置状态异常")
		return
	}

	// 修复后：应该返回错误并且保留原配置
	if err != nil {
		// 检查原配置是否被保留
		if val, exists := postBatchConfig["key1"]; exists && val == "value1" {
			fmt.Println("GREEN（绿灯，缺陷已修复）：BatchUpdateConfig 返回错误，原配置被保留")
			return
		}
	}

	// 如果新配置正确写入
	if postBatchConfig["newkey1"] == "newvalue1" && postBatchConfig["newkey2"] == "newvalue2" {
		fmt.Println("GREEN（绿灯，缺陷已修复）：配置正确更新")
		return
	}

	fmt.Printf("RED（红灯，缺陷未修复）：未知状态，配置=%v, err=%v\n", postBatchConfig, err)
	t.Errorf("状态污染缺陷：未预期的状态")
}
