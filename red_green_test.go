package main

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

	memStore := store.NewMemoryStore()
	defer memStore.Close()

	appSvc := service.NewAppService(memStore)
	configSvc := service.NewConfigService(memStore, appSvc)
	auditSvc := service.NewAuditService(memStore)
	versionSvc := service.NewVersionService(memStore, appSvc, configSvc)

	appID := "test-app"
	env := "dev"
	key := "database.host"
	originalValue := "localhost:5432"
	newValue := "prod-db:5432"
	user := "test-user"

	_, err := appSvc.CreateApp(ctx, appID, "Test App", "Test application", "test-team")
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	_, err = configSvc.CreateConfig(ctx, appID, env, key, originalValue, "Database host", "string", user)
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	_ = auditSvc.LogConfigCreate(ctx, appID, env, key, user, "127.0.0.1")

	_, _, err = versionSvc.AutoSnapshot(ctx, appID, env, user)
	if err != nil {
		t.Fatalf("failed to create initial version: %v", err)
	}

	config, err := configSvc.UpdateConfig(ctx, appID, env, key, newValue, "Database host updated", user)
	if err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	_ = auditSvc.LogConfigChange(ctx, appID, env, key, user, "127.0.0.1", config.Value, config.Value)

	auditFilter := model.AuditLogFilter{
		AppID:    appID,
		Environment: env,
		Action:   model.ActionUpdate,
		User:     user,
	}

	logs, total, err := memStore.ListAuditLogs(ctx, auditFilter)
	if err != nil {
		t.Fatalf("failed to list audit logs: %v", err)
	}

	if total == 0 {
		t.Fatal("no audit logs found for config update - audit logging not working")
	}

	foundChangeLog := false
	var changeLog model.AuditLog
	for _, log := range logs {
		if log.ResourceType == "config" && log.ResourceID == key {
			foundChangeLog = true
			changeLog = log
			break
		}
	}

	if !foundChangeLog {
		t.Fatal("no audit log found for config key change")
	}

	expectedOldValue := originalValue
	oldValueInLog := extractOldValue(changeLog.Details)

	if oldValueInLog == newValue {
		fmt.Println("RED (红灯，缺陷未修复)")
		fmt.Printf("  审计日志中 oldValue='%s' 与 newValue='%s' 相同，无法追溯历史修改\n", oldValueInLog, newValue)
		fmt.Printf("  审计日志详情: %s\n", changeLog.Details)
		t.Fatalf("audit log old value '%s' matches new value '%s', expected '%s' - data flow defect in audit logging",
			oldValueInLog, newValue, expectedOldValue)
	}

	if oldValueInLog != expectedOldValue {
		fmt.Println("RED (红灯，缺陷未修复)")
		fmt.Printf("  审计日志中 oldValue='%s' 与预期值 '%s' 不匹配\n", oldValueInLog, expectedOldValue)
		fmt.Printf("  审计日志详情: %s\n", changeLog.Details)
		t.Fatalf("audit log old value '%s' does not match expected '%s'", oldValueInLog, expectedOldValue)
	}

	fmt.Println("GREEN (绿灯，缺陷已修复)")
	fmt.Printf("  审计日志正确记录了 oldValue='%s' -> newValue='%s'\n", oldValueInLog, newValue)
	fmt.Printf("  审计日志详情: %s\n", changeLog.Details)
}

func extractOldValue(details string) string {
	parts := strings.Split(details, " -> ")
	if len(parts) != 2 {
		return ""
	}
	oldPart := strings.TrimPrefix(parts[0], "old: ")
	return oldPart
}
