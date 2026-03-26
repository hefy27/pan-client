package pan

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// cacheEntry stores a cached value with its expiration time.
type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// Cache is a thread-safe in-memory cache with TTL and singleflight protection.
type Cache struct {
	data       sync.Map
	group      singleflight.Group
	defaultTTL time.Duration
	stopCh     chan struct{}
	stopped    sync.Once
}

// NewCache creates a new Cache with the given default TTL.
// A background goroutine periodically cleans up expired entries.
func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		defaultTTL: ttl,
		stopCh:     make(chan struct{}),
	}
	go c.cleanupLoop()
	return c
}

// Get retrieves a value from the cache. Returns (nil, false) if not found or expired.
func (c *Cache) Get(key string) (interface{}, bool) {
	raw, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}
	entry := raw.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.data.Delete(key)
		return nil, false
	}
	return entry.value, true
}

// GetOrLoad retrieves a value from the cache, or calls loader to populate it.
// Uses singleflight to prevent concurrent duplicate loads for the same key.
func (c *Cache) GetOrLoad(key string, loader func() (interface{}, error)) (interface{}, error) {
	if val, ok := c.Get(key); ok {
		return val, nil
	}
	val, err, _ := c.group.Do(key, func() (interface{}, error) {
		// Double-check after acquiring singleflight
		if val, ok := c.Get(key); ok {
			return val, nil
		}
		result, err := loader()
		if err != nil {
			return nil, err
		}
		c.Set(key, result)
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Set stores a value in the cache with the default TTL.
func (c *Cache) Set(key string, value interface{}) {
	c.data.Store(key, &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.defaultTTL),
	})
}

// SetWithTTL stores a value in the cache with a custom TTL.
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.data.Store(key, &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	})
}

// Del removes a key from the cache.
func (c *Cache) Del(key string) {
	c.data.Delete(key)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.data.Range(func(key, _ interface{}) bool {
		c.data.Delete(key)
		return true
	})
}

// Stop stops the background cleanup goroutine.
func (c *Cache) Stop() {
	c.stopped.Do(func() {
		close(c.stopCh)
	})
}

// cleanupLoop periodically removes expired entries.
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.data.Range(func(key, raw interface{}) bool {
				entry := raw.(*cacheEntry)
				if now.After(entry.expiresAt) {
					c.data.Delete(key)
				}
				return true
			})
		case <-c.stopCh:
			return
		}
	}
}
