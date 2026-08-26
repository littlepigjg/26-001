package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"

	cfgpkg "config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
)

// TestRedGreen is the sole acceptance test for this project. It constructs a
// short-URL store, performs concurrent create / redirect / snapshot cycles,
// and verifies that the system never panics under load.
//
// When a defect is present the underlying map is iterated while another
// goroutine is writing to it, which causes a runtime fatal error of the
// form "concurrent map read and map write" or
// "concurrent map iteration and map write". The fatal error is detected
// through the spawned driver process and the test reports RED; otherwise
// it reports GREEN.
func TestRedGreen(t *testing.T) {
	// The driver is the same test binary invoked recursively with a
	// special environment variable. When RED, the driver process exits
	// with a non-zero status because a runtime map fatal error kills it.
	if os.Getenv("CG2_DRIVER") == "1" {
		runConcurrentWorkload(t)
		return
	}

	// Spawn ourselves as a driver subprocess.
	cmd := exec.Command(os.Args[0], "-test.run=^TestRedGreen$",
		"-test.v", "-test.count=1")
	cmd.Env = append(os.Environ(), "CG2_DRIVER=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		// The driver exited non-zero. This can happen because of a
		// runtime fatal error (concurrent map access) or because the
		// driver explicitly called t.Fatalf. Both cases mean RED.
		fmt.Println("RED (红灯，缺陷未修复)")
		t.Fatalf("RED (红灯，缺陷未修复): driver exited: %v", err)
	}
	fmt.Println("GREEN (绿灯，缺陷已修复)")
}

// runConcurrentWorkload builds the store, seeds it, and then runs the
// concurrent create / redirect / snapshot workload. When the underlying
// map is shared unsafely the runtime will terminate the process with
// "fatal error: concurrent map iteration and map write". When the
// implementation is correct the workload completes normally and the
// process exits with status 0.
func runConcurrentWorkload(t *testing.T) {
	const nWorkers = 10
	const nItemsPerWorker = 40

	baseCfg := cfgpkg.Default()
	baseCfg.Storage.URLFilePath("/tmp/short-urls.json")
	baseCfg.Storage.LogFilePath("/tmp/access-logs.json")
	baseCfg.Storage.SyncInterval(50 * time.Millisecond)
	baseCfg.Storage.FlushOnWrite(true)

	us, err := store.NewURLStore(baseCfg)
	if err != nil {
		t.Fatalf("NewURLStore returned error: %v", err)
	}
	if err := us.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	defer us.Close()

	ls, err := store.NewAccessLogStore(baseCfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore returned error: %v", err)
	}
	if err := ls.Open(context.Background()); err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer ls.Close()

	us.SetPanicGuard(func(code, rawURL string) bool {
		return true
	})

	urlSvc, err := service.NewURLService(baseCfg, us)
	if err != nil {
		t.Fatalf("NewURLService returned error: %v", err)
	}

	redirectSvc, err := service.NewRedirectService(us, ls)
	if err != nil {
		t.Fatalf("NewRedirectService returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seed := make([]*model.ShortURL, 0, nItemsPerWorker*nWorkers)
	for i := 0; i < nItemsPerWorker*nWorkers; i++ {
		req := &model.CreateReq{
			RawURL:    fmt.Sprintf("https://example.com/%d", i),
			CustomCode: "",
			MaxVisits:  0,
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("seed CreateReq invalid: %v", err)
		}
		su, err := urlSvc.Create(ctx, req)
		if err != nil {
			t.Fatalf("seed Create failed: %v", err)
		}
		seed = append(seed, su)
	}

	snap := us.RawSnapshot()
	if len(snap) != len(seed) {
		t.Fatalf("seed snapshot size mismatch: got %d want %d", len(snap), len(seed))
	}

	var wg sync.WaitGroup
	errCh := make(chan error, nWorkers)
	panicCh := make(chan interface{}, nWorkers)

	// Writer workers: continuously create + overwrite short URLs.
	for w := 0; w < nWorkers/2; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()
			for i := 0; i < nItemsPerWorker; i++ {
				req := &model.CreateReq{
					RawURL:    fmt.Sprintf("https://example.com/w%d/%d", id, i),
					CustomCode: "",
					MaxVisits:  0,
				}
				su, err := urlSvc.Create(ctx, req)
				if err != nil {
					errCh <- err
					return
				}
				if err := us.Save(su, true); err != nil {
					errCh <- err
					return
				}
			}
		}(w)
	}

	// Reader / snapshot workers: continuously iterate the snapshot map.
	for w := 0; w < nWorkers/2; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()
			for i := 0; i < nItemsPerWorker; i++ {
				// Exercise the live iterator returned by the store. When
				// the underlying implementation shares the live map with
				// the writer goroutines this will race with Save and
				// trigger a "concurrent map read and map write" panic.
				liveMap := us.RawSnapshotIterator()
				for k := range liveMap {
					_ = k
				}
				if i%5 == 0 {
					su, err := us.Get(fmt.Sprintf("seed-%d", id))
					if err == nil {
						_ = su
					}
					_, _ = redirectSvc.CheckConsistency()
				}
				if len(seed) > 0 {
					_, _ = redirectSvc.HandleRedirect(ctx, &service.RedirectRequest{
						Code:      seed[i%len(seed)].Code,
						Timestamp: time.Now(),
					})
				}
			}
		}(w)
	}

	// Busy-wait a few cycles so that the runtime has sufficient
	// opportunity to observe concurrent map access before we return.
	for spin := 0; spin < 32; spin++ {
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	wg.Wait()
	close(errCh)
	close(panicCh)

	for p := range panicCh {
		t.Fatalf("RED (红灯，缺陷未修复): recovered panic %v", p)
	}

	for e := range errCh {
		if e != nil {
			t.Fatalf("RED (红灯，缺陷未修复): unexpected error %v", e)
		}
	}

	if final := us.RawSnapshot(); len(final) == 0 {
		t.Fatal("RED (红灯，缺陷未修复): snapshot unexpectedly empty")
	}
}
