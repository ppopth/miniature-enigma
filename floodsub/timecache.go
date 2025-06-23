package floodsub

import (
	"context"
	"sync"
	"time"
)

var sweepInterval = 1 * time.Minute

type TimeCache struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	lk  sync.Mutex
	m   map[string]time.Time
	ttl time.Duration
}

func NewTimeCache(ttl time.Duration) *TimeCache {
	ctx, cancel := context.WithCancel(context.Background())

	tc := &TimeCache{
		ctx:    ctx,
		cancel: cancel,

		m:   make(map[string]time.Time),
		ttl: ttl,
	}

	tc.wg.Add(1)
	go tc.background()

	return tc
}

func (tc *TimeCache) Has(key string) bool {
	tc.lk.Lock()
	defer tc.lk.Unlock()

	_, ok := tc.m[key]
	return ok
}

func (tc *TimeCache) Add(key string) {
	tc.lk.Lock()
	defer tc.lk.Unlock()

	if _, ok := tc.m[key]; !ok {
		tc.m[key] = time.Now().Add(tc.ttl)
	}
}

func (tc *TimeCache) background() {
	defer tc.wg.Done()

	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			tc.sweep(now)

		case <-tc.ctx.Done():
			return
		}
	}
}

func (tc *TimeCache) sweep(now time.Time) {
	tc.lk.Lock()
	defer tc.lk.Unlock()

	for k, expiry := range tc.m {
		if expiry.Before(now) {
			delete(tc.m, k)
		}
	}
}

func (tc *TimeCache) Close() error {
	tc.cancel()
	tc.wg.Wait()
	return nil
}
