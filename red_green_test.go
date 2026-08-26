package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.URLFilePath("/tmp/test_urls.json")
	cfg.Storage.LogFilePath("/tmp/test_logs.json")
	cfg.Storage.SyncInterval(5 * time.Second)
	cfg.Storage.FlushOnWrite(true)

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: NewURLStore failed: %v", err)
	}

	if err := urlStore.Load(context.Background()); err != nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: URLStore.Load failed: %v", err)
	}

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: NewAccessLogStore failed: %v", err)
	}

	if err := logStore.Open(context.Background()); err != nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: AccessLogStore.Open failed: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: NewURLService failed: %v", err)
	}

	redirectSvc, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: NewRedirectService failed: %v", err)
	}

	hasFailures := false

	// Test 1: Create a short URL with a custom code
	createReq1 := &model.CreateReq{
		RawURL:     "https://example.com/page1",
		CustomCode: "test001",
		MaxVisits:  100,
	}

	shortURL1, err := urlSvc.Create(context.Background(), createReq1)
	if err != nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: First Create failed unexpectedly: %v", err)
	}
	if shortURL1.Code != "test001" || shortURL1.RawURL != "https://example.com/page1" {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: Created ShortURL has unexpected fields: %+v", shortURL1)
	}
	t.Log("PASS: Test 1 - First short URL created successfully")

	// Test 2: Try to create another with the same code
	createReq2 := &model.CreateReq{
		RawURL:     "https://example.com/page2",
		CustomCode: "test001",
		MaxVisits:  200,
	}

	_, err = urlSvc.Create(context.Background(), createReq2)
	if err == nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: Duplicate code should return error")
	}

	var dupErr *model.ErrURLCodeAlreadyExists
	if !errors.As(err, &dupErr) {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: errors.As failed to detect ErrURLCodeAlreadyExists\n  Got error: %v\n  Expected errors.As(err, &ErrURLCodeAlreadyExists) to return true", err)
	} else {
		t.Log("PASS: Test 2 - errors.As correctly detected ErrURLCodeAlreadyExists")
	}

	// Test 3: Get a non-existent code
	_, err = urlSvc.Get(context.Background(), "nonexist1")
	if err == nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: Non-existent code should return error")
	}

	var notFoundErr *model.ErrURLCodeNotFound
	if !errors.As(err, &notFoundErr) {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: errors.As failed to detect ErrURLCodeNotFound\n  Got error: %v\n  Expected errors.As(err, &ErrURLCodeNotFound) to return true", err)
	} else {
		t.Log("PASS: Test 3 - errors.As correctly detected ErrURLCodeNotFound")
	}

	// Test 4: Redirect with a non-existent code
	redirectReq := &service.RedirectRequest{
		Code:      "nonexist2",
		Timestamp: time.Now(),
	}

	_, err = redirectSvc.HandleRedirect(context.Background(), redirectReq)
	if err == nil {
		t.Fatalf("RED（红灯，缺陷未修复）\nFAIL: Redirect with non-existent code should return error")
	}

	var redirNotFoundErr *model.ErrURLCodeNotFound
	if !errors.As(err, &redirNotFoundErr) {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: errors.As failed to detect ErrURLCodeNotFound in redirect\n  Got error: %v\n  Expected errors.As(err, &ErrURLCodeNotFound) to return true", err)
	} else {
		t.Log("PASS: Test 4 - Redirect errors.As correctly detected ErrURLCodeNotFound")
	}

	// Test 5: Verify RawSnapshot works
	snapshot := urlStore.RawSnapshot()
	if len(snapshot) != 1 {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: RawSnapshot should have 1 entry, got %d", len(snapshot))
	} else {
		if _, exists := snapshot["test001"]; !exists {
			hasFailures = true
			t.Errorf("RED（红灯，缺陷未修复）\nFAIL: RawSnapshot missing test001 entry")
		} else {
			t.Log("PASS: Test 5 - RawSnapshot contains expected entries")
		}
	}

	// Test 6: Verify SetPanicGuard works
	urlStore.SetPanicGuard(func(code, rawURL string) bool {
		return false
	})

	createReq3 := &model.CreateReq{
		RawURL:     "https://example.com/page3",
		CustomCode: "test002",
		MaxVisits:  50,
	}

	shortURL2, err := urlSvc.Create(context.Background(), createReq3)
	if err != nil {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: Create with PanicGuard disabled should succeed: %v", err)
	} else if shortURL2.Code != "test002" {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: Created wrong code: %s", shortURL2.Code)
	} else {
		t.Log("PASS: Test 6 - SetPanicGuard works correctly")
	}

	// Test 7: Verify ShortURL.Validate works
	invalidURL := &model.ShortURL{
		Code:   "",
		RawURL: "test",
	}
	if err := invalidURL.Validate(); err == nil {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: Empty code should fail validation")
	} else {
		t.Log("PASS: Test 7 - ShortURL.Validate rejects empty code")
	}

	// Test 8: Verify CreateReq.Validate works
	invalidReq := &model.CreateReq{
		RawURL:    "",
		CustomCode: "abc",
		MaxVisits:  -1,
	}
	if err := invalidReq.Validate(); err == nil {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: Invalid CreateReq should fail validation")
	} else {
		t.Log("PASS: Test 8 - CreateReq.Validate rejects invalid input")
	}

	// Test 9: Verify RedirectResult struct works
	result := &service.RedirectResult{
		RawURL: "https://example.com",
		Status: 302,
	}
	if result.RawURL != "https://example.com" || result.Status != 302 {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: RedirectResult has unexpected values")
	} else {
		t.Log("PASS: Test 9 - RedirectResult works correctly")
	}

	// Test 10: Verify ShortURL.IsExpired works
	expiredURL := &model.ShortURL{
		Code:     "exp001",
		RawURL:   "https://example.com",
		Visits:   10001,
		Disabled: false,
	}
	if !expiredURL.IsExpired(time.Now()) {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: ShortURL with max visits should be expired")
	} else {
		t.Log("PASS: Test 10 - ShortURL.IsExpired correctly detects expiration")
	}

	// Test 11: Get second created URL via service
	gotURL, err := urlSvc.Get(context.Background(), "test002")
	if err != nil {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: Get existing URL should not fail: %v", err)
	} else if gotURL.Code != "test002" {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: Get returned wrong URL")
	} else {
		t.Log("PASS: Test 11 - Get existing URL works correctly")
	}

	// Test 12: Verify redirect works for existing URL
	redirectReq2 := &service.RedirectRequest{
		Code:      "test001",
		Timestamp: time.Now(),
	}
	result2, err := redirectSvc.HandleRedirect(context.Background(), redirectReq2)
	if err != nil {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: Redirect existing URL should not fail: %v", err)
	} else if result2.RawURL != "https://example.com/page1" || result2.Status != 302 {
		hasFailures = true
		t.Errorf("RED（红灯，缺陷未修复）\nFAIL: Redirect returned unexpected result")
	} else {
		t.Log("PASS: Test 12 - Redirect existing URL works correctly")
	}

	// Cleanup
	urlStore.Close()
	logStore.Close()

	if !hasFailures {
		t.Log("GREEN（绿灯，缺陷已修复）")
	}
}