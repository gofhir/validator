package service

import (
	"context"
	"errors"
	"testing"
)

// mockProfileFetcher is a test implementation
type mockProfileFetcher struct {
	profiles map[string]*StructureDefinition
	err      error
}

func (m *mockProfileFetcher) FetchStructureDefinition(ctx context.Context, url string) (*StructureDefinition, error) {
	if m.err != nil {
		return nil, m.err
	}
	if sd, ok := m.profiles[url]; ok {
		return sd, nil
	}
	return nil, ErrNotFound
}

func (m *mockProfileFetcher) FetchStructureDefinitionByType(ctx context.Context, resourceType string) (*StructureDefinition, error) {
	// Look for profile by type
	for _, sd := range m.profiles {
		if sd.Type == resourceType {
			return sd, nil
		}
	}
	return nil, ErrNotFound
}

// mockCodeValidator is a test implementation that implements TerminologyService
type mockCodeValidator struct {
	results    map[string]*ValidateCodeResult
	expansions map[string]*ValueSetExpansion
	err        error
}

func (m *mockCodeValidator) ValidateCode(ctx context.Context, system, code, valueSetURL string) (*ValidateCodeResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := system + "|" + code + "|" + valueSetURL
	if result, ok := m.results[key]; ok {
		return result, nil
	}
	return nil, ErrNotSupported
}

func (m *mockCodeValidator) ExpandValueSet(ctx context.Context, url string) (*ValueSetExpansion, error) {
	if m.err != nil {
		return nil, m.err
	}
	if expansion, ok := m.expansions[url]; ok {
		return expansion, nil
	}
	return nil, ErrNotSupported
}

// mockReferenceResolver is a test implementation
type mockReferenceResolver struct {
	resolved map[string]*ResolvedReference
	err      error
}

func (m *mockReferenceResolver) ResolveReference(ctx context.Context, reference string) (*ResolvedReference, error) {
	if m.err != nil {
		return nil, m.err
	}
	if result, ok := m.resolved[reference]; ok {
		return result, nil
	}
	return &ResolvedReference{Found: false}, nil
}

func TestProfileChain(t *testing.T) {
	fetcher1 := &mockProfileFetcher{
		profiles: map[string]*StructureDefinition{
			"http://example.com/profile1": {URL: "http://example.com/profile1", Name: "Profile1"},
		},
	}
	fetcher2 := &mockProfileFetcher{
		profiles: map[string]*StructureDefinition{
			"http://example.com/profile2": {URL: "http://example.com/profile2", Name: "Profile2"},
		},
	}

	chain := NewProfileChain(fetcher1, fetcher2)

	// Test finding in first fetcher
	sd, err := chain.FetchStructureDefinition(context.Background(), "http://example.com/profile1")
	if err != nil {
		t.Fatalf("FetchStructureDefinition failed: %v", err)
	}
	if sd.Name != "Profile1" {
		t.Errorf("Name = %q; want %q", sd.Name, "Profile1")
	}

	// Test finding in second fetcher (fallback)
	sd, err = chain.FetchStructureDefinition(context.Background(), "http://example.com/profile2")
	if err != nil {
		t.Fatalf("FetchStructureDefinition failed: %v", err)
	}
	if sd.Name != "Profile2" {
		t.Errorf("Name = %q; want %q", sd.Name, "Profile2")
	}

	// Test not found
	_, err = chain.FetchStructureDefinition(context.Background(), "http://example.com/nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestProfileChain_Add(t *testing.T) {
	chain := NewProfileChain()

	fetcher := &mockProfileFetcher{
		profiles: map[string]*StructureDefinition{
			"http://example.com/profile": {URL: "http://example.com/profile"},
		},
	}

	chain.Add(fetcher)

	sd, err := chain.FetchStructureDefinition(context.Background(), "http://example.com/profile")
	if err != nil {
		t.Fatalf("FetchStructureDefinition failed: %v", err)
	}
	if sd == nil {
		t.Error("Expected non-nil StructureDefinition")
	}
}

func TestProfileChain_Error(t *testing.T) {
	customErr := errors.New("custom error")
	fetcher := &mockProfileFetcher{err: customErr}

	chain := NewProfileChain(fetcher)

	_, err := chain.FetchStructureDefinition(context.Background(), "http://example.com/profile")
	if !errors.Is(err, customErr) {
		t.Errorf("Expected custom error, got %v", err)
	}
}

func TestTerminologyChain(t *testing.T) {
	validator1 := &mockCodeValidator{
		results: map[string]*ValidateCodeResult{
			"sys1|code1|vs1": {Valid: true, Display: "Display1"},
		},
	}
	validator2 := &mockCodeValidator{
		results: map[string]*ValidateCodeResult{
			"sys2|code2|vs2": {Valid: true, Display: "Display2"},
		},
	}

	chain := NewTerminologyChain(validator1, validator2)

	// Test finding in first validator
	result, err := chain.ValidateCode(context.Background(), "sys1", "code1", "vs1")
	if err != nil {
		t.Fatalf("ValidateCode failed: %v", err)
	}
	if !result.Valid {
		t.Error("Expected valid result")
	}
	if result.Display != "Display1" {
		t.Errorf("Display = %q; want %q", result.Display, "Display1")
	}

	// Test finding in second validator (fallback)
	result, err = chain.ValidateCode(context.Background(), "sys2", "code2", "vs2")
	if err != nil {
		t.Fatalf("ValidateCode failed: %v", err)
	}
	if result.Display != "Display2" {
		t.Errorf("Display = %q; want %q", result.Display, "Display2")
	}

	// Test not supported
	_, err = chain.ValidateCode(context.Background(), "sys", "code", "vs")
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("Expected ErrNotSupported, got %v", err)
	}
}

func TestReferenceChain(t *testing.T) {
	resolver1 := &mockReferenceResolver{
		resolved: map[string]*ResolvedReference{
			"Patient/1": {Found: true, ResourceType: "Patient", ResourceID: "1"},
		},
	}
	resolver2 := &mockReferenceResolver{
		resolved: map[string]*ResolvedReference{
			"Observation/2": {Found: true, ResourceType: "Observation", ResourceID: "2"},
		},
	}

	chain := NewReferenceChain(resolver1, resolver2)

	// Test finding in first resolver
	result, err := chain.ResolveReference(context.Background(), "Patient/1")
	if err != nil {
		t.Fatalf("ResolveReference failed: %v", err)
	}
	if !result.Found {
		t.Error("Expected found result")
	}
	if result.ResourceType != "Patient" {
		t.Errorf("ResourceType = %q; want %q", result.ResourceType, "Patient")
	}

	// Test finding in second resolver (fallback)
	result, err = chain.ResolveReference(context.Background(), "Observation/2")
	if err != nil {
		t.Fatalf("ResolveReference failed: %v", err)
	}
	if result.ResourceType != "Observation" {
		t.Errorf("ResourceType = %q; want %q", result.ResourceType, "Observation")
	}

	// Test not found
	result, err = chain.ResolveReference(context.Background(), "Unknown/99")
	if err != nil {
		t.Fatalf("ResolveReference failed: %v", err)
	}
	if result.Found {
		t.Error("Expected not found result")
	}
}

// mockProfileCache is a test implementation
type mockProfileCache struct {
	cache map[string]*StructureDefinition
}

func (m *mockProfileCache) Get(url string) (*StructureDefinition, bool) {
	sd, ok := m.cache[url]
	return sd, ok
}

func (m *mockProfileCache) Set(url string, profile *StructureDefinition) {
	m.cache[url] = profile
}

func TestCachingProfileResolver(t *testing.T) {
	fetcher := &mockProfileFetcher{
		profiles: map[string]*StructureDefinition{
			"http://example.com/profile": {URL: "http://example.com/profile", Name: "TestProfile"},
		},
	}
	cache := &mockProfileCache{cache: make(map[string]*StructureDefinition)}

	resolver := NewCachingProfileResolver(fetcher, cache)

	// First call should fetch and cache
	sd, err := resolver.FetchStructureDefinition(context.Background(), "http://example.com/profile")
	if err != nil {
		t.Fatalf("FetchStructureDefinition failed: %v", err)
	}
	if sd.Name != "TestProfile" {
		t.Errorf("Name = %q; want %q", sd.Name, "TestProfile")
	}

	// Verify it was cached
	cached, ok := cache.Get("http://example.com/profile")
	if !ok {
		t.Error("Profile should be cached")
	}
	if cached.Name != "TestProfile" {
		t.Errorf("Cached Name = %q; want %q", cached.Name, "TestProfile")
	}

	// Remove from fetcher to prove cache is used
	delete(fetcher.profiles, "http://example.com/profile")

	// Second call should use cache
	sd, err = resolver.FetchStructureDefinition(context.Background(), "http://example.com/profile")
	if err != nil {
		t.Fatalf("FetchStructureDefinition failed on cached: %v", err)
	}
	if sd.Name != "TestProfile" {
		t.Errorf("Cached Name = %q; want %q", sd.Name, "TestProfile")
	}
}

// mockValidationCache implements TerminologyValidationCache
type mockValidationCache struct {
	data map[string]*ValidateCodeResult
}

func (m *mockValidationCache) Get(key string) (*ValidateCodeResult, bool) {
	r, ok := m.data[key]
	return r, ok
}

func (m *mockValidationCache) Set(key string, result *ValidateCodeResult) {
	m.data[key] = result
}

// mockExpansionCache implements TerminologyExpansionCache
type mockExpansionCache struct {
	data map[string]*ValueSetExpansion
}

func (m *mockExpansionCache) Get(url string) (*ValueSetExpansion, bool) {
	e, ok := m.data[url]
	return e, ok
}

func (m *mockExpansionCache) Set(url string, expansion *ValueSetExpansion) {
	m.data[url] = expansion
}

func TestCachingTerminologyService(t *testing.T) {
	validator := &mockCodeValidator{
		results: map[string]*ValidateCodeResult{
			"sys|code|vs": {Valid: true, Display: "Test"},
		},
	}
	validationCache := &mockValidationCache{
		data: make(map[string]*ValidateCodeResult),
	}
	expansionCache := &mockExpansionCache{
		data: make(map[string]*ValueSetExpansion),
	}

	service := NewCachingTerminologyService(validator, validationCache, expansionCache)

	// First call should validate and cache
	result, err := service.ValidateCode(context.Background(), "sys", "code", "vs")
	if err != nil {
		t.Fatalf("ValidateCode failed: %v", err)
	}
	if !result.Valid {
		t.Error("Expected valid result")
	}

	// Verify it was cached
	key := "sys|code|vs"
	cached, ok := validationCache.Get(key)
	if !ok {
		t.Error("Result should be cached")
	}
	if cached.Display != "Test" {
		t.Errorf("Cached Display = %q; want %q", cached.Display, "Test")
	}
}

func TestNullImplementations(t *testing.T) {
	// Test NullProfileResolver
	npr := NullProfileResolver{}
	_, err := npr.FetchStructureDefinition(context.Background(), "url")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("NullProfileResolver.FetchStructureDefinition should return ErrNotFound")
	}
	_, err = npr.FetchStructureDefinitionByType(context.Background(), "type")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("NullProfileResolver.FetchStructureDefinitionByType should return ErrNotFound")
	}

	// Test NullTerminologyService
	nts := NullTerminologyService{}
	result, err := nts.ValidateCode(context.Background(), "sys", "code", "vs")
	if err != nil {
		t.Errorf("NullTerminologyService.ValidateCode should not error")
	}
	if !result.Valid {
		t.Error("NullTerminologyService.ValidateCode should return valid")
	}
	_, err = nts.ExpandValueSet(context.Background(), "url")
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("NullTerminologyService.ExpandValueSet should return ErrNotSupported")
	}

	// Test NullReferenceResolver
	nrr := NullReferenceResolver{}
	refResult, err := nrr.ResolveReference(context.Background(), "ref")
	if err != nil {
		t.Errorf("NullReferenceResolver.ResolveReference should not error")
	}
	if refResult.Found {
		t.Error("NullReferenceResolver.ResolveReference should return not found")
	}
}

func TestServices(t *testing.T) {
	services := NewServices()

	// Should have null implementations by default
	_, err := services.Profile.FetchStructureDefinition(context.Background(), "url")
	if !errors.Is(err, ErrNotFound) {
		t.Error("Default Profile should return ErrNotFound")
	}

	// Test fluent API
	customProfile := &mockProfileFetcher{
		profiles: map[string]*StructureDefinition{
			"url": {Name: "Custom"},
		},
	}
	services.WithProfile(customProfile)

	sd, err := services.Profile.FetchStructureDefinition(context.Background(), "url")
	if err != nil {
		t.Fatalf("Custom Profile.FetchStructureDefinition failed: %v", err)
	}
	if sd.Name != "Custom" {
		t.Errorf("Name = %q; want %q", sd.Name, "Custom")
	}
}

func BenchmarkProfileChain(b *testing.B) {
	fetcher1 := &mockProfileFetcher{profiles: make(map[string]*StructureDefinition)}
	fetcher2 := &mockProfileFetcher{
		profiles: map[string]*StructureDefinition{
			"http://example.com/profile": {Name: "Profile"},
		},
	}

	chain := NewProfileChain(fetcher1, fetcher2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.FetchStructureDefinition(context.Background(), "http://example.com/profile")
	}
}

func BenchmarkCachingProfileResolver(b *testing.B) {
	fetcher := &mockProfileFetcher{
		profiles: map[string]*StructureDefinition{
			"http://example.com/profile": {Name: "Profile"},
		},
	}
	cache := &mockProfileCache{cache: make(map[string]*StructureDefinition)}

	resolver := NewCachingProfileResolver(fetcher, cache)

	// Warm up cache
	resolver.FetchStructureDefinition(context.Background(), "http://example.com/profile")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.FetchStructureDefinition(context.Background(), "http://example.com/profile")
	}
}
