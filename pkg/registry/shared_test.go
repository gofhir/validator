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

// newMutableRegistry returns a registry of the embedded FHIR R4 4.0.1 packages (core, terminology
// and extensions), for tests that modify it: a clone of the one sharedVersion loads.
func newMutableRegistry(t *testing.T) *Registry {
	t.Helper()
	return sharedVersion(t, "4.0.1").Clone()
}
