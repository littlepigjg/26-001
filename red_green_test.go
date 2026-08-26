package configcenter_test

import (
	"context"
	"fmt"
	"testing"

	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	ctx := context.Background()

	ms := store.NewMemoryStore()
	appSvc := service.NewAppService(ms)

	for i := 0; i < 3; i++ {
		app := model.NewApplication(
			fmt.Sprintf("app-%d", i),
			fmt.Sprintf("App %d", i),
			"test application",
			"owner",
		)
		if err := ms.CreateApp(ctx, app); err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
	}

	red := false
	defer func() {
		if r := recover(); r != nil {
			red = true
			fmt.Println("RED（红灯，缺陷未修复）")
			t.Errorf("panic occurred: %v", r)
		} else {
			fmt.Println("GREEN（绿灯，缺陷已修复）")
		}
	}()

	_, _, err := appSvc.ListApps(ctx, 1000, 20)
	if err != nil {
		red = true
		fmt.Println("RED（红灯，缺陷未修复）")
		t.Errorf("error occurred: %v", err)
		return
	}

	if !red {
		// Additional check: also test ListVersions with extreme page
		versionSvc := service.NewVersionService(ms, appSvc, nil)
		_, _, err2 := versionSvc.ListVersions(ctx, "app-0", "dev", 1000, 20)
		if err2 != nil {
			red = true
			fmt.Println("RED（红灯，缺陷未修复）")
			t.Errorf("error occurred: %v", err2)
			return
		}
	}
}
