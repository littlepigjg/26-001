package config_center

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.URLFilePath("/tmp/test_red_green.json")
	cfg.Storage.SyncInterval(50 * time.Millisecond)
	cfg.Storage.FlushOnWrite(true)

	defer os.Remove("/tmp/test_red_green.json")
	defer os.Remove("/tmp/test_red_green.json.tmp")

	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create URLStore: %v", err)
	}

	us.SetPanicGuard(func(code, rawURL string) bool {
		return true
	})

	ctx := context.Background()
	svc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("Failed to create URLService: %v", err)
	}

	req := &model.CreateReq{
		RawURL:     "https://example.com/test",
		CustomCode: "abc12345",
		MaxVisits:  100,
	}

	_, err = svc.Create(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("RED（红灯，缺陷未修复）")
			t.Errorf("缺陷检测：%v", r)
		}
	}()

	err = us.Close()
	if err != nil {
		t.Logf("第一次Close返回错误: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	err = us.Close()
	if err != nil {
		t.Logf("第二次Close返回错误: %v", err)
	}

	fmt.Println("GREEN（绿灯，缺陷已修复）")
}