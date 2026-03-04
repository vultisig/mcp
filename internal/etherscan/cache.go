package etherscan

import (
	"sync"
	"time"
)

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

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

func (c *ttlCache[V]) set(key string, value V) {
	c.mu.Lock()
	c.entries[key] = cacheEntry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}
