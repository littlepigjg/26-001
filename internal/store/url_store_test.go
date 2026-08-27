package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"config-center/internal/config"
)

func newTestStore(t *testing.T) *URLStore {
	t.Helper()
	s, err := NewURLStore(&config.Config{})
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	return s
}

// Create mirrors internal/service/url_service.go.Create, which calls
// PrepareContext twice per request (once in Create, once in validateAndPrepare).
func create(ctx context.Context, s *URLStore) {
	childCtx, cancel := s.PrepareContext(ctx)
	defer cancel()
	// emulate validateAndPrepare also calling PrepareContext
	innerCtx, innerCancel := s.PrepareContext(childCtx)
	defer innerCancel()
	_ = innerCtx
}

// handleRedirect mirrors internal/service/redirect_service.go.HandleRedirect,
// which calls PrepareContext once per request.
func handleRedirect(ctx context.Context, s *URLStore) {
	childCtx, cancel := s.PrepareContext(ctx)
	defer cancel()
	_ = childCtx
}

func TestCreateLeakReproduction(t *testing.T) {
	s := newTestStore(t)
	parent := context.Background()
	for i := 0; i < 50; i++ {
		create(parent, s)
	}
	// Give the background Done() goroutines a moment to run.
	waitZero(t, s, "after 50 creates")
}

func TestRedirectLeakReproduction(t *testing.T) {
	s := newTestStore(t)
	parent := context.Background()
	for i := 0; i < 50; i++ {
		handleRedirect(parent, s)
	}
	waitZero(t, s, "after 50 redirects")
}

// Concurrent calls should also leave zero active contexts.
func TestConcurrentLeak(t *testing.T) {
	s := newTestStore(t)
	parent := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handleRedirect(parent, s)
			create(parent, s)
		}()
	}
	wg.Wait()
	waitZero(t, s, "after concurrent create+redirect")
}

// If a caller drops the cancel func without calling it, the parent context
// being cancelled (e.g. HTTP request ends) must still free the entry.
func TestParentCancelFreesEntry(t *testing.T) {
	s := newTestStore(t)
	parent, parentCancel := context.WithCancel(context.Background())
	_, _ = s.PrepareContext(parent) // intentionally drop cancel
	if got := s.ActiveContextCount(); got != 1 {
		t.Fatalf("before parent cancel: got %d, want 1", got)
	}
	parentCancel()
	waitZero(t, s, "after parent cancel")
}

// Close must drop every tracked context.
func TestCloseDropsAll(t *testing.T) {
	s := newTestStore(t)
	parent := context.Background()
	for i := 0; i < 20; i++ {
		_, _ = s.PrepareContext(parent) // drop cancels to force Close to clean up
	}
	if got := s.ActiveContextCount(); got != 20 {
		t.Fatalf("before close: got %d, want 20", got)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := s.ActiveContextCount(); got != 0 {
		t.Fatalf("after close: got %d, want 0", got)
	}
}

func waitZero(t *testing.T, s *URLStore, stage string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("%s: active context count never reached 0, got %d", stage, s.ActiveContextCount())
		case <-ticker.C:
			if s.ActiveContextCount() == 0 {
				return
			}
		}
	}
}
