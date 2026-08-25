package configcenter_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"config-center/config"
	"config-center/model"
	"config-center/service"
	"config-center/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()

	s, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("failed to create URL store: %v", err)
	}
	defer s.Close()

	svc, err := service.NewURLService(cfg, s)
	if err != nil {
		t.Fatalf("failed to create URL service: %v", err)
	}

	s.SetPanicGuard(func(code, rawURL string) bool {
		return false
	})

	testCode := "abcde"

	req := &model.CreateReq{
		RawURL:     "https://example.com/test",
		CustomCode: testCode,
		MaxVisits:  100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = svc.Create(ctx, req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	u, err := s.Get(testCode)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if u.Processed {
		fmt.Println("RED（红灯，缺陷未修复）：context 取消后子任务仍继续执行，URL 被标记为已处理")
		t.FailNow()
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）：context 取消后子任务正确终止，URL 未被处理")
	}
}
