// Command verify_defect tests whether the concurrency defect in the short URL
// service exists. It spawns concurrent goroutines that perform simultaneous
// reads and writes on the shared map without proper locking, which should
// trigger a fatal "concurrent map iteration and map write" error if the
// defect is present.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"config-center/config"
	"config-center/model"
	"config-center/service"
	"config-center/store"
)

func main() {
	cfg := config.Default()

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: create URLStore: %v\n", err)
		os.Exit(1)
	}

	if err := urlStore.Load(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: load URLStore: %v\n", err)
		os.Exit(1)
	}

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: create AccessLogStore: %v\n", err)
		os.Exit(1)
	}

	if err := logStore.Open(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: open AccessLogStore: %v\n", err)
		os.Exit(1)
	}

	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: create URLService: %v\n", err)
		os.Exit(1)
	}

	redirectSvc, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: create RedirectService: %v\n", err)
		os.Exit(1)
	}

	// Seed initial data
	for i := 0; i < 50; i++ {
		req := &model.CreateReq{
			RawURL:     fmt.Sprintf("https://example.com/seed/%d", i),
			CustomCode: fmt.Sprintf("seed%d", i),
			MaxVisits:  1000,
		}
		if _, err := urlSvc.Create(context.Background(), req); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: seed %d: %v\n", i, err)
			os.Exit(1)
		}
	}

	// Trigger concurrent operations
	var wg sync.WaitGroup
	rounds := 60

	// Concurrent Create goroutines (each calls RawSnapshot + Save internally)
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

	// Concurrent RawSnapshot goroutines (read without lock)
	for i := 0; i < rounds; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			urlStore.RawSnapshot()
		}()
	}

	// Concurrent Save goroutines
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

	// Concurrent Get goroutines
	for i := 0; i < rounds/3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			urlStore.Get(fmt.Sprintf("seed%d", idx%50))
		}(i)
	}

	// Concurrent HandleRedirect goroutines (calls Get + IncrementVisits)
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

	fmt.Println("GREEN (no panic, defect not triggered)")
	os.Exit(0)
}