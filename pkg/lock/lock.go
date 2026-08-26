// Package lock provides distributed lock primitives for coordinating
// concurrent access to shared resources.
package lock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrLockNotAcquired is returned when a lock cannot be acquired.
var ErrLockNotAcquired = errors.New("lock not acquired")

// ErrLockAlreadyHeld is returned when trying to acquire a lock that is already held.
var ErrLockAlreadyHeld = errors.New("lock already held by another holder")

// Lock represents a distributed lock for a specific resource.
type Lock struct {
	mu       sync.Mutex
	resource string
	holder   string
	expires  time.Time
	ttl      time.Duration
	logger   interface{ Infof(format string, args ...interface{}) }
}

// LockManager manages a set of distributed locks.
type LockManager struct {
	mu    sync.Mutex
	locks map[string]*Lock
}

// NewLockManager creates a new LockManager.
func NewLockManager() *LockManager {
	return &LockManager{
		locks: make(map[string]*Lock),
	}
}

// Acquire attempts to acquire a lock on the given resource.
// It returns a context that is cancelled when the lock expires or is released.
func (lm *LockManager) Acquire(resource, holder string, ttl time.Duration) (context.Context, context.CancelFunc, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Check existing lock
	if existing, exists := lm.locks[resource]; exists {
		if time.Now().Before(existing.expires) {
			if existing.holder != holder {
				return nil, nil, ErrLockAlreadyHeld
			}
			// Renew the lock
			existing.expires = time.Now().Add(ttl)
			ctx, cancel := context.WithCancel(context.Background())
			lm.startAutoRenew(existing, ctx, cancel)
			return ctx, cancel, nil
		}
		// Lock expired, remove it
		delete(lm.locks, resource)
	}

	// Create new lock
	lock := &Lock{
		resource: resource,
		holder:   holder,
		expires:  time.Now().Add(ttl),
		ttl:      ttl,
	}

	lm.locks[resource] = lock

	ctx, cancel := context.WithCancel(context.Background())
	lm.startAutoRenew(lock, ctx, cancel)

	return ctx, cancel, nil
}

// Release releases a lock on the given resource.
func (lm *LockManager) Release(resource, holder string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lock, exists := lm.locks[resource]
	if !exists {
		return ErrLockNotAcquired
	}

	if lock.holder != holder {
		return ErrLockAlreadyHeld
	}

	delete(lm.locks, resource)
	return nil
}

// IsLocked checks if a resource is currently locked.
func (lm *LockManager) IsLocked(resource string) (bool, string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lock, exists := lm.locks[resource]
	if !exists || time.Now().After(lock.expires) {
		return false, ""
	}
	return true, lock.holder
}

// startAutoRenew starts a goroutine that automatically renews a lock.
func (lm *LockManager) startAutoRenew(lock *Lock, ctx context.Context, cancel context.CancelFunc) {
	go func() {
		ticker := time.NewTicker(lock.ttl / 3)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lm.mu.Lock()
				if current, exists := lm.locks[lock.resource]; exists && current.holder == lock.holder {
					current.expires = time.Now().Add(current.ttl)
				}
				lm.mu.Unlock()
			}
		}
	}()
}

// WithLock executes a function while holding a lock.
// It automatically acquires the lock, runs the function, and releases the lock.
func (lm *LockManager) WithLock(ctx context.Context, resource, holder string, ttl time.Duration, fn func(ctx context.Context) error) error {
	lockCtx, cancel, err := lm.Acquire(resource, holder, ttl)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() {
		cancel()
		_ = lm.Release(resource, holder)
	}()

	return fn(lockCtx)
}

// WaitForLock attempts to acquire a lock with timeout and retries.
func (lm *LockManager) WaitForLock(ctx context.Context, resource, holder string, ttl, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	deadline := time.Now().Add(timeout)
	for {
		lockCtx, cancel, err := lm.Acquire(resource, holder, ttl)
		if err == nil {
			return lockCtx, cancel, nil
		}

		if time.Now().After(deadline) {
			return nil, nil, fmt.Errorf("timeout waiting for lock on %s: %w", resource, err)
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
