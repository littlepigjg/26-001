package config_center

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
	cfg := config.Default()
	cfg.Storage.SyncInterval(5 * time.Second)
	cfg.Storage.FlushOnWrite(true)

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("failed to create URL store: %v", err)
	}

	ctx := context.Background()
	if err := urlStore.Load(ctx); err != nil {
		t.Fatalf("failed to load URL store: %v", err)
	}

	defer urlStore.Close()

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("failed to create URL service: %v", err)
	}

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("failed to create access log store: %v", err)
	}
	if err := logStore.Open(ctx); err != nil {
		t.Fatalf("failed to open access log store: %v", err)
	}
	defer logStore.Close()

	redirectSvc, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("failed to create redirect service: %v", err)
	}

	t.Run("create url and verify integrity", func(t *testing.T) {
		req := &model.CreateReq{
			RawURL:     "https://example.com/original",
			CustomCode: "mycode",
			MaxVisits:  100,
		}

		shortURL, err := urlSvc.Create(ctx, req)
		if err != nil {
			t.Fatalf("first create failed: %v", err)
		}
		if shortURL.Code != "mycode" {
			t.Errorf("expected code 'mycode', got '%s'", shortURL.Code)
		}
		if shortURL.RawURL != "https://example.com/original" {
			t.Errorf("expected raw URL 'https://example.com/original', got '%s'", shortURL.RawURL)
		}
		if shortURL.MaxVisits != 100 {
			t.Errorf("expected max visits 100, got %d", shortURL.MaxVisits)
		}
		if shortURL.Disabled {
			t.Error("newly created URL should not be disabled")
		}

		dupReq := &model.CreateReq{
			RawURL:     "https://example.com/duplicate",
			CustomCode: "mycode",
			MaxVisits:  50,
		}
		_, dupErr := urlSvc.Create(ctx, dupReq)
		if dupErr == nil {
			t.Fatal("expected duplicate creation to fail, but it succeeded")
		}

		fetched, getErr := urlSvc.Get(ctx, "mycode")
		if getErr != nil {
			t.Fatalf("failed to get URL after duplicate attempt: %v", getErr)
		}

		dataCorrupted := false
		if fetched.Disabled {
			dataCorrupted = true
			t.Log("detected corruption: URL was disabled by duplicate creation attempt")
		}
		if fetched.RawURL != "https://example.com/original" {
			dataCorrupted = true
			t.Logf("detected corruption: raw URL changed from 'https://example.com/original' to '%s'", fetched.RawURL)
		}
		if fetched.MaxVisits != 100 {
			dataCorrupted = true
			t.Logf("detected corruption: max visits changed from 100 to %d", fetched.MaxVisits)
		}

		snapshot := urlStore.RawSnapshot()
		if snapURL, ok := snapshot["mycode"]; ok {
			if snapURL.Disabled {
				dataCorrupted = true
				t.Log("detected corruption: snapshot shows URL is disabled")
			}
			if snapURL.RawURL != "https://example.com/original" {
				dataCorrupted = true
				t.Logf("detected corruption: snapshot raw URL changed to '%s'", snapURL.RawURL)
			}
		}

		redirectResult, redirErr := redirectSvc.HandleRedirect(ctx, &model.RedirectRequest{
			Code:      "mycode",
			Timestamp: time.Now(),
		})
		if redirErr != nil {
			t.Fatalf("redirect failed: %v", redirErr)
		}
		if redirectResult.Status == 410 {
			dataCorrupted = true
			t.Log("detected corruption: redirect returns 410 (gone) for valid URL")
		}
		if redirectResult.Status == 404 {
			dataCorrupted = true
			t.Log("detected corruption: redirect returns 404 for existing URL")
		}
		if redirectResult.RawURL != "https://example.com/original" {
			dataCorrupted = true
			t.Logf("detected corruption: redirect raw URL changed to '%s'", redirectResult.RawURL)
		}

		if dataCorrupted {
			fmt.Println("RED（红灯，缺陷未修复）")
			t.FailNow()
		}
		fmt.Println("GREEN（绿灯，缺陷已修复）")
	})
}
