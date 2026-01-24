package service

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a resource cannot be found.
var ErrNotFound = errors.New("resource not found")

// ErrNotSupported is returned when an operation is not supported.
var ErrNotSupported = errors.New("operation not supported")

// --- Profile Chain ---

// ProfileChain implements ProfileResolver by trying multiple resolvers in order.
// This follows the Chain of Responsibility pattern used by HAPI FHIR.
type ProfileChain struct {
	resolvers []StructureDefinitionFetcher
}

// NewProfileChain creates a new profile chain.
func NewProfileChain(resolvers ...StructureDefinitionFetcher) *ProfileChain {
	return &ProfileChain{resolvers: resolvers}
}

// FetchStructureDefinition tries each resolver until one succeeds.
func (c *ProfileChain) FetchStructureDefinition(ctx context.Context, url string) (*StructureDefinition, error) {
	for _, resolver := range c.resolvers {
		sd, err := resolver.FetchStructureDefinition(ctx, url)
		if err == nil && sd != nil {
			return sd, nil
		}
		// Continue to next resolver if not found
		if !errors.Is(err, ErrNotFound) && err != nil {
			return nil, err
		}
	}
	return nil, ErrNotFound
}

// Add appends a resolver to the chain.
func (c *ProfileChain) Add(resolver StructureDefinitionFetcher) {
	c.resolvers = append(c.resolvers, resolver)
}

// --- Terminology Chain ---

// TerminologyChain implements TerminologyService by trying multiple services.
type TerminologyChain struct {
	services []CodeValidator
}

// NewTerminologyChain creates a new terminology chain.
func NewTerminologyChain(services ...CodeValidator) *TerminologyChain {
	return &TerminologyChain{services: services}
}

// ValidateCode tries each service until one succeeds.
func (c *TerminologyChain) ValidateCode(ctx context.Context, system, code, valueSetURL string) (*ValidateCodeResult, error) {
	for _, svc := range c.services {
		result, err := svc.ValidateCode(ctx, system, code, valueSetURL)
		if err == nil {
			return result, nil
		}
		// Continue to next service if not supported
		if !errors.Is(err, ErrNotSupported) && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return nil, ErrNotSupported
}

// Add appends a service to the chain.
func (c *TerminologyChain) Add(service CodeValidator) {
	c.services = append(c.services, service)
}

// --- Reference Chain ---

// ReferenceChain implements ReferenceResolver by trying multiple resolvers.
type ReferenceChain struct {
	resolvers []ReferenceResolver
}

// NewReferenceChain creates a new reference chain.
func NewReferenceChain(resolvers ...ReferenceResolver) *ReferenceChain {
	return &ReferenceChain{resolvers: resolvers}
}

// ResolveReference tries each resolver until one succeeds.
func (c *ReferenceChain) ResolveReference(ctx context.Context, reference string) (*ResolvedReference, error) {
	for _, resolver := range c.resolvers {
		result, err := resolver.ResolveReference(ctx, reference)
		if err == nil && result.Found {
			return result, nil
		}
		// Continue to next resolver if not found
		if !errors.Is(err, ErrNotFound) && err != nil {
			return nil, err
		}
	}
	return &ResolvedReference{Found: false}, nil
}

// Add appends a resolver to the chain.
func (c *ReferenceChain) Add(resolver ReferenceResolver) {
	c.resolvers = append(c.resolvers, resolver)
}

// --- Caching Wrappers ---

// CachingProfileResolver wraps a ProfileResolver with caching.
type CachingProfileResolver struct {
	resolver ProfileResolver
	cache    ProfileCache
}

// NewCachingProfileResolver creates a caching wrapper.
func NewCachingProfileResolver(resolver ProfileResolver, cache ProfileCache) *CachingProfileResolver {
	return &CachingProfileResolver{
		resolver: resolver,
		cache:    cache,
	}
}

// FetchStructureDefinition checks cache first, then calls the wrapped resolver.
func (c *CachingProfileResolver) FetchStructureDefinition(ctx context.Context, url string) (*StructureDefinition, error) {
	// Check cache first
	if sd, ok := c.cache.Get(url); ok {
		return sd, nil
	}

	// Fetch from wrapped resolver
	sd, err := c.resolver.FetchStructureDefinition(ctx, url)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.cache.Set(url, sd)
	return sd, nil
}

// FetchStructureDefinitionByType checks cache first, then calls the wrapped resolver.
func (c *CachingProfileResolver) FetchStructureDefinitionByType(ctx context.Context, resourceType string) (*StructureDefinition, error) {
	// Use resourceType as cache key with prefix to distinguish from URLs
	cacheKey := "type:" + resourceType

	// Check cache first
	if sd, ok := c.cache.Get(cacheKey); ok {
		return sd, nil
	}

	// Fetch from wrapped resolver
	sd, err := c.resolver.FetchStructureDefinitionByType(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.cache.Set(cacheKey, sd)
	return sd, nil
}

// CachingTerminologyService wraps a TerminologyService with caching.
type CachingTerminologyService struct {
	service         TerminologyService
	validationCache TerminologyValidationCache
	expansionCache  TerminologyExpansionCache
}

// TerminologyValidationCache caches validation results.
type TerminologyValidationCache interface {
	Get(key string) (*ValidateCodeResult, bool)
	Set(key string, result *ValidateCodeResult)
}

// TerminologyExpansionCache caches ValueSet expansions.
type TerminologyExpansionCache interface {
	Get(url string) (*ValueSetExpansion, bool)
	Set(url string, expansion *ValueSetExpansion)
}

// NewCachingTerminologyService creates a caching wrapper.
func NewCachingTerminologyService(service TerminologyService, validationCache TerminologyValidationCache, expansionCache TerminologyExpansionCache) *CachingTerminologyService {
	return &CachingTerminologyService{
		service:         service,
		validationCache: validationCache,
		expansionCache:  expansionCache,
	}
}

// ValidateCode checks cache first, then calls the wrapped service.
func (c *CachingTerminologyService) ValidateCode(ctx context.Context, system, code, valueSetURL string) (*ValidateCodeResult, error) {
	// Build cache key
	key := system + "|" + code + "|" + valueSetURL

	// Check cache first
	if c.validationCache != nil {
		if result, ok := c.validationCache.Get(key); ok {
			return result, nil
		}
	}

	// Call wrapped service
	result, err := c.service.ValidateCode(ctx, system, code, valueSetURL)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if c.validationCache != nil {
		c.validationCache.Set(key, result)
	}
	return result, nil
}

// ExpandValueSet checks cache first, then calls the wrapped service.
func (c *CachingTerminologyService) ExpandValueSet(ctx context.Context, url string) (*ValueSetExpansion, error) {
	// Check cache first
	if c.expansionCache != nil {
		if expansion, ok := c.expansionCache.Get(url); ok {
			return expansion, nil
		}
	}

	// Call wrapped service
	expansion, err := c.service.ExpandValueSet(ctx, url)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if c.expansionCache != nil {
		c.expansionCache.Set(url, expansion)
	}
	return expansion, nil
}

// --- Null Implementations ---

// NullProfileResolver is a no-op implementation that always returns ErrNotFound.
type NullProfileResolver struct{}

// FetchStructureDefinition always returns ErrNotFound.
func (NullProfileResolver) FetchStructureDefinition(_ context.Context, _ string) (*StructureDefinition, error) {
	return nil, ErrNotFound
}

// FetchStructureDefinitionByType always returns ErrNotFound.
func (NullProfileResolver) FetchStructureDefinitionByType(_ context.Context, _ string) (*StructureDefinition, error) {
	return nil, ErrNotFound
}

// NullTerminologyService is a no-op implementation.
type NullTerminologyService struct{}

// ValidateCode always returns valid (permissive default).
func (NullTerminologyService) ValidateCode(_ context.Context, _, _, _ string) (*ValidateCodeResult, error) {
	return &ValidateCodeResult{Valid: true}, nil
}

// ExpandValueSet always returns ErrNotSupported.
func (NullTerminologyService) ExpandValueSet(_ context.Context, _ string) (*ValueSetExpansion, error) {
	return nil, ErrNotSupported
}

// NullReferenceResolver is a no-op implementation.
type NullReferenceResolver struct{}

// ResolveReference always returns not found.
func (NullReferenceResolver) ResolveReference(_ context.Context, _ string) (*ResolvedReference, error) {
	return &ResolvedReference{Found: false}, nil
}

// --- Service Aggregator ---

// Services aggregates all validation services.
type Services struct {
	Profile     ProfileResolver
	Terminology TerminologyService
	Reference   ReferenceResolver
}

// NewServices creates a Services with null implementations.
func NewServices() *Services {
	return &Services{
		Profile:     NullProfileResolver{},
		Terminology: NullTerminologyService{},
		Reference:   NullReferenceResolver{},
	}
}

// WithProfile sets the profile resolver.
func (s *Services) WithProfile(p ProfileResolver) *Services {
	s.Profile = p
	return s
}

// WithTerminology sets the terminology service.
func (s *Services) WithTerminology(t TerminologyService) *Services {
	s.Terminology = t
	return s
}

// WithReference sets the reference resolver.
func (s *Services) WithReference(r ReferenceResolver) *Services {
	s.Reference = r
	return s
}
