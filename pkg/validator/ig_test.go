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
