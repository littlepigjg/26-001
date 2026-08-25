package main

import (
	"context"
	"fmt"
	"testing"

	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	ctx := context.Background()

	memoryStore := store.NewMemoryStore()

	appSvc := service.NewAppService(memoryStore)
	configSvc := service.NewConfigService(memoryStore, appSvc)
	versionSvc := service.NewVersionService(memoryStore, appSvc, configSvc)

	_, err := appSvc.CreateApp(ctx, "test-app", "Test App", "Test Description", "test-owner")
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	if err := appSvc.AddEnvironment(ctx, "test-app", "production"); err != nil {
		t.Fatalf("failed to add environment: %v", err)
	}

	_, err = configSvc.CreateConfig(ctx, "test-app", "production", "key1", "value1", "test key", "string", "test-user")
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Set fault injector to simulate disk full:
	// the store will accept the write (return nil) but store nil instead of the actual version
	memoryStore.SetFaultInjector(func() error {
		return nil
	})

	// Also set panic guard to return nil (no additional simulated error)
	versionSvc.SetPanicGuard(nil)

	// Test AutoSnapshot which triggers the nil pointer dereference defect
	var panicOccurred bool
	var panicValue interface{}

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicOccurred = true
				panicValue = r
			}
		}()

		_, _, callErr := versionSvc.AutoSnapshot(ctx, "test-app", "production", "test-user")
		if callErr != nil {
			t.Logf("AutoSnapshot returned error: %v", callErr)
		}
	}()

	if panicOccurred {
		fmt.Println("RED（红灯，缺陷未修复）")
		t.Errorf("AutoSnapshot panicked (defect present): %v", panicValue)
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
	}
}
