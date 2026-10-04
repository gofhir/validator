package validator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
)

// A package loaded in two versions is refused: which of its definitions applies would depend on
// the order they were loaded in.
func TestOneVersionEach(t *testing.T) {
	pkg := func(name, version string) *loader.Package { return &loader.Package{Name: name, Version: version} }
	if err := oneVersionEach([]*loader.Package{pkg("a", "1"), pkg("b", "1"), pkg("a", "1")}); err != nil {
		t.Errorf("the same version twice: %v", err)
	}
	err := oneVersionEach([]*loader.Package{pkg("a", "1"), pkg("b", "1"), pkg("a", "2")})
	if err == nil || !strings.Contains(err.Error(), "a is loaded in two versions, 1 and 2") {
		t.Errorf("two versions: %v", err)
	}
}

// A base package that is not in the package cache fails the validator's creation, naming it.
func TestBasePackageMissing(t *testing.T) {
	_, err := New(WithVersion("4.0.1"), WithPackagePath(t.TempDir()),
		WithBasePackages(PackageSpec{Name: "hl7.fhir.r4.core", Version: "4.0.1"}))
	if err == nil || !strings.Contains(err.Error(), "base package hl7.fhir.r4.core#4.0.1") {
		t.Errorf("err = %v, want the missing base package named", err)
	}
}

// The base packages given replace the embedded set, and validate with the versions they are.
func TestBasePackagesReplaceTheEmbeddedSet(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	cache := filepath.Join(home, ".fhir", "packages")
	base := []PackageSpec{
		{Name: "hl7.fhir.r4.core", Version: "4.0.1"},
		{Name: "hl7.terminology.r4", Version: "6.2.0"},
		{Name: "hl7.fhir.uv.extensions.r4", Version: "5.3.0"},
	}
	for _, p := range base {
		if _, err := os.Stat(filepath.Join(cache, p.Name+"#"+p.Version)); err != nil {
			t.Skipf("%s#%s is not in the package cache", p.Name, p.Version)
		}
	}
	v, err := New(WithVersion("4.0.1"), WithPackagePath(cache), WithBasePackages(base...))
	if err != nil {
		t.Fatal(err)
	}
	// questionnaire-maxOccurs as extensions 5.3.0 defines it, not as the embedded 5.2.0 does.
	// (The core package defines it too, as 4.0.1: the canonical resolves to the latest version.)
	sd, _ := v.Registry().ResolveCanonical("http://hl7.org/fhir/StructureDefinition/questionnaire-maxOccurs")
	if sd == nil || sd.Version != "5.3.0" {
		t.Fatalf("questionnaire-maxOccurs resolves to %v, want the base set's 5.3.0", sd)
	}
	if _, err := v.Validate(context.Background(), []byte(`{"resourceType":"Patient"}`)); err != nil {
		t.Fatal(err)
	}

	// Adding a base package in another version is refused.
	_, err = New(WithVersion("4.0.1"), WithPackagePath(cache), WithPackage("hl7.terminology.r4", "6.2.0"))
	if err == nil || !strings.Contains(err.Error(), "two versions") {
		t.Errorf("err = %v, want two versions of hl7.terminology.r4 refused", err)
	}
}
