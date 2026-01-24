package terminology

import (
	"context"

	"github.com/gofhir/validator/service"
)

// CachedTerminologyService wraps a TerminologyService with caching.
// It uses a ShardedCache to cache validation results and expansions.
type CachedTerminologyService struct {
	inner *InMemoryTerminologyService
	cache *ShardedCache
}

// NewCachedTerminologyService creates a new cached terminology service.
func NewCachedTerminologyService(config CacheConfig) *CachedTerminologyService {
	return &CachedTerminologyService{
		inner: NewInMemoryTerminologyService(),
		cache: NewShardedCache(config),
	}
}

// NewCachedTerminologyServiceWithDefaults creates a cached terminology service
// with default configuration.
func NewCachedTerminologyServiceWithDefaults() *CachedTerminologyService {
	return NewCachedTerminologyService(DefaultCacheConfig())
}

// Inner returns the underlying InMemoryTerminologyService for loading data.
func (s *CachedTerminologyService) Inner() *InMemoryTerminologyService {
	return s.inner
}

// Cache returns the underlying cache for inspection or manual operations.
func (s *CachedTerminologyService) Cache() *ShardedCache {
	return s.cache
}

// ValidateCode implements service.CodeValidator with caching.
func (s *CachedTerminologyService) ValidateCode(ctx context.Context, system, code, valueSetURL string) (*service.ValidateCodeResult, error) {
	// Check cache first
	key := MakeValidationKey(system, code, valueSetURL)
	if cached, ok := s.cache.GetValidation(key); ok {
		return cached, nil
	}

	// Cache miss - delegate to inner service
	result, err := s.inner.ValidateCode(ctx, system, code, valueSetURL)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cache.SetValidation(key, result)
	return result, nil
}

// ExpandValueSet implements service.ValueSetExpander with caching.
func (s *CachedTerminologyService) ExpandValueSet(ctx context.Context, url string) (*service.ValueSetExpansion, error) {
	// Check cache first
	if cached, ok := s.cache.GetExpansion(url); ok {
		return cached, nil
	}

	// Cache miss - delegate to inner service
	expansion, err := s.inner.ExpandValueSet(ctx, url)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cache.SetExpansion(url, expansion)
	return expansion, nil
}

// ClearCache clears all cached entries.
func (s *CachedTerminologyService) ClearCache() {
	s.cache.Clear()
}

// CacheStats returns cache statistics.
func (s *CachedTerminologyService) CacheStats() CacheStats {
	return s.cache.Stats()
}

// Verify interface compliance
var _ service.TerminologyService = (*CachedTerminologyService)(nil)
