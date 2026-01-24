package terminology

import (
	"hash/fnv"
	"sync"
	"time"

	"github.com/gofhir/validator/service"
)

const (
	// DefaultShardCount is the default number of cache shards.
	// Use a power of 2 for efficient modulo operation.
	DefaultShardCount = 64

	// DefaultCacheTTL is the default time-to-live for cache entries.
	DefaultCacheTTL = 15 * time.Minute
)

// ShardedCache provides a thread-safe, sharded cache for terminology lookups.
// It uses multiple shards to reduce lock contention in concurrent scenarios.
type ShardedCache struct {
	shards    []*cacheShard
	shardMask uint32
	ttl       time.Duration
}

// cacheShard represents a single shard of the cache.
type cacheShard struct {
	mu         sync.RWMutex
	validations map[string]*cachedValidation
	expansions  map[string]*cachedExpansion
}

// cachedValidation holds a cached validation result with expiration.
type cachedValidation struct {
	result    *service.ValidateCodeResult
	expiresAt time.Time
}

// cachedExpansion holds a cached ValueSet expansion with expiration.
type cachedExpansion struct {
	expansion *service.ValueSetExpansion
	expiresAt time.Time
}

// CacheConfig holds configuration options for the cache.
type CacheConfig struct {
	// ShardCount is the number of shards. Must be a power of 2.
	ShardCount int

	// TTL is the time-to-live for cache entries.
	TTL time.Duration
}

// DefaultCacheConfig returns default cache configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		ShardCount: DefaultShardCount,
		TTL:        DefaultCacheTTL,
	}
}

// NewShardedCache creates a new sharded cache with the given configuration.
func NewShardedCache(config CacheConfig) *ShardedCache {
	// Ensure shard count is a power of 2
	shardCount := config.ShardCount
	if shardCount <= 0 {
		shardCount = DefaultShardCount
	}
	// Round up to nearest power of 2
	shardCount = nextPowerOf2(shardCount)

	ttl := config.TTL
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}

	shards := make([]*cacheShard, shardCount)
	for i := range shards {
		shards[i] = &cacheShard{
			validations: make(map[string]*cachedValidation),
			expansions:  make(map[string]*cachedExpansion),
		}
	}

	return &ShardedCache{
		shards:    shards,
		shardMask: uint32(shardCount - 1),
		ttl:       ttl,
	}
}

// getShard returns the shard for the given key.
func (c *ShardedCache) getShard(key string) *cacheShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.shards[h.Sum32()&c.shardMask]
}

// GetValidation retrieves a cached validation result.
func (c *ShardedCache) GetValidation(key string) (*service.ValidateCodeResult, bool) {
	shard := c.getShard(key)
	shard.mu.RLock()
	cached, ok := shard.validations[key]
	shard.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(cached.expiresAt) {
		// Expired - remove asynchronously
		go func() {
			shard.mu.Lock()
			delete(shard.validations, key)
			shard.mu.Unlock()
		}()
		return nil, false
	}

	return cached.result, true
}

// SetValidation stores a validation result in the cache.
func (c *ShardedCache) SetValidation(key string, result *service.ValidateCodeResult) {
	shard := c.getShard(key)
	shard.mu.Lock()
	shard.validations[key] = &cachedValidation{
		result:    result,
		expiresAt: time.Now().Add(c.ttl),
	}
	shard.mu.Unlock()
}

// GetExpansion retrieves a cached ValueSet expansion.
func (c *ShardedCache) GetExpansion(url string) (*service.ValueSetExpansion, bool) {
	shard := c.getShard(url)
	shard.mu.RLock()
	cached, ok := shard.expansions[url]
	shard.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(cached.expiresAt) {
		go func() {
			shard.mu.Lock()
			delete(shard.expansions, url)
			shard.mu.Unlock()
		}()
		return nil, false
	}

	return cached.expansion, true
}

// SetExpansion stores a ValueSet expansion in the cache.
func (c *ShardedCache) SetExpansion(url string, expansion *service.ValueSetExpansion) {
	shard := c.getShard(url)
	shard.mu.Lock()
	shard.expansions[url] = &cachedExpansion{
		expansion: expansion,
		expiresAt: time.Now().Add(c.ttl),
	}
	shard.mu.Unlock()
}

// Clear removes all entries from the cache.
func (c *ShardedCache) Clear() {
	for _, shard := range c.shards {
		shard.mu.Lock()
		shard.validations = make(map[string]*cachedValidation)
		shard.expansions = make(map[string]*cachedExpansion)
		shard.mu.Unlock()
	}
}

// Cleanup removes expired entries from the cache.
func (c *ShardedCache) Cleanup() {
	now := time.Now()
	for _, shard := range c.shards {
		shard.mu.Lock()
		for key, cached := range shard.validations {
			if now.After(cached.expiresAt) {
				delete(shard.validations, key)
			}
		}
		for url, cached := range shard.expansions {
			if now.After(cached.expiresAt) {
				delete(shard.expansions, url)
			}
		}
		shard.mu.Unlock()
	}
}

// Stats returns cache statistics.
type CacheStats struct {
	Validations int
	Expansions  int
	Shards      int
}

// Stats returns current cache statistics.
func (c *ShardedCache) Stats() CacheStats {
	var validations, expansions int
	for _, shard := range c.shards {
		shard.mu.RLock()
		validations += len(shard.validations)
		expansions += len(shard.expansions)
		shard.mu.RUnlock()
	}
	return CacheStats{
		Validations: validations,
		Expansions:  expansions,
		Shards:      len(c.shards),
	}
}

// MakeValidationKey creates a cache key for a validation lookup.
func MakeValidationKey(system, code, valueSetURL string) string {
	// Use a separator that won't appear in URLs or codes
	return system + "\x00" + code + "\x00" + valueSetURL
}

// nextPowerOf2 returns the smallest power of 2 >= n.
func nextPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}

// Verify interface compliance
var _ service.TerminologyCache = (*ShardedCache)(nil)
