package config_center_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"config-center/config"
	"config-center/model"
	"config-center/service"
	"config-center/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()
	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore failed: %v", err)
	}

	us.SetPanicGuard(func(code, rawURL string) bool {
		return true
	})

	var wg sync.WaitGroup
	var counter int64
	N := 1000

	errCh := make(chan error, N*4)
	panicCh := make(chan string, N*4)

	// Concurrent Create goroutines - trigger Save() lost-update and map race
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCh <- fmt.Sprintf("panic in create goroutine %d: %v", idx, r)
				}
			}()
			svc, svcErr := service.NewURLService(cfg, us)
			if svcErr != nil {
				errCh <- svcErr
				return
			}
			num := atomic.AddInt64(&counter, 1)
			code := fmt.Sprintf("c-%d-%d", idx, num)
			req := &model.CreateReq{
				RawURL:    "https://example.com/long-url-path",
				CustomCode: code,
				MaxVisits: 50,
			}
			_, createErr := svc.Create(context.Background(), req)
			if createErr != nil {
				errCh <- createErr
			}
		}(i)
	}

	// Concurrent Load goroutines - trigger Load() unprotected initialization race
	for i := 0; i < N/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCh <- fmt.Sprintf("panic in load goroutine: %v", r)
				}
			}()
			_ = us.Load(context.Background())
		}()
	}

	// Concurrent RawSnapshot goroutines - trigger RawSnapshot() unprotected read race
	for i := 0; i < N*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCh <- fmt.Sprintf("panic in snapshot goroutine: %v", r)
				}
			}()
			snap := us.RawSnapshot()
			for k := range snap {
				_ = k
			}
		}()
	}

	wg.Wait()
	close(errCh)
	close(panicCh)

	errCount := 0
	for range errCh {
		errCount++
	}

	panicCount := 0
	for range panicCh {
		panicCount++
	}

	snap := us.RawSnapshot()

	if panicCount > 0 {
		fmt.Printf("RED（红灯，缺陷未修复）- 发生 %d 次并发 panic\n", panicCount)
		t.Fail()
		return
	}

	if errCount > N/10 {
		fmt.Printf("RED（红灯，缺陷未修复）- 大量操作失败 (errors: %d)\n", errCount)
		t.Fail()
		return
	}

	if len(snap) < N/2 {
		fmt.Printf("RED（红灯，缺陷未修复）- 数据不一致，快照条目数不足 (expected >= %d, got %d)\n", N/2, len(snap))
		t.Fail()
		return
	}

	fmt.Printf("GREEN（绿灯，缺陷已修复）- 快照条目数: %d, 错误数: %d, panic数: %d\n", len(snap), errCount, panicCount)
}
