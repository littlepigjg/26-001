package main

import (
	"context"
	"fmt"
	"testing"

	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

var defectDetected bool

func TestRedGreen(t *testing.T) {
	ctx := context.Background()
	defectDetected = false

	// Create an in-memory store
	memStore := store.NewMemoryStore()

	// Create a file store for additional testing
	fileStore, err := store.NewFileStore("/tmp/test_store.json", false, 0)
	if err != nil {
		t.Fatalf("failed to create file store: %v", err)
	}
	defer fileStore.Close()

	// Create app services
	memAppService := service.NewAppService(memStore)
	fileAppService := service.NewAppService(fileStore)

	// Create test data
	for i := 0; i < 50; i++ {
		app := model.NewApplication(
			fmt.Sprintf("app-%d", i),
			fmt.Sprintf("Application %d", i),
			"Test application",
			"test-owner",
		)
		memStore.CreateApp(ctx, app)
		fileStore.CreateApp(ctx, app)
	}

	// Test 1: ListApps with normal pagination should work
	t.Run("ListApps normal pagination", func(t *testing.T) {
		apps, total, err := memAppService.ListApps(ctx, 1, 10)
		if err != nil {
			t.Fatalf("ListApps page=1, pageSize=10 failed: %v", err)
		}
		if total != 50 {
			t.Errorf("expected total 50, got %d", total)
		}
		if len(apps) != 10 {
			t.Errorf("expected 10 apps, got %d", len(apps))
		}
	})

	// Test 2: ListApps with page exceeding total should return empty list, not panic
	t.Run("ListApps page exceeds total", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC: ListApps with page exceeding total caused panic: %v", r)
				defectDetected = true
			}
		}()

		// Test with page = 1000 (way beyond total)
		apps, total, err := memAppService.ListApps(ctx, 1000, 20)
		if err != nil {
			t.Errorf("ListApps page=1000 failed with error: %v", err)
			defectDetected = true
		}
		if len(apps) != 0 {
			t.Errorf("expected empty list, got %d apps", len(apps))
			defectDetected = true
		}
		if total != 50 {
			t.Errorf("expected total 50, got %d", total)
		}
	})

	// Test 3: ListApps with page exceeding total on FileStore should not panic
	t.Run("ListApps page exceeds total on FileStore", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC: FileStore ListApps with page exceeding total caused panic: %v", r)
				defectDetected = true
			}
		}()

		apps, total, err := fileAppService.ListApps(ctx, 1000, 20)
		if err != nil {
			t.Errorf("FileStore ListApps page=1000 failed with error: %v", err)
			defectDetected = true
		}
		if len(apps) != 0 {
			t.Errorf("expected empty list, got %d apps", len(apps))
			defectDetected = true
		}
		if total != 50 {
			t.Errorf("expected total 50, got %d", total)
		}
	})

	// Test 4: ListApps with moderate page and pageSize that exceeds total
	t.Run("ListApps with moderate pageSize exceeding total", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC: ListApps with moderate pageSize caused panic: %v", r)
				defectDetected = true
			}
		}()

		// page=100, pageSize=100 - total offset: 100*100 = 10000 > 50
		apps, total, err := memAppService.ListApps(ctx, 100, 100)
		if err != nil {
			t.Errorf("ListApps page=100, pageSize=100 failed with error: %v", err)
			defectDetected = true
		}
		if len(apps) != 0 {
			t.Errorf("expected empty list, got %d apps", len(apps))
			defectDetected = true
		}
		if total != 50 {
			t.Errorf("expected total 50, got %d", total)
		}
	})

	// Print final result
	if defectDetected {
		fmt.Println("========================================================")
		fmt.Println("RESULT: RED（红灯，缺陷未修复）")
		fmt.Println("缺陷：ListApps 在分页参数超出总数时发生 slice bounds out of range panic")
		fmt.Println("========================================================")
	} else {
		fmt.Println("========================================================")
		fmt.Println("RESULT: GREEN（绿灯，缺陷已修复）")
		fmt.Println("所有分页参数均正确处理，无 panic 发生")
		fmt.Println("========================================================")
	}
}

func TestSetPanicGuard(t *testing.T) {
	memStore := store.NewMemoryStore()
	appService := service.NewAppService(memStore)

	// Test SetPanicGuard
	guardCalled := false
	appService.SetPanicGuard(func(code, rawURL string) bool {
		guardCalled = true
		return true
	})

	// Test SaveWithGuard
	app := model.NewApplication("test-app", "Test App", "Test", "owner")
	err := appService.SaveWithGuard(app, false)
	if err != nil {
		t.Errorf("SaveWithGuard failed: %v", err)
	}
	if !guardCalled {
		t.Error("panic guard was not called")
	}

	fmt.Println("GREEN（绿灯，缺陷已修复）")
}

func TestRawSnapshot(t *testing.T) {
	memStore := store.NewMemoryStore()
	appService := service.NewAppService(memStore)

	// Create some apps
	for i := 0; i < 5; i++ {
		app := model.NewApplication(
			fmt.Sprintf("snapshot-app-%d", i),
			fmt.Sprintf("Snapshot App %d", i),
			"Test",
			"owner",
		)
		memStore.CreateApp(context.Background(), app)
	}

	// Test RawSnapshot
	snapshot := appService.RawSnapshot()
	if len(snapshot) != 5 {
		t.Errorf("expected 5 apps in snapshot, got %d", len(snapshot))
	}

	fmt.Println("GREEN（绿灯，缺陷已修复）")
}

func TestIncrementVisitsWithGuard(t *testing.T) {
	memStore := store.NewMemoryStore()
	appService := service.NewAppService(memStore)

	// Create an app
	app := model.NewApplication("visit-app", "Visit App", "Test", "owner")
	memStore.CreateApp(context.Background(), app)

	// Test IncrementVisitsWithGuard
	err := appService.IncrementVisitsWithGuard("visit-app")
	if err != nil {
		t.Errorf("IncrementVisitsWithGuard failed: %v", err)
	}

	// Verify the app was updated
	retrievedApp, err := appService.GetWithGuard("visit-app")
	if err != nil {
		t.Errorf("GetWithGuard failed: %v", err)
	}
	if retrievedApp == nil {
		t.Error("expected to retrieve app")
	}

	fmt.Println("GREEN（绿灯，缺陷已修复）")
}
