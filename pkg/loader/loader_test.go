package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPackagePath(t *testing.T) {
	path := DefaultPackagePath()
	if path == "" {
		t.Error("DefaultPackagePath returned empty string")
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".fhir", "packages")
	if path != expected {
		t.Errorf("DefaultPackagePath = %q, want %q", path, expected)
	}
}

func TestPackageRefString(t *testing.T) {
	ref := PackageRef{Name: "hl7.fhir.r4.core", Version: "4.0.1"}
	expected := "hl7.fhir.r4.core#4.0.1"
	if ref.String() != expected {
		t.Errorf("PackageRef.String() = %q, want %q", ref.String(), expected)
	}
}

func TestParsePackageSpec(t *testing.T) {
	tests := []struct {
		spec        string
		wantName    string
		wantVersion string
	}{
		{"hl7.fhir.r4.core#4.0.1", "hl7.fhir.r4.core", "4.0.1"},
		{"hl7.terminology.r4#7.0.1", "hl7.terminology.r4", "7.0.1"},
		{"package-without-version", "package-without-version", ""},
	}

	for _, tt := range tests {
		name, version := ParsePackageSpec(tt.spec)
		if name != tt.wantName || version != tt.wantVersion {
			t.Errorf("ParsePackageSpec(%q) = (%q, %q), want (%q, %q)",
				tt.spec, name, version, tt.wantName, tt.wantVersion)
		}
	}
}

func TestDefaultPackagesConfig(t *testing.T) {
	// Verify all expected versions are configured
	versions := []string{"4.0.1", "4.3.0", "5.0.0"}
	for _, v := range versions {
		refs, ok := DefaultPackages[v]
		if !ok {
			t.Errorf("DefaultPackages missing version %s", v)
			continue
		}
		if len(refs) < 3 {
			t.Errorf("DefaultPackages[%s] has %d packages, want at least 3", v, len(refs))
		}

		// Verify core package exists
		hasCore := false
		for _, ref := range refs {
			if ref.Name == "hl7.fhir.r4.core" || ref.Name == "hl7.fhir.r4b.core" || ref.Name == "hl7.fhir.r5.core" {
				hasCore = true
				break
			}
		}
		if !hasCore {
			t.Errorf("DefaultPackages[%s] missing core package", v)
		}
	}
}

func TestLoaderListPackages(t *testing.T) {
	loader := NewLoader("")
	packages, err := loader.ListPackages()
	if err != nil {
		t.Skipf("Cannot list packages (cache may not exist): %v", err)
	}

	t.Logf("Found %d packages in cache", len(packages))
	for _, pkg := range packages {
		t.Logf("  - %s", pkg)
	}
}

func TestLoaderLoadPackage(t *testing.T) {
	loader := NewLoader("")

	// Try to load R4 core package
	pkg, err := loader.LoadPackage("hl7.fhir.r4.core", "4.0.1")
	if err != nil {
		t.Skipf("Cannot load hl7.fhir.r4.core#4.0.1 (may not be installed): %v", err)
	}

	if pkg.Name != "hl7.fhir.r4.core" {
		t.Errorf("Package.Name = %q, want %q", pkg.Name, "hl7.fhir.r4.core")
	}
	if pkg.Version != "4.0.1" {
		t.Errorf("Package.Version = %q, want %q", pkg.Version, "4.0.1")
	}

	t.Logf("Loaded package with %d resources", len(pkg.Resources))

	// Verify some expected resources exist
	expectedResources := []string{
		"http://hl7.org/fhir/StructureDefinition/Patient",
		"http://hl7.org/fhir/StructureDefinition/Observation",
		"http://hl7.org/fhir/StructureDefinition/HumanName",
	}

	for _, url := range expectedResources {
		if _, ok := pkg.Resources[url]; !ok {
			t.Errorf("Package missing expected resource: %s", url)
		}
	}
}

func TestLoaderLoadVersion(t *testing.T) {
	loader := NewLoader("")

	packages, err := loader.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR 4.0.1 packages: %v", err)
	}

	if len(packages) == 0 {
		t.Error("LoadVersion returned no packages")
	}

	t.Logf("Loaded %d packages for FHIR 4.0.1", len(packages))
	for _, pkg := range packages {
		t.Logf("  - %s#%s (%d resources)", pkg.Name, pkg.Version, len(pkg.Resources))
	}
}

func TestLoaderLoadVersionUnknown(t *testing.T) {
	loader := NewLoader("")

	_, err := loader.LoadVersion("99.99.99")
	if err == nil {
		t.Error("LoadVersion should fail for unknown version")
	}
}

func TestLoaderLoadFromTgzData(t *testing.T) {
	// Find a .tgz file in the package cache to use as test data
	l := NewLoader("")
	cacheDir := l.BasePath()
	tgzPath := filepath.Join(cacheDir, "hl7.fhir.r4.core#4.0.1.tgz")

	data, err := os.ReadFile(tgzPath)
	if err != nil {
		// Try alternative location
		tgzPath = filepath.Join(cacheDir, "hl7.fhir.r4.core-4.0.1.tgz")
		data, err = os.ReadFile(tgzPath)
		if err != nil {
			t.Skipf("No .tgz file available for testing: %v", err)
		}
	}

	pkg, err := l.LoadFromTgzData(data)
	if err != nil {
		t.Fatalf("LoadFromTgzData() error: %v", err)
	}

	if pkg.Name == "" {
		t.Error("Package.Name is empty")
	}
	if pkg.Path != "memory" {
		t.Errorf("Package.Path = %q, want %q", pkg.Path, "memory")
	}
	if len(pkg.Resources) == 0 {
		t.Error("Package has no resources")
	}
	t.Logf("Loaded %s#%s from memory (%d resources)", pkg.Name, pkg.Version, len(pkg.Resources))
}

func TestLoaderLoadFromResources(t *testing.T) {
	l := NewLoader("")

	sd := []byte(`{
		"resourceType": "StructureDefinition",
		"id": "test-profile",
		"url": "http://example.org/fhir/StructureDefinition/test-profile",
		"name": "TestProfile",
		"status": "active",
		"kind": "resource",
		"type": "Patient"
	}`)

	vs := []byte(`{
		"resourceType": "ValueSet",
		"id": "test-valueset",
		"url": "http://example.org/fhir/ValueSet/test-valueset",
		"name": "TestValueSet",
		"status": "active"
	}`)

	invalid := []byte(`not valid json`)

	pkg, err := l.LoadFromResources([][]byte{sd, vs, invalid})
	if err != nil {
		t.Fatalf("LoadFromResources() error: %v", err)
	}

	if pkg.Name != "custom" {
		t.Errorf("Package.Name = %q, want %q", pkg.Name, "custom")
	}
	if pkg.Path != "memory" {
		t.Errorf("Package.Path = %q, want %q", pkg.Path, "memory")
	}

	// Should have 4 entries: 2 by URL + 2 by resourceType/id (invalid JSON skipped)
	if len(pkg.Resources) != 4 {
		t.Errorf("Package has %d resources, want 4", len(pkg.Resources))
	}

	if _, ok := pkg.Resources["http://example.org/fhir/StructureDefinition/test-profile"]; !ok {
		t.Error("Missing resource by URL: StructureDefinition")
	}
	if _, ok := pkg.Resources["StructureDefinition/test-profile"]; !ok {
		t.Error("Missing resource by resourceType/id: StructureDefinition")
	}
	if _, ok := pkg.Resources["http://example.org/fhir/ValueSet/test-valueset"]; !ok {
		t.Error("Missing resource by URL: ValueSet")
	}
	if _, ok := pkg.Resources["ValueSet/test-valueset"]; !ok {
		t.Error("Missing resource by resourceType/id: ValueSet")
	}
}
