package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("failed to create URLStore: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("failed to create URLService: %v", err)
	}

	ctx := context.Background()

	_, err = urlSvc.Get(ctx, "nonexistent-code-12345")
	if err == nil {
		t.Fatal("expected error for non-existent code, got nil")
	}

	var appErr *model.AppError
	errors.As(err, &appErr)

	directCheck := false
	if _, ok := err.(*model.AppError); ok {
		directCheck = true
	}

	chainCheck := appErr != nil

	if chainCheck && !directCheck {
		fmt.Println("RED（红灯，缺陷未修复）")
		fmt.Println("缺陷描述：错误传播链断裂。AppError被fmt.Errorf(\"...: %w\", err)包裹后，")
		fmt.Println("handler层的handleError使用直接类型断言(err.(*model.AppError))无法识别，")
		fmt.Println("导致原本应返回404的错误被当作500内部错误处理。")
		fmt.Printf("  - errors.As 能找到 AppError: %v (code=%d)\n", chainCheck, appErr.Code)
		fmt.Printf("  - 直接类型断言能找到: %v\n", directCheck)
		fmt.Printf("  - 错误信息: %s\n", err.Error())
		t.FailNow()
	}

	if chainCheck && directCheck {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
		fmt.Println("修复成功：错误传播链正确处理。AppError无论是否被包裹，")
		fmt.Println("都能通过errors.As或直接类型断言被正确识别和分类。")
	} else if !chainCheck {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
		fmt.Println("修复成功：底层错误不包含AppError，服务层不再不必要地包装错误。")
	}

	if err := urlStore.Close(); err != nil {
		t.Logf("warning: error closing store: %v", err)
	}
}
