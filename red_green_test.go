package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"config-center/internal/handler"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	var err error

	// 创建内存存储
	memStore := store.NewMemoryStore()

	// 创建服务
	appSvc := service.NewAppService(memStore)
	configSvc := service.NewConfigService(memStore, appSvc)
	versionSvc := service.NewVersionService(memStore, appSvc, configSvc)
	auditSvc := service.NewAuditService(memStore)
	rollbackSvc := service.NewRollbackService(memStore, appSvc, configSvc, versionSvc, auditSvc)
	validationSvc := service.NewValidationService(memStore, appSvc)
	diffSvc := service.NewDiffService(memStore, appSvc)
	clientSvc := service.NewClientService(memStore, appSvc, false)

	// 创建处理器
	h := handler.NewHandlers(appSvc, configSvc, versionSvc, clientSvc, auditSvc, rollbackSvc, validationSvc, diffSvc)

	ctx := context.Background()

	// 创建一个应用以便测试
	_, err = appSvc.CreateApp(ctx, "test-app", "Test Application", "For testing", "test-owner")
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// 添加环境
	appSvc.AddEnvironment(ctx, "test-app", "staging")

	// 创建配置
	_, err = configSvc.CreateConfig(ctx, "test-app", "staging", "config.key", "config.value", "Test config", "string", "test-user")
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// 创建版本
	_, err = versionSvc.CreateVersion(ctx, "test-app", "staging", "test-user", "Initial version")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	// 测试1: DiffVersions 使用不存在的版本号
	// 期望：错误信息应包含版本号和操作上下文
	_, err = diffSvc.DiffVersions(ctx, "test-app", "staging", 1, 999)
	if err == nil {
		t.Errorf("Expected error when using non-existent version")
	} else {
		errMsg := err.Error()
		// 检查错误信息是否包含上下文信息
		if !strings.Contains(errMsg, "999") {
			t.Errorf("错误信息应包含版本号999，但实际为: %s", errMsg)
		}
		if !strings.Contains(errMsg, "version") && !strings.Contains(errMsg, "failed to get") {
			t.Errorf("错误信息应包含操作上下文(如'version'或'failed to get')，但实际为: %s", errMsg)
		}
	}

	// 测试2: DiffCurrentVersion 使用不存在的历史版本
	_, err = diffSvc.DiffCurrentVersion(ctx, "test-app", "staging", 999)
	if err == nil {
		t.Errorf("Expected error when using non-existent historical version")
	} else {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "999") {
			t.Errorf("错误信息应包含历史版本号999，但实际为: %s", errMsg)
		}
	}

	// 测试3: CompareVersions 使用不存在的版本
	_, err = versionSvc.CompareVersions(ctx, "test-app", "staging", 1, 999)
	if err == nil {
		t.Errorf("Expected error when using non-existent version in CompareVersions")
	} else {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "999") {
			t.Errorf("错误信息应包含版本号999，但实际为: %s", errMsg)
		}
	}

	// 测试4: Rollback 使用不存在的目标版本
	_, err = rollbackSvc.Rollback(ctx, "test-app", "staging", 999, "test-user", "127.0.0.1")
	if err == nil {
		t.Errorf("Expected error when rolling back to non-existent version")
	} else {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "999") {
			t.Errorf("回滚错误信息应包含目标版本号999，但实际为: %s", errMsg)
		}
	}

	// 测试5: CanRollback 使用不存在的目标版本
	var canRollback bool
	canRollback, err = rollbackSvc.CanRollback(ctx, "test-app", "staging", 999)
	if err == nil {
		t.Errorf("Expected error when checking rollback with non-existent version, got canRollback=%v", canRollback)
	} else {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "999") {
			t.Errorf("CanRollback错误信息应包含目标版本号999，但实际为: %s", errMsg)
		}
	}

	// 测试6: handleError 应该对 AppError 返回正确的 HTTP 状态码
	// 测试 NotFound 错误
	notFoundErr := model.ErrVersionNotFound("test-app", 999)
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	h.DiffConfig(rec, req) // 先初始化响应

	// 直接测试 handleError 的行为
	// 正常情况下，NotFound错误应该返回404状态码
	// 但缺陷中handleError对所有错误都返回500
	_ = notFoundErr
	_ = http.StatusNotFound

	// 通过DiffConfig handler测试错误处理
	req = httptest.NewRequest("GET", "/api/diff?app_id=test-app&environment=staging&v1=1&v2=999", nil)
	rec = httptest.NewRecorder()
	h.DiffConfig(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("获取不存在的版本时应返回404，但实际返回: %d", rec.Code)
	}

	// 测试7: 获取不存在的应用
	_, err = diffSvc.DiffVersions(ctx, "non-existent-app", "staging", 1, 2)
	if err == nil {
		t.Errorf("Expected error when using non-existent app")
	} else {
		// 错误应该是AppError类型
		appErr, ok := err.(*model.AppError)
		if !ok {
			t.Errorf("错误应为*model.AppError类型，但实际类型为: %T", err)
		} else if !appErr.IsNotFound() {
			t.Errorf("错误应为NotFound类型，Code=%d，但实际Code=%d", model.ErrCodeNotFound, appErr.Code)
		}
	}

	// 打印最终判定结果
	// 缺陷存在时：错误信息缺少上下文、HTTP状态码不正确、错误类型丢失
	// 缺陷修复后：错误信息包含完整上下文、HTTP状态码正确、错误类型正确保留
	fmt.Println("TEST COMPLETED - CHECKING RESULTS...")

	// 综合判定
	hasDefect := false

	// 检查错误信息上下文丢失
	_, err = diffSvc.DiffVersions(ctx, "test-app", "staging", 1, 999)
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "999") {
			hasDefect = true
			t.Log("缺陷: DiffVersions错误信息缺少版本号上下文")
		}
	}

	// 检查HTTP状态码不正确
	req = httptest.NewRequest("GET", "/api/diff?app_id=test-app&environment=staging&v1=1&v2=999", nil)
	rec = httptest.NewRecorder()
	h.DiffConfig(rec, req)
	if rec.Code != http.StatusNotFound {
		hasDefect = true
		t.Logf("缺陷: DiffConfig handler返回%d而非404", rec.Code)
	}

	// 检查CanRollback错误处理
	_, err = rollbackSvc.CanRollback(ctx, "test-app", "staging", 999)
	if err == nil {
		hasDefect = true
		t.Log("缺陷: CanRollback对不存在的版本未返回错误")
	} else {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "999") {
			hasDefect = true
			t.Log("缺陷: CanRollback错误信息缺少版本号上下文")
		}
	}

	if hasDefect {
		fmt.Println("RED (红灯，缺陷存在 - 错误传播链断裂)")
		t.Log("RED (红灯，缺陷存在 - 错误传播链断裂)")
	} else {
		fmt.Println("GREEN (绿灯，缺陷已修复)")
		t.Log("GREEN (绿灯，缺陷已修复)")
	}
}
