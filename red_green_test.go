package config_center_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/internal/service"
)

func setupTestEnv(t *testing.T) (*config.Config, *store.URLStore, *store.AccessLogStore, *service.URLService, *service.RedirectService) {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.Storage.URLFilePath(filepath.Join(tmpDir, "urls.dat"))
	cfg.Storage.LogFilePath(filepath.Join(tmpDir, "access.log"))
	cfg.Storage.SyncInterval(100 * time.Millisecond)
	cfg.Storage.FlushOnWrite(true)

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore failed: %v", err)
	}

	err = urlStore.Load(context.Background())
	if err != nil {
		t.Fatalf("urlStore.Load failed: %v", err)
	}

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore failed: %v", err)
	}

	err = logStore.Open(context.Background())
	if err != nil {
		t.Fatalf("logStore.Open failed: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("NewURLService failed: %v", err)
	}

	redirectSvc, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("NewRedirectService failed: %v", err)
	}

	t.Cleanup(func() {
		urlStore.Close()
		logStore.Close()
		os.RemoveAll(tmpDir)
	})

	return cfg, urlStore, logStore, urlSvc, redirectSvc
}

func TestContextLeakInURLService_Create(t *testing.T) {
	_, urlStore, _, urlSvc, _ := setupTestEnv(t)

	initialCtxCount := urlStore.ActiveContextCount()

	for i := 0; i < 50; i++ {
		req := &model.CreateReq{
			RawURL:    "https://example.com/path",
			CustomCode: fmt.Sprintf("test-%d", i),
			MaxVisits: 0,
		}
		_, err := urlSvc.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("Create failed at iteration %d: %v", i, err)
		}
	}

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)

	currentCtxCount := urlStore.ActiveContextCount()
	increase := currentCtxCount - initialCtxCount

	if increase > 0 {
		t.Logf("RED: Detected %d leaked context(s) after 50 Create calls (initial=%d, current=%d)",
			increase, initialCtxCount, currentCtxCount)
		t.Errorf("CONTEXT LEAK: %d context(s) not properly canceled after URLService.Create calls.", increase)
	} else {
		t.Logf("GREEN: No context leak detected after 50 Create calls (active=%d)", currentCtxCount)
	}
}

func TestContextLeakInRedirectService_HandleRedirect(t *testing.T) {
	_, urlStore, _, urlSvc, redirectSvc := setupTestEnv(t)

	codes := make([]string, 50)
	for i := 0; i < 50; i++ {
		req := &model.CreateReq{
			RawURL:    "https://example.com/redirect",
			CustomCode: fmt.Sprintf("redir-%d", i),
		}
		url, err := urlSvc.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		codes[i] = url.Code
	}

	// Wait for contexts from Create to accumulate if any
	time.Sleep(50 * time.Millisecond)
	createCtxCount := urlStore.ActiveContextCount()

	for i, code := range codes {
		req := &service.RedirectRequest{
			Code:      code,
			Timestamp: time.Now(),
		}
		_, err := redirectSvc.HandleRedirect(context.Background(), req)
		if err != nil {
			t.Fatalf("HandleRedirect failed at %d: %v", i, err)
		}
	}

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)

	currentCtxCount := urlStore.ActiveContextCount()
	increase := currentCtxCount - createCtxCount

	if increase > 0 {
		t.Logf("RED: Detected %d leaked context(s) after 50 HandleRedirect calls (baseline=%d, current=%d)",
			increase, createCtxCount, currentCtxCount)
		t.Errorf("CONTEXT LEAK: %d context(s) not properly canceled after RedirectService.HandleRedirect calls.", increase)
	} else {
		t.Logf("GREEN: No context leak detected after 50 HandleRedirect calls (active=%d)", currentCtxCount)
	}
}

func TestContextLeakCrossService(t *testing.T) {
	_, urlStore, _, urlSvc, redirectSvc := setupTestEnv(t)

	initialCtxCount := urlStore.ActiveContextCount()

	// Mix of operations that create child contexts
	for i := 0; i < 30; i++ {
		createReq := &model.CreateReq{
			RawURL:    "https://example.com/mixed",
			CustomCode: fmt.Sprintf("mix-%d", i),
		}
		_, err := urlSvc.Create(context.Background(), createReq)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		redirectReq := &service.RedirectRequest{
			Code:      fmt.Sprintf("mix-%d", i),
			Timestamp: time.Now(),
		}
		_, err = redirectSvc.HandleRedirect(context.Background(), redirectReq)
		if err != nil {
			t.Fatalf("HandleRedirect failed: %v", err)
		}

		validateReq := &service.RedirectRequest{
			Code:      fmt.Sprintf("mix-%d", i),
			Timestamp: time.Now(),
		}
		err = redirectSvc.ValidateRedirect(context.Background(), validateReq)
		if err != nil {
			t.Fatalf("ValidateRedirect failed: %v", err)
		}
	}

	// Give goroutines time to start
	time.Sleep(100 * time.Millisecond)

	currentCtxCount := urlStore.ActiveContextCount()
	increase := currentCtxCount - initialCtxCount

	if increase > 0 {
		t.Logf("RED: Detected %d total leaked context(s) after 90 mixed operations (initial=%d, current=%d)",
			increase, initialCtxCount, currentCtxCount)
		t.Errorf("CONTEXT LEAK (cross-service): %d context(s) not properly canceled. "+
			"URLService.Create and RedirectService.HandleRedirect/ValidateRedirect leak child contexts.", increase)
	} else {
		t.Logf("GREEN: No context leak detected after 90 mixed operations (active=%d)", currentCtxCount)
	}
}

func TestContextLeakInBatchRedirect(t *testing.T) {
	_, urlStore, _, urlSvc, redirectSvc := setupTestEnv(t)

	codes := make([]string, 20)
	for i := 0; i < 20; i++ {
		req := &model.CreateReq{
			RawURL:    "https://example.com/batch",
			CustomCode: fmt.Sprintf("batch-%d", i),
		}
		url, err := urlSvc.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		codes[i] = url.Code
	}

	time.Sleep(50 * time.Millisecond)
	createCtxCount := urlStore.ActiveContextCount()

	reqs := make([]service.RedirectRequest, 0, len(codes))
	for _, code := range codes {
		reqs = append(reqs, service.RedirectRequest{
			Code:      code,
			Timestamp: time.Now(),
		})
	}

	_, err := redirectSvc.BatchRedirect(context.Background(), reqs)
	if err != nil {
		t.Fatalf("BatchRedirect failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	currentCtxCount := urlStore.ActiveContextCount()
	increase := currentCtxCount - createCtxCount

	if increase > 0 {
		t.Logf("RED: Detected %d leaked context(s) after BatchRedirect (baseline=%d, current=%d)",
			increase, createCtxCount, currentCtxCount)
		t.Errorf("CONTEXT LEAK: BatchRedirect creates child contexts that are not properly canceled. "+
			"Leaked: %d context(s).", increase)
	} else {
		t.Logf("GREEN: No context leak detected after BatchRedirect (active=%d)", currentCtxCount)
	}
}

func TestContextLeakFinalDiagnosis(t *testing.T) {
	_, urlStore, _, urlSvc, redirectSvc := setupTestEnv(t)

	initialCtxCount := urlStore.ActiveContextCount()

	// Perform operations across all three functions that create contexts
	for i := 0; i < 20; i++ {
		req := &model.CreateReq{
			RawURL:    "https://example.com/diag",
			CustomCode: fmt.Sprintf("diag-%d", i),
		}
		_, err := urlSvc.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		_, err = redirectSvc.HandleRedirect(context.Background(), &service.RedirectRequest{
			Code:      fmt.Sprintf("diag-%d", i),
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("HandleRedirect failed: %v", err)
		}

		err = redirectSvc.ValidateRedirect(context.Background(), &service.RedirectRequest{
			Code:      fmt.Sprintf("diag-%d", i),
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("ValidateRedirect failed: %v", err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	finalCtxCount := urlStore.ActiveContextCount()
	increase := finalCtxCount - initialCtxCount

	if increase > 0 {
		t.Logf("RED FINAL DIAGNOSIS: %d leaked context(s) detected", increase)
		t.Log("")
		t.Log("Defect Summary:")
		t.Log("  Location 1: internal/store/url_store.go PrepareContext()")
		t.Log("  - context.WithCancel() creates child context with real cancel function")
		t.Log("  - Returns no-op func() {} instead of real cancel")
		t.Log("  - Real cancel stored in trackedCtxs but never called externally")
		t.Log("")
		t.Log("  Location 2: internal/service/url_service.go Create() and validateAndPrepare()")
		t.Log("  - Calls PrepareContext, defers the returned no-op cancel()")
		t.Log("  - defer cancel() does nothing -> context never actually canceled")
		t.Log("")
		t.Log("  Location 3: internal/service/redirect_service.go HandleRedirect() and ValidateRedirect()")
		t.Log("  - Same pattern: calls PrepareContext, defers no-op cancel")
		t.Log("  - Context leaks accumulate across multiple operations")
		t.Errorf("CONTEXT LEAK CONFIRMED: %d context(s) leaked across all service layers", increase)
	} else {
		t.Log("GREEN FINAL DIAGNOSIS: No context leak detected across all service layers")
	}
}
