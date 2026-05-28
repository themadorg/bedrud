package handlers

import (
	"sync"
	"time"
)

// CooldownCache provides a per-key cooldown mechanism for rate-limiting actions
// like resending verification emails. Thread-safe with lazy cleanup of expired entries.
//
// The cache is in-memory only. For multi-instance deployments, a shared Redis
// implementation would be needed.
// TODO: shared Redis cooldown for multi-instance deployments
type CooldownCache struct {
	mu    sync.Mutex
	items map[string]time.Time
	ttl   time.Duration
}

// NewCooldownCache creates a CooldownCache with the given TTL.
func NewCooldownCache(ttl time.Duration) *CooldownCache {
	return &CooldownCache{
		items: make(map[string]time.Time),
		ttl:   ttl,
	}
}

// Allow returns true if the key is not in cooldown (and starts the cooldown).
// Returns false if the key is still in cooldown.
func (c *CooldownCache) Allow(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.purge()
	if expiry, exists := c.items[key]; exists && time.Now().Before(expiry) {
		return false
	}
	c.items[key] = time.Now().Add(c.ttl)
	return true
}

// Remaining returns the duration until the key is allowed again.
// Returns 0 if the key is not in cooldown.
func (c *CooldownCache) Remaining(key string) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.purge()
	if expiry, exists := c.items[key]; exists {
		if remaining := time.Until(expiry); remaining > 0 {
			return remaining
		}
	}
	return 0
}

// purge removes expired entries from the map.
func (c *CooldownCache) purge() {
	now := time.Now()
	for k, expiry := range c.items {
		if now.After(expiry) {
			delete(c.items, k)
		}
	}
}
