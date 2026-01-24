package terminology

import (
	"sync"
	"testing"
	"time"

	"github.com/gofhir/validator/service"
)

func TestShardedCache_Validation(t *testing.T) {
	cache := NewShardedCache(DefaultCacheConfig())

	key := MakeValidationKey("http://example.com", "test", "")
	result := &service.ValidateCodeResult{
		Valid:   true,
		Code:    "test",
		Display: "Test",
	}

	// Get before set should return false
	if _, ok := cache.GetValidation(key); ok {
		t.Error("expected GetValidation to return false for non-existent key")
	}

	// Set and get
	cache.SetValidation(key, result)

	got, ok := cache.GetValidation(key)
	if !ok {
		t.Fatal("expected GetValidation to return true")
	}

	if got.Code != result.Code {
		t.Errorf("expected Code %s, got %s", result.Code, got.Code)
	}
}

func TestShardedCache_Expansion(t *testing.T) {
	cache := NewShardedCache(DefaultCacheConfig())

	url := "http://example.com/ValueSet/test"
	expansion := &service.ValueSetExpansion{
		URL:   url,
		Total: 5,
	}

	// Get before set should return false
	if _, ok := cache.GetExpansion(url); ok {
		t.Error("expected GetExpansion to return false for non-existent key")
	}

	// Set and get
	cache.SetExpansion(url, expansion)

	got, ok := cache.GetExpansion(url)
	if !ok {
		t.Fatal("expected GetExpansion to return true")
	}

	if got.Total != expansion.Total {
		t.Errorf("expected Total %d, got %d", expansion.Total, got.Total)
	}
}

func TestShardedCache_TTL(t *testing.T) {
	// Use very short TTL for testing
	cache := NewShardedCache(CacheConfig{
		ShardCount: 4,
		TTL:        50 * time.Millisecond,
	})

	key := MakeValidationKey("http://example.com", "test", "")
	result := &service.ValidateCodeResult{Valid: true}

	cache.SetValidation(key, result)

	// Should be present immediately
	if _, ok := cache.GetValidation(key); !ok {
		t.Error("expected entry to be present immediately")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	if _, ok := cache.GetValidation(key); ok {
		t.Error("expected entry to be expired")
	}
}

func TestShardedCache_Concurrent(t *testing.T) {
	cache := NewShardedCache(DefaultCacheConfig())

	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := MakeValidationKey("http://example.com", "code", "")
				result := &service.ValidateCodeResult{
					Valid: true,
					Code:  "code",
				}

				// Mix reads and writes
				if j%2 == 0 {
					cache.SetValidation(key, result)
				} else {
					cache.GetValidation(key)
				}
			}
		}()
	}

	wg.Wait()

	stats := cache.Stats()
	t.Logf("Cache stats after concurrent access: validations=%d, shards=%d",
		stats.Validations, stats.Shards)
}

func TestShardedCache_Clear(t *testing.T) {
	cache := NewShardedCache(DefaultCacheConfig())

	// Add some entries
	for i := 0; i < 100; i++ {
		key := MakeValidationKey("http://example.com", "code", "")
		cache.SetValidation(key, &service.ValidateCodeResult{Valid: true})
	}

	stats := cache.Stats()
	if stats.Validations == 0 {
		t.Error("expected some validations before clear")
	}

	// Clear
	cache.Clear()

	stats = cache.Stats()
	if stats.Validations != 0 {
		t.Errorf("expected 0 validations after clear, got %d", stats.Validations)
	}
}

func TestShardedCache_Cleanup(t *testing.T) {
	cache := NewShardedCache(CacheConfig{
		ShardCount: 4,
		TTL:        50 * time.Millisecond,
	})

	// Add entries
	for i := 0; i < 10; i++ {
		key := MakeValidationKey("http://example.com", "code", "")
		cache.SetValidation(key, &service.ValidateCodeResult{Valid: true})
	}

	// Wait for entries to expire
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cache.Cleanup()

	stats := cache.Stats()
	if stats.Validations != 0 {
		t.Errorf("expected 0 validations after cleanup, got %d", stats.Validations)
	}
}

func BenchmarkShardedCache_SetGet(b *testing.B) {
	cache := NewShardedCache(DefaultCacheConfig())
	key := MakeValidationKey("http://example.com", "test", "")
	result := &service.ValidateCodeResult{Valid: true}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.SetValidation(key, result)
			cache.GetValidation(key)
		}
	})
}

func BenchmarkShardedCache_ConcurrentGet(b *testing.B) {
	cache := NewShardedCache(DefaultCacheConfig())
	key := MakeValidationKey("http://example.com", "test", "")
	result := &service.ValidateCodeResult{Valid: true}
	cache.SetValidation(key, result)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.GetValidation(key)
		}
	})
}
