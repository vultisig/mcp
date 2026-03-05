package cache

import (
	"sync"
	"time"
)

// Entry holds a value and its expiry time.
type Entry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTL is a simple in-memory cache with per-entry TTL.
// Expired entries are evicted on read.
type TTL[V any] struct {
	mu      sync.RWMutex
	entries map[string]Entry[V]
	ttl     time.Duration
}

// NewTTL creates a TTL cache with the given entry lifetime.
func NewTTL[V any](ttl time.Duration) *TTL[V] {
	return &TTL[V]{
		entries: make(map[string]Entry[V]),
		ttl:     ttl,
	}
}

// Get returns the cached value and true if it exists and hasn't expired.
// Expired entries are evicted on access.
func (c *TTL[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	now := time.Now()
	if now.After(e.expiresAt) {
		c.mu.Lock()
		if cur, exists := c.entries[key]; exists && now.After(cur.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores a value with the cache's TTL.
func (c *TTL[V]) Set(key string, value V) {
	c.mu.Lock()
	c.entries[key] = Entry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}
