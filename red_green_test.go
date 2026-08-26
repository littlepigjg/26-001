package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"config-center/internal/service"
	"config-center/internal/store"
)

// TestRedGreen tests whether the version diff comparison works correctly.
// RED (fail): When the defect exists, DiffVersions incorrectly returns no changes
// GREEN (pass): When the defect is fixed, DiffVersions correctly detects differences
func TestRedGreen(t *testing.T) {
	ctx := context.Background()

	// Initialize store
	memStore := store.NewMemoryStore()

	// Initialize services
	appSvc := service.NewAppService(memStore)
	configSvc := service.NewConfigService(memStore, appSvc)
	versionSvc := service.NewVersionService(memStore, appSvc, configSvc)
	diffSvc := service.NewDiffService(memStore, appSvc)

	// Create application
	appID := "test-app"
	env := "dev"
	_, err := appSvc.CreateApp(ctx, appID, "Test Application", "Test Description", "test-owner")
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Create initial configuration
	_, err = configSvc.CreateConfig(ctx, appID, env, "key1", "value1", "Test config", "string", "test-user")
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Create first version (v1) with initial config
	_, err = versionSvc.CreateVersion(ctx, appID, env, "test-user", "Initial version")
	if err != nil {
		t.Fatalf("Failed to create version 1: %v", err)
	}

	// Update configuration
	_, err = configSvc.UpdateConfig(ctx, appID, env, "key1", "value2", "Updated config", "test-user")
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Create second version (v2) with updated config
	_, err = versionSvc.CreateVersion(ctx, appID, env, "test-user", "Updated version")
	if err != nil {
		t.Fatalf("Failed to create version 2: %v", err)
	}

	// Compare v1 and v2 - should detect differences
	result, err := diffSvc.DiffVersions(ctx, appID, env, 1, 2)
	if err != nil {
		t.Fatalf("Failed to diff versions: %v", err)
	}

	// The test verdict
	if result.HasChanges {
		fmt.Println("GREEN (绿灯，缺陷已修复)")
		fmt.Printf("Successfully detected %d change(s) between version 1 and version 2\n", len(result.Changes))
		for _, change := range result.Changes {
			fmt.Printf("  - Key: %s, Type: %d, Old: %s, New: %s\n",
				change.Key, change.Type, change.OldValue, change.NewValue)
		}
	} else {
		fmt.Println("RED (红灯，缺陷未修复)")
		fmt.Println("DiffVersions incorrectly returns no changes between versions that should differ")
		fmt.Printf("HasChanges: %v, Changes count: 0 (expected: changes should be detected)\n", result.HasChanges)
		t.Errorf("Expected HasChanges to be true, but got false. The diff between version 1 and version 2 should show changes (key1: value1 -> value2)")
		os.Exit(1)
	}
}
