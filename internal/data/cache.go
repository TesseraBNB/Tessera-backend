package data

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data    []byte
	expires time.Time
}

// ttlCache is a small concurrency-safe time-to-live cache for upstream
// responses. Octant epoch data is effectively immutable once an epoch closes,
// so caching avoids re-fetching the same payloads within a request and across
// concurrent requests.
type ttlCache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]cacheEntry
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{ttl: ttl, m: make(map[string]cacheEntry)}
}

func (c *ttlCache) get(key string) ([]byte, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.data, true
}

func (c *ttlCache) set(key string, data []byte) {
	c.mu.Lock()
	c.m[key] = cacheEntry{data: data, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}
