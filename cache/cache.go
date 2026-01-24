// Package cache provides generic, thread-safe LRU caches with metrics.
package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// Cache is a generic thread-safe LRU cache with built-in metrics.
// It uses Go generics (1.18+) for type safety without interface{} overhead.
type Cache[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]*entry[K, V]
	order    *list.List
	capacity int

	// Metrics (lock-free using atomics)
	hits   atomic.Uint64
	misses atomic.Uint64
	evicts atomic.Uint64
	sets   atomic.Uint64
}

// entry holds a cached value and its position in the LRU list.
type entry[K comparable, V any] struct {
	key     K
	value   V
	element *list.Element
}

// New creates a new Cache with the specified capacity.
// When the cache is full, the least recently used item is evicted.
func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity <= 0 {
		capacity = 100
	}
	return &Cache[K, V]{
		items:    make(map[K]*entry[K, V], capacity),
		order:    list.New(),
		capacity: capacity,
	}
}

// Get retrieves a value from the cache.
// Returns the value and true if found, zero value and false otherwise.
// Accessing an item moves it to the front of the LRU list.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		c.misses.Add(1)
		var zero V
		return zero, false
	}

	c.hits.Add(1)

	// Move to front (most recently used)
	c.mu.Lock()
	c.order.MoveToFront(e.element)
	c.mu.Unlock()

	return e.value, true
}

// Set adds or updates a value in the cache.
// If the cache is at capacity, the least recently used item is evicted.
func (c *Cache[K, V]) Set(key K, value V) {
	c.sets.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry
	if e, ok := c.items[key]; ok {
		e.value = value
		c.order.MoveToFront(e.element)
		return
	}

	// Evict oldest if at capacity
	if len(c.items) >= c.capacity {
		c.evictOldest()
	}

	// Add new entry
	element := c.order.PushFront(key)
	c.items[key] = &entry[K, V]{
		key:     key,
		value:   value,
		element: element,
	}
}

// evictOldest removes the least recently used item.
// Must be called with mu held.
func (c *Cache[K, V]) evictOldest() {
	oldest := c.order.Back()
	if oldest == nil {
		return
	}

	key := oldest.Value.(K)
	delete(c.items, key)
	c.order.Remove(oldest)
	c.evicts.Add(1)
}

// Delete removes an item from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.items[key]; ok {
		delete(c.items, key)
		c.order.Remove(e.element)
	}
}

// Len returns the current number of items in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Clear removes all items from the cache.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[K]*entry[K, V], c.capacity)
	c.order.Init()
}

// Stats holds cache statistics.
type Stats struct {
	Size     int
	Capacity int
	Hits     uint64
	Misses   uint64
	Evicts   uint64
	Sets     uint64
	HitRate  float64
}

// Stats returns cache statistics.
func (c *Cache[K, V]) Stats() Stats {
	c.mu.RLock()
	size := len(c.items)
	c.mu.RUnlock()

	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return Stats{
		Size:     size,
		Capacity: c.capacity,
		Hits:     hits,
		Misses:   misses,
		Evicts:   c.evicts.Load(),
		Sets:     c.sets.Load(),
		HitRate:  hitRate,
	}
}

// GetOrSet returns the existing value for key if present.
// Otherwise, it calls fn to compute the value, stores it, and returns it.
// This is atomic with respect to the cache.
func (c *Cache[K, V]) GetOrSet(key K, fn func() V) V {
	// Fast path: check if already cached
	if v, ok := c.Get(key); ok {
		return v
	}

	// Slow path: compute and cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if e, ok := c.items[key]; ok {
		c.order.MoveToFront(e.element)
		return e.value
	}

	// Compute value
	value := fn()

	// Evict if needed
	if len(c.items) >= c.capacity {
		c.evictOldest()
	}

	// Store
	element := c.order.PushFront(key)
	c.items[key] = &entry[K, V]{
		key:     key,
		value:   value,
		element: element,
	}
	c.sets.Add(1)

	return value
}

// Keys returns all keys in the cache (in no particular order).
func (c *Cache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]K, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

// Range calls fn for each item in the cache.
// If fn returns false, iteration stops.
func (c *Cache[K, V]) Range(fn func(key K, value V) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for k, e := range c.items {
		if !fn(k, e.value) {
			break
		}
	}
}
