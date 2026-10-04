package validator

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/logger"
	"github.com/gofhir/validator/pkg/specs"
)

// testPackage is a package with one StructureDefinition, whose url names it.
type testPackage struct {
	name, version, typ, fhirVersion string
	dependencies                    map[string]string
}

func (p testPackage) files() map[string][]byte {
	manifest, _ := json.Marshal(map[string]any{"name": p.name, "version": p.version, "type": p.typ,
		"fhirVersions": []string{p.fhirVersion}, "dependencies": p.dependencies})
	sd, _ := json.Marshal(map[string]any{"resourceType": "StructureDefinition", "id": p.name,
		"url": sdURL(p.name), "version": p.version, "fhirVersion": p.fhirVersion,
		"kind": "logical", "type": sdURL(p.name), "derivation": "specialization"})
	return map[string][]byte{"package/package.json": manifest, "package/StructureDefinition-" + p.name + ".json": sd}
}

func sdURL(name string) string { return "http://example.org/StructureDefinition/" + name }

// install writes the package into a package cache directory.
func (p testPackage) install(t *testing.T, cache string) {
	t.Helper()
	for name, data := range p.files() {
		path := filepath.Join(cache, p.name+"#"+p.version, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// tgz is the package as a registry serves it.
func (p testPackage) tgz(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range p.files() {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// captureLog sends the default logger's output to a buffer for the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := logger.Default()
	logger.SetDefault(logger.New(&buf, logger.LevelWarn))
	t.Cleanup(func() { logger.SetDefault(previous) })
	return &buf
}

// dependencyCache is a package cache with a guide, acme.ig, whose dependencies are:
//   - acme.dep#1.0.0, which depends on acme.leaf#2.0.0 (loaded transitively);
//   - acme.wild#1.x, installed as 1.2.0 and 1.10.0 (the highest matching is loaded);
//   - acme.base#1.0.0, a base package in another version (loaded too);
//   - the core packages of the FHIR version validated (loaded already) and of another (not loaded);
//   - acme.missing#1.0.0, not installed.
func dependencyCache(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	for _, p := range []testPackage{
		{"acme.core", "4.0.1", "fhir.core", "4.0.1", nil},
		{"acme.core.r5", "5.0.0", "Core", "5.0.0", nil},
		{"acme.base", "2.0.0", "IG", "4.0.1", nil},
		{"acme.base", "1.0.0", "IG", "4.0.1", nil},
		{"acme.ig", "1.0.0", "IG", "4.0.1", map[string]string{
			"acme.core": "4.0.1", "acme.core.r5": "5.0.0", "acme.dep": "1.0.0", "acme.wild": "1.x",
			"acme.base": "1.0.0", "acme.missing": "1.0.0"}},
		{"acme.dep", "1.0.0", "IG", "4.0.1", map[string]string{"acme.leaf": "2.0.0"}},
		{"acme.leaf", "2.0.0", "IG", "4.0.1", nil},
		{"acme.wild", "1.2.0", "IG", "4.0.1", nil},
		{"acme.wild", "1.10.0", "IG", "4.0.1", nil},
	} {
		p.install(t, cache)
	}
	return cache
}

func newWithDependencies(t *testing.T, cache string, opts ...Option) *Validator {
	t.Helper()
	opts = append([]Option{WithVersion("4.0.1"), WithPackagePath(cache),
		WithBasePackages(PackageSpec{Name: "acme.core", Version: "4.0.1"}, PackageSpec{Name: "acme.base", Version: "2.0.0"}),
		WithPackage("acme.ig", "1.0.0")}, opts...)
	v, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// A guide's dependencies are loaded, transitively, in the versions it declares; one core package,
// the version validated's, is loaded; a dependency missing from the cache is reported.
func TestPackageDependenciesAreLoaded(t *testing.T) {
	log := captureLog(t)
	v := newWithDependencies(t, dependencyCache(t))
	reg := v.Registry()
	for name, version := range map[string]string{"acme.ig": "1.0.0", "acme.dep": "1.0.0", "acme.leaf": "2.0.0",
		"acme.wild": "1.10.0", "acme.base": "2.0.0"} {
		if sd := reg.GetByURL(sdURL(name)); sd == nil || sd.Version != version {
			t.Errorf("%s resolves to %v, want version %s", name, sd, version)
		}
	}
	if sd, _ := reg.ResolveCanonical(sdURL("acme.base") + "|1.0.0"); sd == nil {
		t.Error("acme.base#1.0.0, a dependency in another version than the base one, is not loaded")
	}
	if sd := reg.GetByURL(sdURL("acme.core.r5")); sd != nil {
		t.Error("the core package of another FHIR version is loaded")
	}
	if sd := reg.GetByURL(sdURL("acme.missing")); sd != nil {
		t.Error("acme.missing is loaded")
	}
	for _, want := range []string{
		"Dependency acme.missing#1.0.0 of acme.ig#1.0.0 is not loaded: it is not in the package cache",
		"Dependency acme.core.r5#5.0.0 of acme.ig#1.0.0 is the core package of FHIR 5.0.0",
	} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log does not report %q:\n%s", want, log)
		}
	}
}

// With a package registry, a package missing from the cache is downloaded into it: a dependency
// and a package given.
func TestMissingPackagesAreDownloaded(t *testing.T) {
	captureLog(t)
	served := map[string][]byte{
		"/acme.missing/1.0.0": testPackage{"acme.missing", "1.0.0", "IG", "4.0.1", nil}.tgz(t),
		// A package downloaded brings its dependencies, a wildcard resolved against the registry.
		"/acme.given/3.0.0":  testPackage{"acme.given", "3.0.0", "IG", "4.0.1", map[string]string{"acme.remote": "2.x"}}.tgz(t),
		"/acme.remote":       []byte(`{"versions":{"2.0.0":{},"2.4.0":{},"2.5.0-ballot":{},"3.0.0":{}}}`),
		"/acme.remote/2.4.0": testPackage{"acme.remote", "2.4.0", "IG", "4.0.1", nil}.tgz(t),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := served[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	cache := dependencyCache(t)
	v := newWithDependencies(t, cache, WithPackageRegistry(server.URL), WithPackage("acme.given", "3.0.0"))
	for _, name := range []string{"acme.missing", "acme.given"} {
		if v.Registry().GetByURL(sdURL(name)) == nil {
			t.Errorf("%s is not loaded", name)
		}
	}
	if sd := v.Registry().GetByURL(sdURL("acme.remote")); sd == nil || sd.Version != "2.4.0" {
		t.Errorf("acme.remote resolves to %v, want 2.4.0", sd)
	}
	if _, err := os.Stat(filepath.Join(cache, "acme.missing#1.0.0", "package", "package.json")); err != nil {
		t.Errorf("acme.missing is not installed in the cache: %v", err)
	}
}

// A dependency on a package loaded already, at the version loaded (the embedded base packages), is
// neither looked for in the cache nor downloaded.
func TestDependenciesOnLoadedPackages(t *testing.T) {
	embedded, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages("4.0.1"))
	if err != nil || len(embedded) == 0 {
		t.Fatalf("embedded packages: %v", err)
	}
	deps := map[string]string{}
	for _, pkg := range embedded {
		deps[pkg.Name] = pkg.Version
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.NotFound(w, r)
	}))
	defer server.Close()

	cache := t.TempDir()
	testPackage{"acme.ig", "1.0.0", "IG", "4.0.1", deps}.install(t, cache)
	log := captureLog(t)
	if _, err := New(WithVersion("4.0.1"), WithPackagePath(cache), WithPackageRegistry(server.URL),
		WithPackage("acme.ig", "1.0.0")); err != nil {
		t.Fatal(err)
	}
	if requests != 0 || log.Len() != 0 {
		t.Errorf("%d requests to the registry, log:\n%s", requests, log)
	}
}

// A core package of another FHIR version given directly is not loaded either.
func TestOtherCoreGivenIsNotLoaded(t *testing.T) {
	log := captureLog(t)
	cache := dependencyCache(t)
	v := newWithDependencies(t, cache, WithPackage("acme.core.r5", "5.0.0"))
	if v.Registry().GetByURL(sdURL("acme.core.r5")) != nil {
		t.Error("the core package of another FHIR version is loaded")
	}
	if !strings.Contains(log.String(), "Package acme.core.r5#5.0.0 is the core package of another FHIR version") {
		t.Errorf("log:\n%s", log)
	}
}

// A package added that is loaded already, at the same version, is not loaded again.
func TestPackageLoadedOnce(t *testing.T) {
	captureLog(t)
	cache := dependencyCache(t)
	once := newWithDependencies(t, cache).Registry().Count()
	twice := newWithDependencies(t, cache, WithPackage("acme.base", "2.0.0"), WithPackage("acme.ig", "1.0.0")).Registry().Count()
	if once != twice {
		t.Errorf("%d definitions, %d with acme.base#2.0.0 and acme.ig#1.0.0 given again", once, twice)
	}
}
