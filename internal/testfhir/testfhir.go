// Package testfhir gives tests the packages the validator embeds for a FHIR version and registries
// of them, loaded once per test binary: loading them is most of what many tests cost, ten times
// more under the race detector.
package testfhir

import (
	"slices"
	"sync"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/specs"
	"github.com/gofhir/validator/pkg/terminology"
)

// base is what is loaded once for a FHIR version.
type base struct {
	packages    []*loader.Package
	registry    *registry.Registry
	terminology *terminology.Registry
	err         error
}

var (
	mu    sync.Mutex
	bases = map[string]func() *base{} // FHIR version -> its base, loaded once
)

// build loads the packages embedded for the FHIR version and registries of them.
func build(version string) *base {
	packages, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages(version))
	if err != nil {
		return &base{err: err}
	}
	reg := registry.New()
	reg.SetFHIRVersion(version)
	if err := reg.LoadFromPackages(packages); err != nil {
		return &base{err: err}
	}
	term := terminology.NewRegistry()
	term.SetFHIRVersion(version)
	if err := term.LoadFromPackages(packages); err != nil {
		return &base{err: err}
	}
	return &base{packages: packages, registry: reg, terminology: term}
}

// loaded returns the base of the FHIR version, loading it the first time it is asked for.
func loaded(version string) *base {
	mu.Lock()
	once, ok := bases[version]
	if !ok {
		once = sync.OnceValue(func() *base { return build(version) })
		bases[version] = once
	}
	mu.Unlock()
	return once()
}

func load(t testing.TB, version string) *base {
	t.Helper()
	b := loaded(version)
	if b.err != nil {
		t.Fatalf("loading the FHIR %s packages: %v", version, b.err)
	}
	return b
}

// Packages returns the packages embedded for the FHIR version. The packages are shared and must
// not be changed; the slice is the caller's.
func Packages(t testing.TB, version string) []*loader.Package {
	t.Helper()
	return slices.Clone(load(t, version).packages)
}

// Registry returns a registry of the packages embedded for the FHIR version, the caller's to load
// more into (registry.Registry.Clone).
func Registry(t testing.TB, version string) *registry.Registry {
	t.Helper()
	return load(t, version).registry.Clone()
}

// Terminology returns a terminology registry of the packages embedded for the FHIR version, the
// caller's to load more into.
func Terminology(t testing.TB, version string) *terminology.Registry {
	t.Helper()
	return load(t, version).terminology.Clone()
}

// Base returns the packages embedded for the FHIR version and registries of them, the caller's to
// load more into, for code that loads them without a test at hand.
func Base(version string) (packages []*loader.Package, reg *registry.Registry, term *terminology.Registry, err error) {
	b := loaded(version)
	if b.err != nil {
		return nil, nil, nil, b.err
	}
	return slices.Clone(b.packages), b.registry.Clone(), b.terminology.Clone(), nil
}
