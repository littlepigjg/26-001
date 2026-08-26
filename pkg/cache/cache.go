// Package cache provides a thread-safe in-memory cache with TTL-based expiration.
// It supports automatic cleanup of expired entries and concurrent access.
package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// DefaultTTL is the default time-to-live for cache entries.
const DefaultTTL = 5 * time.Minute

// Entry represents a single cache entry with expiration time.
type Entry struct {
	// Value is the cached value.
	Value interface{}
	// ExpiresAt is the time when this entry expires (zero means no expiration).
	ExpiresAt time.Time
	// CreatedAt is when the entry was created.
	CreatedAt time.Time
}

// IsExpired checks if the entry has expired.
func (e *Entry) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

// Cache is a thread-safe in-memory cache.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	defaultTTL time.Duration
	maxSize    int
	// quit channel for background cleanup goroutine
	quit chan struct{}
	// closed flag
	closed bool
	// activeCleanup tracks number of active cleanup goroutines
	activeCleanup int32
}

// New creates a new Cache with the given default TTL and max size.
func New(defaultTTL time.Duration, maxSize int) *Cache {
	c := &Cache{
		entries:    make(map[string]*Entry),
		defaultTTL: defaultTTL,
		maxSize:    maxSize,
		quit:       make(chan struct{}),
	}
	return c
}

// NewDefault creates a new Cache with default settings.
func NewDefault() *Cache {
	return New(DefaultTTL, 10000)
}

// StartCleanup begins a background goroutine that periodically cleans up expired entries.
// The interval parameter controls how often cleanup runs.
func (c *Cache) StartCleanup(interval time.Duration) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.quit = make(chan struct{})
	currentQuit := c.quit
	c.mu.Unlock()

	atomic.AddInt32(&c.activeCleanup, 1)
	go func() {
		defer atomic.AddInt32(&c.activeCleanup, -1)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.cleanup()
			case <-currentQuit:
				return
			}
		}
	}()
}

// ActiveGoroutines returns the number of active cleanup goroutines.
func (c *Cache) ActiveGoroutines() int32 {
	return atomic.LoadInt32(&c.activeCleanup)
}

// Stop stops the background cleanup goroutine and clears the cache.
func (c *Cache) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.quit)
		c.clearLocked()
	}
}

// StopAndWait stops all background cleanup goroutines and waits for them to finish.
func (c *Cache) StopAndWait() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.quit)
	c.clearLocked()
	c.mu.Unlock()

	for i := 0; i < 100 && atomic.LoadInt32(&c.activeCleanup) > 0; i++ {
		time.Sleep(time.Millisecond * 5)
	}
}

// Set stores a value in the cache with the default TTL.
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL stores a value in the cache with a specific TTL.
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	// Check if we need to evict
	if c.maxSize > 0 && len(c.entries) >= c.maxSize {
		c.evictOne()
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	c.entries[key] = &Entry{
		Value:     value,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
}

// Get retrieves a value from the cache. Returns the value and whether it was found.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if entry.IsExpired() {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}

	return entry.Value, true
}

// GetEntry retrieves a full cache entry (including metadata).
func (c *Cache) GetEntry(key string) (*Entry, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok || entry.IsExpired() {
		return nil, false
	}
	return entry, true
}

// Delete removes a key from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Exists checks if a key exists in the cache (even if expired).
func (c *Cache) Exists(key string) bool {
	c.mu.RLock()
	_, ok := c.entries[key]
	c.mu.RUnlock()
	return ok
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearLocked()
}

// clearLocked clears entries (must be called with write lock held).
func (c *Cache) clearLocked() {
	c.entries = make(map[string]*Entry)
}

// Size returns the number of entries in the cache (including expired ones).
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Keys returns all keys in the cache.
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.entries))
	for k := range c.entries {
		keys = append(keys, k)
	}
	return keys
}

// cleanup removes expired entries from the cache.
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, entry := range c.entries {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(c.entries, key)
		}
	}
}

// evictOne removes one entry when the cache is full.
// It removes the oldest entry (based on creation time).
func (c *Cache) evictOne() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for key, entry := range c.entries {
		if first || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// SetDefaultTTL changes the default TTL for new entries.
func (c *Cache) SetDefaultTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultTTL = ttl
}

// GetDefaultTTL returns the current default TTL.
func (c *Cache) GetDefaultTTL() time.Duration {
	return c.defaultTTL
}
