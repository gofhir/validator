package validator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
)

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
	// An extension's contexts are those of the version its url resolves to: event-location 5.3.0
	// lists DocumentReference, not Media (the R4 core package's 4.0.1 lists Media), as in the HL7
	// validator.
	media := `{"resourceType":"Media","status":"completed","content":{"contentType":"image/png"},"extension":[` +
		`{"url":"http://hl7.org/fhir/StructureDefinition/event-location","valueReference":{"reference":"Location/l"}}]}`
	res, err := v.Validate(context.Background(), []byte(media))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, iss := range res.Issues {
		found = found || iss.MessageID == string(issue.DiagExtensionInvalidContext)
	}
	if !found {
		t.Errorf("event-location on Media: %v, want it not allowed there", res.Issues)
	}

	// A base package in another version is loaded too: an unversioned canonical resolves to the
	// highest version loaded, a pinned one to its version.
	if _, err := os.Stat(filepath.Join(cache, "hl7.fhir.uv.extensions.r4#5.2.0")); err != nil {
		t.Skip("hl7.fhir.uv.extensions.r4#5.2.0 is not in the package cache")
	}
	v, err = New(WithVersion("4.0.1"), WithPackagePath(cache), WithBasePackages(base...),
		WithPackage("hl7.fhir.uv.extensions.r4", "5.2.0"))
	if err != nil {
		t.Fatal(err)
	}
	const maxOccurs = "http://hl7.org/fhir/StructureDefinition/questionnaire-maxOccurs"
	if sd, _ := v.Registry().ResolveCanonical(maxOccurs); sd == nil || sd.Version != "5.3.0" {
		t.Errorf("%s resolves to %v, want 5.3.0", maxOccurs, sd)
	}
	if sd, _ := v.Registry().ResolveCanonical(maxOccurs + "|5.2.0"); sd == nil || sd.Version != "5.2.0" {
		t.Errorf("%s|5.2.0 resolves to %v, want 5.2.0", maxOccurs, sd)
	}
}
