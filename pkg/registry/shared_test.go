package registry

import (
	"sync"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
)

// Shared registry instance loaded once for all read-only tests.
var (
	sharedRegistry     *Registry
	sharedRegistryOnce sync.Once
	errSharedRegistry  error
)

// getSharedRegistry returns a shared, read-only registry pre-loaded with FHIR
// R4 4.0.1 packages. Tests that mutate the registry must NOT use this helper;
// they should create their own instance via newMutableRegistry instead.
func getSharedRegistry(t *testing.T) *Registry {
	t.Helper()
	sharedRegistryOnce.Do(func() {
		l := loader.NewLoader("")
		packages, err := l.LoadVersion("4.0.1")
		if err != nil {
			errSharedRegistry = err
			return
		}
		r := New()
		if err := r.LoadFromPackages(packages); err != nil {
			errSharedRegistry = err
			return
		}
		sharedRegistry = r
	})
	if errSharedRegistry != nil {
		t.Skipf("Cannot load FHIR packages: %v", errSharedRegistry)
	}
	return sharedRegistry
}

// newMutableRegistry creates a fresh registry loaded with FHIR R4 4.0.1
// packages. Use this for tests that need to modify the registry.
func newMutableRegistry(t *testing.T) *Registry {
	t.Helper()
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}
	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}
	return r
}
