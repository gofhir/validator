package validator

import (
	"context"
	"testing"
)

func TestValidateWithIGUnknownPackage(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{"resourceType": "Patient", "active": true}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithIG("nonexistent.package#1.0.0"))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Should emit warning about missing IG
	found := false
	for _, iss := range result.Issues {
		if iss.Diagnostics == "No profiles found for IG package 'nonexistent.package#1.0.0'" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning about missing IG package not found")
	}
}

func TestValidateWithIGCorePackage(t *testing.T) {
	v := getSharedValidator(t)

	// Core package has SDs but they are specializations (Derivation != "constraint"),
	// so GetProfilesByPackage returns only constraint SDs (profiles).
	// The core package should not have profiles matching Patient.
	resource := []byte(`{"resourceType": "Patient", "active": true}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithIG("hl7.fhir.r4.core#4.0.1"))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	t.Logf("Core IG: Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
	for _, iss := range result.Issues {
		t.Logf("Issue [%s]: %s", iss.Severity, iss.Diagnostics)
	}
}

func TestValidateWithIGPackageIDTracking(t *testing.T) {
	v := getSharedValidator(t)

	// Verify that the core package SDs have PackageID set
	sd := v.Registry().GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	if sd == nil {
		t.Fatal("Patient SD not found in registry")
	}

	if sd.PackageID == "" {
		t.Error("Patient SD PackageID should not be empty after loading from package")
	}

	t.Logf("Patient SD PackageID: %s", sd.PackageID)
}

func TestValidateWithIGCombinedWithProfile(t *testing.T) {
	v := getSharedValidator(t)

	// ValidateWithIG can be combined with ValidateWithProfile
	resource := []byte(`{"resourceType": "Patient", "id": "123", "active": true}`)
	result, err := v.Validate(context.Background(), resource,
		ValidateWithIG("hl7.fhir.r4.core#4.0.1"),
		ValidateWithProfile("http://hl7.org/fhir/StructureDefinition/Patient"),
	)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	t.Logf("IG + Profile: Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}

func TestValidateWithIGCombinedWithMode(t *testing.T) {
	v := getSharedValidator(t)

	// ValidateWithIG can be combined with ValidateWithMode
	resource := []byte(`{"resourceType": "Patient", "id": "123", "active": true}`)
	result, err := v.Validate(context.Background(), resource,
		ValidateWithIG("hl7.fhir.r4.core#4.0.1"),
		ValidateWithMode(ValidateModeUpdate),
	)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	t.Logf("IG + Mode: Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}

func TestGetProfilesByPackage(t *testing.T) {
	v := getSharedValidator(t)

	// Core R4 package should have some constraint SDs (profiles)
	profiles := v.Registry().GetProfilesByPackage("hl7.fhir.r4.core#4.0.1")
	t.Logf("Core R4 profiles (constraint SDs): %d", len(profiles))
	for i, p := range profiles {
		if i < 5 { // Log first 5
			t.Logf("  %s (type: %s)", p.URL, p.Type)
		}
	}

	// Non-existent package should return nil
	profiles = v.Registry().GetProfilesByPackage("nonexistent#1.0.0")
	if len(profiles) != 0 {
		t.Errorf("Expected 0 profiles for nonexistent package, got %d", len(profiles))
	}
}

// TestWithConformancePackage_PackageIDPreserved verifies that conformance resources
// loaded via WithConformancePackage preserve their package metadata, so that
// Registry.GetProfilesByPackage and ValidateWithIG can scope by IG.
//
// Reproduces the gofhir/go-fhir-server bug where IG SDs loaded via
// WithConformanceResources all collapsed under PackageID="custom#0.0.0",
// breaking IG-scoped validation.
func TestWithConformancePackage_PackageIDPreserved(t *testing.T) {
	const profileURL = "https://example.org/fhir/StructureDefinition/test-patient-profile"
	const pkgName = "test.ig"
	const pkgVersion = "1.0.0"
	const pkgID = pkgName + "#" + pkgVersion

	profileJSON := []byte(`{
		"resourceType": "StructureDefinition",
		"url": "` + profileURL + `",
		"name": "TestPatientProfile",
		"type": "Patient",
		"kind": "resource",
		"abstract": false,
		"derivation": "constraint",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
		"differential": {
			"element": [
				{"id": "Patient", "path": "Patient"},
				{"id": "Patient.identifier", "path": "Patient.identifier", "min": 1}
			]
		}
	}`)

	v, err := New(
		WithVersion("4.0.1"),
		WithConformancePackage(pkgName, pkgVersion, [][]byte{profileJSON}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sd := v.Registry().GetByURL(profileURL)
	if sd == nil {
		t.Fatalf("profile %q not loaded", profileURL)
	}
	if sd.PackageID != pkgID {
		t.Errorf("PackageID = %q, want %q", sd.PackageID, pkgID)
	}

	profiles := v.Registry().GetProfilesByPackage(pkgID)
	if len(profiles) != 1 {
		t.Fatalf("GetProfilesByPackage(%q) = %d profiles, want 1", pkgID, len(profiles))
	}
	if profiles[0].URL != profileURL {
		t.Errorf("profile URL = %q, want %q", profiles[0].URL, profileURL)
	}

	// Resources loaded via the legacy WithConformanceResources MUST NOT leak into
	// the IG-scoped lookup (regression guard).
	if got := v.Registry().GetProfilesByPackage("custom#0.0.0"); len(got) != 0 {
		t.Errorf("custom#0.0.0 should be empty when WithConformancePackage is used, got %d", len(got))
	}
}

// TestWithConformancePackage_ValidateWithIGScopesProfiles verifies that
// ValidateWithIG resolves to the profiles loaded via WithConformancePackage and
// applies them to the resource (the failing path on go-fhir-server v0.13.0).
func TestWithConformancePackage_ValidateWithIGScopesProfiles(t *testing.T) {
	const profileURL = "https://example.org/fhir/StructureDefinition/strict-patient"
	const pkgID = "test.strict.ig#2.0.0"

	profileJSON := []byte(`{
		"resourceType": "StructureDefinition",
		"url": "` + profileURL + `",
		"name": "StrictPatient",
		"type": "Patient",
		"kind": "resource",
		"abstract": false,
		"derivation": "constraint",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
		"differential": {
			"element": [
				{"id": "Patient", "path": "Patient"},
				{"id": "Patient.identifier", "path": "Patient.identifier", "min": 1}
			]
		}
	}`)

	v, err := New(
		WithVersion("4.0.1"),
		WithConformancePackage("test.strict.ig", "2.0.0", [][]byte{profileJSON}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Patient with NO identifier — must fail under the strict profile.
	resource := []byte(`{"resourceType": "Patient", "active": true}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithIG(pkgID))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if !result.Stats.IsCustomProfile {
		t.Error("Stats.IsCustomProfile should be true when IG profiles are applied")
	}

	// The strict profile requires Patient.identifier; without it we expect an error.
	if result.ErrorCount() == 0 {
		t.Errorf("expected at least one cardinality error from the IG profile, got 0")
		for _, iss := range result.Issues {
			t.Logf("issue [%s]: %s", iss.Severity, iss.Diagnostics)
		}
	}
}
