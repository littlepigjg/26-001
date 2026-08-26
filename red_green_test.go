package config_center_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"config-center/internal/config"
	"config-center/internal/handler"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

// mockResponseWriter captures the HTTP status code and body for verification.
type mockResponseWriter struct {
	statusCode int
	headers    http.Header
	body       bytes.Buffer
}

func (m *mockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return m.body.Write(b)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

func TestRedGreen(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.URLFilePath("/tmp/test_url_store.json")
	cfg.Storage.LogFilePath("/tmp/test_access.log")
	cfg.Storage.SyncInterval(5 * time.Second)
	cfg.Storage.FlushOnWrite(true)

	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore failed: %v", err)
	}
	defer us.Close()

	ctx := context.Background()
	if err := us.Load(ctx); err != nil {
		t.Fatalf("URLStore.Load failed: %v", err)
	}

	ls, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore failed: %v", err)
	}
	defer ls.Close()

	if err := ls.Open(ctx); err != nil {
		t.Fatalf("AccessLogStore.Open failed: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("NewURLService failed: %v", err)
	}

	redirectSvc, err := service.NewRedirectService(us, ls)
	if err != nil {
		t.Fatalf("NewRedirectService failed: %v", err)
	}

	hasFailure := false

	// Test 1: Direct store error — should work with direct type assertion (no wrapping).
	// Store returns *AppError directly, HandleError can identify it.
	t.Run("direct_store_error_returns_404", func(t *testing.T) {
		_, err := us.Get("nonexistent_code")
		if err == nil {
			t.Fatal("expected error for non-existent code")
		}

		w := &mockResponseWriter{}
		handler.HandleError(w, err)

		if w.statusCode == 404 {
			fmt.Println("GREEN（绿灯，缺陷已修复）— 直接返回的 *AppError 可以被 HandleError 正确识别为 404")
		} else {
			fmt.Printf("RED（红灯，缺陷未修复）— 直接 store 错误返回 %d 而非 404\n", w.statusCode)
			hasFailure = true
			t.Errorf("expected 404 for direct store error, got %d", w.statusCode)
		}
	})

	// Test 2: Service-wrapped not-found error — the core defect.
	// Service wraps *AppError with fmt.Errorf("...%w", err).
	// HandleError uses err.(*model.AppError) which fails on wrapped errors.
	// Expected: 404 (not found), Actual with defect: 500 (internal error).
	t.Run("service_wrapped_not_found_should_return_404", func(t *testing.T) {
		_, err := redirectSvc.HandleRedirect(ctx, &service.RedirectRequest{Code: "nonexistent_xyz"})
		if err == nil {
			t.Fatal("expected error for non-existent redirect code")
		}

		w := &mockResponseWriter{}
		handler.HandleError(w, err)

		if w.statusCode == 404 {
			fmt.Println("GREEN（绿灯，缺陷已修复）— HandleError 能正确遍历 %w 错误链，识别被包裹的 *AppError")
		} else {
			fmt.Printf("RED（红灯，缺陷未修复）— HandleError 使用 err.(*model.AppError) 直接类型断言失败，返回 %d 而非 404，被 %%w 包裹的 *AppError 无法被识别\n", w.statusCode)
			hasFailure = true
			t.Errorf("expected 404 for wrapped not-found error, got %d", w.statusCode)
		}
	})

	// Test 3: Create flow — basic CRUD works end-to-end.
	t.Run("create_and_get_flow", func(t *testing.T) {
		req := &model.CreateReq{
			RawURL:     "https://example.com/test",
			CustomCode: "custom123",
			MaxVisits:  100,
		}

		shortURL, err := urlSvc.Create(ctx, req)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		t.Logf("Created short URL: code=%s", shortURL.Code)

		got, err := us.Get(shortURL.Code)
		if err != nil {
			t.Fatalf("Get after Create failed: %v", err)
		}
		if got.Code != shortURL.Code {
			t.Errorf("code mismatch: got %s, want %s", got.Code, shortURL.Code)
		}

		snapshot := us.RawSnapshot()
		if len(snapshot) < 1 {
			t.Error("snapshot should have at least 1 entry")
		}

		fmt.Println("GREEN（绿灯，缺陷已修复）— 基本 CRUD 流程正常")
	})

	// Test 4: Service-wrapped conflict error — second aspect of the defect.
	// Duplicate creation returns *AppError wrapped with %w.
	// HandleError cannot identify it, returns 500 instead of 409.
	t.Run("service_wrapped_conflict_should_return_409", func(t *testing.T) {
		req := &model.CreateReq{
			RawURL:     "https://example.com/dup",
			CustomCode: "dup_code",
			MaxVisits:  50,
		}
		_, err := urlSvc.Create(ctx, req)
		if err != nil {
			t.Fatalf("First Create failed: %v", err)
		}

		_, err = urlSvc.Create(ctx, req)
		if err == nil {
			t.Fatal("expected duplicate error on second Create")
		}

		w := &mockResponseWriter{}
		handler.HandleError(w, err)

		if w.statusCode == 409 {
			fmt.Println("GREEN（绿灯，缺陷已修复）— 重复创建错误被正确识别为 409")
		} else {
			fmt.Printf("RED（红灯，缺陷未修复）— 重复创建错误返回 %d 而非 409，HandleError 无法识别被 %%w 包裹的 *AppError\n", w.statusCode)
			hasFailure = true
			t.Errorf("expected 409 for duplicate error, got %d", w.statusCode)
		}
	})

	// Test 5: Verifies that direct *AppError from store still works
	// even after service-layer wrapping is introduced.
	t.Run("wrapped_validation_error_should_return_400", func(t *testing.T) {
		invalidReq := &model.CreateReq{
			RawURL:     "",
			CustomCode: "inv",
			MaxVisits:  -1,
		}
		_, err := urlSvc.Create(ctx, invalidReq)
		if err == nil {
			t.Fatal("expected validation error for invalid request")
		}

		w := &mockResponseWriter{}
		handler.HandleError(w, err)

		if w.statusCode == 400 {
			fmt.Println("GREEN（绿灯，缺陷已修复）— 校验错误被正确识别为 400")
		} else {
			fmt.Printf("RED（红灯，缺陷未修复）— 校验错误返回 %d 而非 400，HandleError 无法识别被包裹的 *AppError\n", w.statusCode)
			hasFailure = true
			t.Errorf("expected 400 for validation error, got %d", w.statusCode)
		}
	})

	fmt.Println()
	if hasFailure {
		fmt.Println("========================================")
		fmt.Println("  最终判定: RED（红灯）")
		fmt.Println("  缺陷存在，部分测试失败")
		fmt.Println("  根因: HandleError 使用 err.(*model.AppError)")
		fmt.Println("  直接类型断言，无法识别被")
		fmt.Println("  fmt.Errorf(\"...: %%w\", err) 包裹的 *AppError")
		fmt.Println("  修复: 使用 errors.As(err, &appErr)")
		fmt.Println("========================================")
	} else {
		fmt.Println("========================================")
		fmt.Println("  最终判定: GREEN（绿灯）")
		fmt.Println("  缺陷已修复，所有测试通过")
		fmt.Println("========================================")
	}
}
