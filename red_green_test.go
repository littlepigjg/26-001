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

// TestRedGreen tests context cancellation handling in health checks.
// RED: health check ignores context cancellation, blocking beyond timeout.
// GREEN: health check properly respects context cancellation.
func TestRedGreen(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.URLFilePath("./test_urls.json")
	cfg.Storage.LogFilePath("./test_access.log")

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("failed to create URLStore: %v", err)
	}
	defer urlStore.Close()

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("failed to create AccessLogStore: %v", err)
	}
	defer logStore.Close()

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("failed to create URLService: %v", err)
	}

	redirectSvc, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("failed to create RedirectService: %v", err)
	}

	// Step 1: Create a short URL normally
	req := &model.CreateReq{
		RawURL:     "https://example.com/very/long/url/path",
		CustomCode: "testcode",
		MaxVisits:  100,
	}

	shortURL, err := urlSvc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to create short URL: %v", err)
	}
	if shortURL.Code != "testcode" {
		t.Fatalf("expected code 'testcode', got '%s'", shortURL.Code)
	}

	// Step 2: Verify redirect works
	result, err := redirectSvc.HandleRedirect(context.Background(), &service.RedirectRequest{
		Code:      "testcode",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to handle redirect: %v", err)
	}
	if result.RawURL != "https://example.com/very/long/url/path" {
		t.Fatalf("unexpected redirect result: %s", result.RawURL)
	}

	// Step 3: Test Load with short timeout - context should be respected.
	// HealthCheck internally does ~250ms of work ignoring context.
	// With a 50ms timeout, the operation should abort quickly after timeout fires.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = urlStore.Load(ctx)
	elapsed := time.Since(start)

	// Defect: HealthCheck blocks for ~250ms despite 50ms timeout,
	// and Load returns nil because context cancellation is not checked.
	if err == nil && elapsed > 150*time.Millisecond {
		fmt.Printf("RED（红灯，缺陷未修复）: Load took %v with 50ms timeout, returned nil - context ignored\n", elapsed)
		t.FailNow()
	}

	// Step 4: Test with immediately cancelled context
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()

	err = urlStore.Load(ctx2)
	if err == nil {
		fmt.Printf("RED（红灯，缺陷未修复）: Load with cancelled context returned nil error\n")
		t.FailNow()
	}

	// Step 5: Verify basic operations still work correctly
	saved := &model.ShortURL{
		Code:      "another1",
		RawURL:    "https://example.com/another",
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    false,
		Disabled:  false,
	}
	if err := urlStore.Save(saved, false); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := urlStore.Get("another1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.RawURL != "https://example.com/another" {
		t.Fatalf("unexpected RawURL: %s", got.RawURL)
	}

	snapshot := urlStore.RawSnapshot()
	if len(snapshot) < 2 {
		t.Fatalf("expected at least 2 entries in snapshot, got %d", len(snapshot))
	}

	fmt.Printf("GREEN（绿灯，缺陷已修复）: all context checks passed\n")
}
