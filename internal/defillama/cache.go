package defillama

import (
	"sync"
	"time"
)

// cacheEntry holds a value and its expiry time.
type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// ttlCache is a simple in-memory cache with per-entry TTL.
type ttlCache[V any] struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry[V]
	ttl     time.Duration
}

func newTTLCache[V any](ttl time.Duration) *ttlCache[V] {
	return &ttlCache[V]{
		entries: make(map[string]cacheEntry[V]),
		ttl:     ttl,
	}
}

// get returns the cached value and true if it exists and hasn't expired.
func (c *ttlCache[V]) get(key string) (V, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// set stores a value with the cache's TTL.
func (c *ttlCache[V]) set(key string, value V) {
	c.mu.Lock()
	c.entries[key] = cacheEntry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}
