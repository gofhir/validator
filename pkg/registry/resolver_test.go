package registry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// mockResolver implements ProfileResolver for testing.
type mockResolver struct {
	profiles map[string][]byte
	calls    []string // records (url, version) calls for verification
}

func (m *mockResolver) ResolveProfile(_ context.Context, url, version string) ([]byte, error) {
	key := url
	if version != "" {
		key = url + "|" + version
	}
	m.calls = append(m.calls, key)

	data, ok := m.profiles[key]
	if !ok {
		return nil, nil // not found
	}
	return data, nil
}

// errorResolver always returns an error.
type errorResolver struct{}

func (e *errorResolver) ResolveProfile(_ context.Context, _, _ string) ([]byte, error) {
	return nil, errors.New("resolver unavailable")
}

// makeSDJSON creates a minimal StructureDefinition JSON for testing.
func makeSDJSON(url, version, typeName string) []byte {
	sd := map[string]any{
		"resourceType": "StructureDefinition",
		"url":          url,
		"version":      version,
		"type":         typeName,
		"kind":         "resource",
		"abstract":     false,
		"derivation":   "constraint",
		"snapshot": map[string]any{
			"element": []map[string]any{
				{"path": typeName, "min": 0, "max": "*"},
			},
		},
	}
	data, _ := json.Marshal(sd)
	return data
}

func TestGetByCanonical_VersionMatch(t *testing.T) {
	reg := New()

	// Simulate loading two versions of the same SD
	sd1 := &StructureDefinition{URL: "http://example.org/SD/test", Version: "1.0.0", Type: "Patient"}
	sd2 := &StructureDefinition{URL: "http://example.org/SD/test", Version: "2.0.0", Type: "Patient"}

	reg.mu.Lock()
	reg.byURL[sd1.URL] = sd1 // latest wins in byURL
	reg.byURLVersion[sd1.URL+"|"+sd1.Version] = sd1
	reg.byURL[sd2.URL] = sd2
	reg.byURLVersion[sd2.URL+"|"+sd2.Version] = sd2
	reg.mu.Unlock()

	// Exact version match
	got := reg.GetByCanonical("http://example.org/SD/test", "1.0.0")
	if got != sd1 {
		t.Error("expected sd1 for version 1.0.0")
	}

	got = reg.GetByCanonical("http://example.org/SD/test", "2.0.0")
	if got != sd2 {
		t.Error("expected sd2 for version 2.0.0")
	}

	// No version — returns whatever is in byURL
	got = reg.GetByCanonical("http://example.org/SD/test", "")
	if got == nil {
		t.Error("expected non-nil for empty version")
	}

	// Non-existent version — falls back to byURL
	got = reg.GetByCanonical("http://example.org/SD/test", "9.9.9")
	if got == nil {
		t.Error("expected fallback to byURL for unknown version")
	}

	// Non-existent URL
	got = reg.GetByCanonical("http://example.org/SD/nonexistent", "")
	if got != nil {
		t.Error("expected nil for unknown URL")
	}
}

func TestResolveByCanonical_RegistryHit(t *testing.T) {
	reg := New()
	resolver := &mockResolver{profiles: make(map[string][]byte)}
	reg.SetResolver(resolver)

	// Pre-load SD in registry
	sd := &StructureDefinition{URL: "http://example.org/SD/test", Version: "1.0.0"}
	reg.mu.Lock()
	reg.byURL[sd.URL] = sd
	reg.byURLVersion[sd.URL+"|"+sd.Version] = sd
	reg.mu.Unlock()

	// Should return from registry without calling resolver
	got := reg.ResolveByCanonical(context.Background(), "http://example.org/SD/test", "1.0.0")
	if got != sd {
		t.Error("expected SD from registry")
	}
	if len(resolver.calls) != 0 {
		t.Errorf("resolver should not be called, got %d calls", len(resolver.calls))
	}
}

func TestResolveByCanonical_ResolverFetch(t *testing.T) {
	reg := New()
	sdJSON := makeSDJSON("http://example.org/SD/custom", "1.0.0", "Patient")
	resolver := &mockResolver{
		profiles: map[string][]byte{
			"http://example.org/SD/custom|1.0.0": sdJSON,
		},
	}
	reg.SetResolver(resolver)

	// Not in registry — should fetch from resolver
	got := reg.ResolveByCanonical(context.Background(), "http://example.org/SD/custom", "1.0.0")
	if got == nil {
		t.Fatal("expected resolved SD")
	}
	if got.URL != "http://example.org/SD/custom" {
		t.Errorf("URL = %q, want %q", got.URL, "http://example.org/SD/custom")
	}
	if got.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", got.Version, "1.0.0")
	}
	if len(resolver.calls) != 1 {
		t.Errorf("expected 1 resolver call, got %d", len(resolver.calls))
	}

	// Subsequent call should hit cache (no additional resolver call)
	got2 := reg.ResolveByCanonical(context.Background(), "http://example.org/SD/custom", "1.0.0")
	if got2 == nil {
		t.Fatal("expected cached SD")
	}
	if len(resolver.calls) != 1 {
		t.Errorf("expected still 1 resolver call after cache hit, got %d", len(resolver.calls))
	}
}

func TestResolveByCanonical_ResolverNotFound(t *testing.T) {
	reg := New()
	resolver := &mockResolver{profiles: make(map[string][]byte)}
	reg.SetResolver(resolver)

	got := reg.ResolveByCanonical(context.Background(), "http://example.org/SD/missing", "")
	if got != nil {
		t.Error("expected nil for missing profile")
	}
	if len(resolver.calls) != 1 {
		t.Errorf("expected 1 resolver call, got %d", len(resolver.calls))
	}
}

func TestResolveByCanonical_ResolverError(t *testing.T) {
	reg := New()
	reg.SetResolver(&errorResolver{})

	// Resolver error should be treated as "not found" (fail-open)
	got := reg.ResolveByCanonical(context.Background(), "http://example.org/SD/test", "")
	if got != nil {
		t.Error("expected nil on resolver error")
	}
}

func TestResolveByCanonical_InvalidJSON(t *testing.T) {
	reg := New()
	resolver := &mockResolver{
		profiles: map[string][]byte{
			"http://example.org/SD/bad": []byte(`{invalid json`),
		},
	}
	reg.SetResolver(resolver)

	got := reg.ResolveByCanonical(context.Background(), "http://example.org/SD/bad", "")
	if got != nil {
		t.Error("expected nil for invalid JSON")
	}
}

func TestResolveByCanonical_NoResolver(t *testing.T) {
	reg := New()
	// No resolver configured — pure standalone mode

	got := reg.ResolveByCanonical(context.Background(), "http://example.org/SD/test", "")
	if got != nil {
		t.Error("expected nil without resolver")
	}
}

func TestResolveBaseChain(t *testing.T) {
	reg := New()

	baseJSON := makeSDJSON("http://example.org/SD/base", "1.0.0", "Patient")
	profileJSON := makeSDJSON("http://example.org/SD/profile", "1.0.0", "Patient")

	// Modify profileJSON to have baseDefinition
	var profileMap map[string]any
	_ = json.Unmarshal(profileJSON, &profileMap)
	profileMap["baseDefinition"] = "http://example.org/SD/base"
	profileJSON, _ = json.Marshal(profileMap)

	resolver := &mockResolver{
		profiles: map[string][]byte{
			"http://example.org/SD/base": baseJSON,
		},
	}
	reg.SetResolver(resolver)

	// Parse the profile SD
	var profileSD StructureDefinition
	_ = json.Unmarshal(profileJSON, &profileSD)
	profileSD.raw = profileJSON

	// Resolve base chain — should fetch base SD via resolver
	reg.ResolveBaseChain(context.Background(), &profileSD)

	// Base should now be cached in registry
	base := reg.GetByURL("http://example.org/SD/base")
	if base == nil {
		t.Error("expected base SD to be cached after ResolveBaseChain")
	}
}

func TestResolveBaseChain_CycleProtection(_ *testing.T) {
	reg := New()

	// Create two SDs that reference each other
	sdA := &StructureDefinition{URL: "http://example.org/SD/a", BaseDefinition: "http://example.org/SD/b"}
	sdB := &StructureDefinition{URL: "http://example.org/SD/b", BaseDefinition: "http://example.org/SD/a"}

	reg.mu.Lock()
	reg.byURL[sdA.URL] = sdA
	reg.byURL[sdB.URL] = sdB
	reg.mu.Unlock()

	// Should not hang or panic — cycle protection breaks the loop
	reg.ResolveBaseChain(context.Background(), sdA)
}
