package config_center

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"config-center/config"
	"config-center/model"
	"config-center/service"
	"config-center/store"
)

func TestRedGreen(t *testing.T) {
	tmpDir := t.TempDir()
	invalidDir := filepath.Join(tmpDir, "nonexistent_subdir")

	cfg := config.Default()
	cfg.Storage.URLFilePath(invalidDir + "/urls.json")
	cfg.Storage.LogFilePath(filepath.Join(tmpDir, "access.log"))

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore failed: %v", err)
	}

	ctx := context.Background()
	if err := urlStore.Load(ctx); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("NewURLService failed: %v", err)
	}

	req := &model.CreateReq{
		RawURL:     "https://example.com/very/long/path",
		CustomCode: "testfix",
		MaxVisits:  100,
	}

	_, err = urlSvc.Create(ctx, req)
	if err == nil {
		t.Fatal("Create should have failed with invalid file path, but returned success")
	}

	got, getErr := urlStore.Get("testfix")
	if getErr != nil {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
		fmt.Printf("Save失败后，Get正确返回错误: %v\n", getErr)
		t.Log("GREEN: Store状态一致，Save失败后未产生脏数据")
		return
	}

	if got != nil {
		fmt.Println("RED（红灯，缺陷未修复）")
		fmt.Printf("Save失败但Get仍返回数据: code=%s, rawURL=%s\n", got.Code, got.RawURL)
		fmt.Println("Store存在状态污染：内存中的数据未在持久化失败时回滚")
		t.Error("RED: Store状态不一致 - Save返回错误但数据已写入内存，造成状态污染")
	}

	_ = urlStore.Close()
	_ = os.RemoveAll(tmpDir)
}

func TestRedGreenPanicGuard(t *testing.T) {
	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "urls.json")

	cfg := config.Default()
	cfg.Storage.URLFilePath(validPath)
	cfg.Storage.LogFilePath(filepath.Join(tmpDir, "access.log"))
	cfg.Storage.FlushOnWrite(true)

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore failed: %v", err)
	}

	ctx := context.Background()
	if err := urlStore.Load(ctx); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	urlStore.SetPanicGuard(func(code, rawURL string) bool {
		return code == "guard_test"
	})

	shortURL := &model.ShortURL{
		Code:      "guard_test",
		RawURL:    "https://example.com/guard",
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    false,
		Disabled:  false,
	}

	saveErr := urlStore.Save(shortURL, false)

	if saveErr == nil {
		t.Fatal("Save should have failed due to panic guard")
	}

	_, getErr := urlStore.Get("guard_test")
	if getErr != nil {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
		fmt.Printf("panic guard触发后，Get正确返回错误: %v\n", getErr)
		t.Log("GREEN: Panic guard触发时Store正确回滚了内存数据")
	} else {
		fmt.Println("RED（红灯，缺陷未修复）")
		fmt.Printf("Save失败(panic guard)但Get仍返回数据，造成状态污染\n")
		t.Error("RED: Panic guard触发后，Store未回滚内存中的脏数据")
	}

	_ = urlStore.Close()
	_ = os.RemoveAll(tmpDir)
}
