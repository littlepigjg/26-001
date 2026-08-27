package config_center_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"config-center/config"
	"config-center/model"
	"config-center/service"
	"config-center/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("failed to create URL store: %v", err)
	}

	if err := urlStore.Load(context.Background()); err != nil {
		t.Fatalf("failed to load URL store: %v", err)
	}

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("failed to create access log store: %v", err)
	}

	if err := logStore.Open(context.Background()); err != nil {
		t.Fatalf("failed to open access log store: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("failed to create URL service: %v", err)
	}

	for i := 0; i < 10; i++ {
		req := &model.CreateReq{
			RawURL:     fmt.Sprintf("https://example.com/test/%d", i),
			CustomCode: fmt.Sprintf("tst%d", i),
			MaxVisits:  100,
		}
		_, createErr := urlSvc.Create(context.Background(), req)
		if createErr != nil {
			t.Fatalf("failed to seed URL %d: %v", i, createErr)
		}
	}

	binPath := filepath.Join(os.TempDir(), "concurrent_test_binary")
	srcCode := `package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/config"
	"config-center/model"
	"config-center/service"
	"config-center/store"
)

func main() {
	cfg := config.Default()

	urlStore, _ := store.NewURLStore(cfg)
	urlStore.Load(context.Background())

	logStore, _ := store.NewAccessLogStore(cfg)
	logStore.Open(context.Background())

	urlSvc, _ := service.NewURLService(cfg, urlStore)
	redirectSvc, _ := service.NewRedirectService(urlStore, logStore)

	for i := 0; i < 50; i++ {
		req := &model.CreateReq{
			RawURL:     fmt.Sprintf("https://example.com/seed/%d", i),
			CustomCode: fmt.Sprintf("seed%d", i),
			MaxVisits:  1000,
		}
		urlSvc.Create(context.Background(), req)
	}

	var wg sync.WaitGroup
	rounds := 60

	for i := 0; i < rounds; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := &model.CreateReq{
				RawURL:     fmt.Sprintf("https://example.com/conc/%d", idx),
				CustomCode: fmt.Sprintf("cnc%d", idx),
				MaxVisits:  500,
			}
			urlSvc.Create(context.Background(), req)
		}(i)
	}

	for i := 0; i < rounds; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			urlStore.RawSnapshot()
		}()
	}

	for i := 0; i < rounds/2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			short := &model.ShortURL{
				Code:      fmt.Sprintf("wrt%d", idx),
				RawURL:    fmt.Sprintf("https://example.com/write/%d", idx),
				CreatedAt: time.Now(),
				Visits:    0,
				Custom:    false,
				Disabled:  false,
			}
			urlStore.Save(short, false)
		}(i)
	}

	for i := 0; i < rounds/3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			urlStore.Get(fmt.Sprintf("seed%d", idx%50))
		}(i)
	}

	for i := 0; i < rounds/3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			redirectReq := &service.RedirectRequest{
				Code:      fmt.Sprintf("seed%d", idx%50),
				Timestamp: time.Now(),
			}
			redirectSvc.HandleRedirect(context.Background(), redirectReq)
		}(i)
	}

	wg.Wait()
}
`

	srcFile := filepath.Join(os.TempDir(), "concurrent_test_main.go")
	if err := os.WriteFile(srcFile, []byte(srcCode), 0644); err != nil {
		t.Fatalf("failed to write temp source: %v", err)
	}
	defer os.Remove(srcFile)

	buildCmd := exec.Command("go", "build", "-o", binPath, srcFile)
	buildCmd.Dir = "/home/admin/code/26/001/25-001-1"
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build subprocess: %v\n%s", err, buildOutput)
	}
	defer os.Remove(binPath)

	runCmd := exec.Command(binPath)
	runCmd.Dir = "/home/admin/code/26/001/25-001-1"
	runOutput, err := runCmd.CombinedOutput()

	if err != nil {
		t.Log("RED（红灯，缺陷未修复）")
		t.Logf("subprocess error: %v\noutput: %s", err, runOutput)
		t.FailNow()
	}

	t.Log("GREEN（绿灯，缺陷已修复）")
}
