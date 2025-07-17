package floodsub

import (
	"context"
	"sync"
	"time"
)

// sweepInterval controls how often expired entries are cleaned up
var sweepInterval = 1 * time.Minute

// TimeCache provides a time-based cache with automatic expiration
type TimeCache struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mutex      sync.Mutex           // Protects entries map
	entries    map[string]time.Time // Maps keys to expiration times
	timeToLive time.Duration        // How long entries remain valid
}

// NewTimeCache creates a new time-based cache with the given TTL
func NewTimeCache(timeToLive time.Duration) *TimeCache {
	ctx, cancel := context.WithCancel(context.Background())

	cache := &TimeCache{
		ctx:    ctx,
		cancel: cancel,

		entries:    make(map[string]time.Time),
		timeToLive: timeToLive,
	}

	cache.wg.Add(1)
	go cache.background()

	return cache
}

// Has checks if a key exists in the cache
func (cache *TimeCache) Has(key string) bool {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	_, exists := cache.entries[key]
	return exists
}

// Add inserts a key into the cache with TTL-based expiration
func (cache *TimeCache) Add(key string) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if _, exists := cache.entries[key]; !exists {
		cache.entries[key] = time.Now().Add(cache.timeToLive)
	}
}

// background runs the cleanup routine in a separate goroutine
func (cache *TimeCache) background() {
	defer cache.wg.Done()

	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			cache.sweep(now)

		case <-cache.ctx.Done():
			return
		}
	}
}

// sweep removes expired entries from the cache
func (cache *TimeCache) sweep(now time.Time) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	for key, expirationTime := range cache.entries {
		if expirationTime.Before(now) {
			delete(cache.entries, key)
		}
	}
}

// Close shuts down the cache and stops background cleanup
func (cache *TimeCache) Close() error {
	cache.cancel()
	cache.wg.Wait()
	return nil
}
