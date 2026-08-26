package config_center

import (
	"context"
	"fmt"
	"testing"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/internal/service"
)

func TestRedGreen(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()

	cfg.Storage.
		URLFilePath("/tmp/test_urls.json").
		LogFilePath("/tmp/test_logs.json").
		SyncInterval(time.Second).
		FlushOnWrite(true)

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

	if err := logStore.Open(ctx); err != nil {
		t.Fatalf("failed to open AccessLogStore: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("failed to create URLService: %v", err)
	}

	redirectSvc, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("failed to create RedirectService: %v", err)
	}

	panicGuard := func(code, rawURL string) bool {
		return code == "trigger"
	}
	urlStore.SetPanicGuard(panicGuard)

	snapshotBefore := urlStore.RawSnapshot()
	t.Logf("snapshot before fill: %d entries", len(snapshotBefore))

	for i := 0; i < 100; i++ {
		auditLog := model.NewAuditLog(
			model.ActionCreate, "short_url", fmt.Sprintf("code-%d", i),
			"test", "default", "tester", "127.0.0.1",
			fmt.Sprintf("audit log %d", i),
			"details",
			"success",
		)
		if err := urlStore.CreateAuditLog(ctx, auditLog); err != nil {
			t.Logf("audit log %d returned error (expected near capacity): %v", i, err)
			break
		}
	}

	req := &model.CreateReq{
		RawURL:     "https://example.com/test-page",
		CustomCode: "testcode1",
		MaxVisits:  100,
	}

	shortURL, createErr := urlSvc.Create(ctx, req)

	if createErr != nil {
		t.Log("GREEN（绿灯，缺陷已修复）")
		t.Logf("URLService.Create returned error as expected when audit log storage is full: %v", createErr)

		snapshot := urlStore.RawSnapshot()
		if len(snapshot) > 0 {
			t.Logf("snapshot contains %d entries after failed create", len(snapshot))
		}

		if shortURL != nil {
			t.Error("expected nil short URL when creation fails, got non-nil")
		}
	} else {
		t.Log("RED（红灯，缺陷未修复）")
		t.Log("URLService.Create returned success despite audit log storage being full")
		t.Log("This indicates the error from CreateAuditLog was silently ignored")
		t.Log("The audit log write failure was swallowed and the operation appeared successful")

		if shortURL != nil {
			t.Logf("short URL was saved: code=%s, rawURL=%s", shortURL.Code, shortURL.RawURL)
		}

		t.Errorf("expected error from Create when audit log storage is full, but got nil")
	}

	_ = redirectSvc

	t.Log("Verification complete. Check output above for RED/GREEN determination.")
}
