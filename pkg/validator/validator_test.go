package validator

import (
	"context"
	"sync"
	"testing"
)

// Shared validator instance for tests to avoid repeated package loading.
var (
	sharedValidator     *Validator
	sharedValidatorOnce sync.Once
	errSharedValidator  error
)

// getSharedValidator returns a shared validator instance for tests.
func getSharedValidator(t *testing.T) *Validator {
	t.Helper()
	sharedValidatorOnce.Do(func() {
		sharedValidator, errSharedValidator = New()
	})
	if errSharedValidator != nil {
		t.Skipf("Cannot create validator (packages may not be installed): %v", errSharedValidator)
	}
	return sharedValidator
}

func TestNewValidator(t *testing.T) {
	v := getSharedValidator(t)

	if v == nil {
		t.Fatal("New() returned nil validator")
	}

	if v.Version() != "4.0.1" {
		t.Errorf("Version() = %q, want %q", v.Version(), "4.0.1")
	}

	if v.Registry() == nil {
		t.Error("Registry() returned nil")
	}

	t.Logf("Validator created with %d StructureDefinitions", v.Registry().Count())
}

func TestNewValidatorDefaultVersion(t *testing.T) {
	v := getSharedValidator(t)

	if v.Version() != "4.0.1" {
		t.Errorf("Default version = %q, want %q", v.Version(), "4.0.1")
	}
}

func TestNewValidatorUnknownVersion(t *testing.T) {
	_, err := New(WithVersion("99.99.99"))
	if err == nil {
		t.Error("New() should fail for unknown FHIR version")
	}
}

func TestValidateInvalidJSON(t *testing.T) {
	v := getSharedValidator(t)

	result, err := v.Validate(context.Background(), []byte("not valid json"))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("Result should have errors for invalid JSON")
	}

	if result.ErrorCount() != 1 {
		t.Errorf("Result should have 1 error, got %d", result.ErrorCount())
	}

	t.Logf("Error: %s", result.Issues[0].Diagnostics)
}

func TestValidateMissingResourceType(t *testing.T) {
	v := getSharedValidator(t)

	result, err := v.Validate(context.Background(), []byte(`{"name": "test"}`))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("Result should have errors for missing resourceType")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Diagnostics == "Missing 'resourceType' property" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'Missing resourceType' error not found")
	}
}

func TestValidateUnknownResourceType(t *testing.T) {
	v := getSharedValidator(t)

	result, err := v.Validate(context.Background(), []byte(`{"resourceType": "NotAResource"}`))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("Result should have errors for unknown resourceType")
	}

	found := false
	for _, issue := range result.Issues {
		t.Logf("Issue: %s", issue.Diagnostics)
		if issue.Diagnostics == "Unknown resourceType 'NotAResource'" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'Unknown resourceType' error not found")
	}
}

func TestValidateMinimalPatient(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{"resourceType": "Patient"}`)
	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// A minimal Patient with just resourceType should be valid (no required fields)
	t.Logf("Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}

func TestValidatePatientWithName(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{
		"resourceType": "Patient",
		"name": [{"family": "Smith", "given": ["John"]}]
	}`)
	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	t.Logf("Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
	for _, issue := range result.Issues {
		t.Logf("Issue [%s]: %s @ %v", issue.Severity, issue.Diagnostics, issue.Expression)
	}
}

func TestValidatorConfig(t *testing.T) {
	v := getSharedValidator(t)

	config := v.Config()
	if config.FHIRVersion != "4.0.1" {
		t.Errorf("Config.FHIRVersion = %q, want %q", config.FHIRVersion, "4.0.1")
	}
}

func TestValidateJSON(t *testing.T) {
	v := getSharedValidator(t)

	jsonStr := `{"resourceType": "Patient", "active": true}`
	result, err := v.ValidateJSON(context.Background(), jsonStr)
	if err != nil {
		t.Fatalf("ValidateJSON() returned error: %v", err)
	}

	t.Logf("Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}

func TestValidateWithUSCoreProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping US Core test in short mode")
	}

	v, err := New(WithPackage("hl7.fhir.us.core", "6.1.0"))
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}

	// Check if US Core is available
	usCorePatient := v.Registry().GetByURL("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient")
	if usCorePatient == nil {
		t.Skip("US Core Patient profile not available (package may not be installed)")
	}

	// Valid US Core Patient (minimal requirements: identifier, name, gender)
	validPatient := `{
		"resourceType": "Patient",
		"meta": {
			"profile": ["http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"]
		},
		"identifier": [{"system": "http://example.org/mrn", "value": "12345"}],
		"name": [{"family": "Test", "given": ["John"]}],
		"gender": "male"
	}`

	result, err := v.ValidateJSON(context.Background(), validPatient)
	if err != nil {
		t.Fatalf("ValidateJSON() returned error: %v", err)
	}

	t.Logf("Valid US Core Patient: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
	for _, iss := range result.Issues {
		t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
	}

	// Invalid US Core Patient (missing required elements)
	invalidPatient := `{
		"resourceType": "Patient",
		"meta": {
			"profile": ["http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"]
		},
		"active": true
	}`

	result2, err := v.ValidateJSON(context.Background(), invalidPatient)
	if err != nil {
		t.Fatalf("ValidateJSON() returned error: %v", err)
	}

	t.Logf("Invalid US Core Patient: %d errors, %d warnings", result2.ErrorCount(), result2.WarningCount())
	for _, iss := range result2.Issues {
		t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
	}

	// US Core Patient requires: identifier (1..*), name (1..*), gender (1..1)
	if result2.ErrorCount() == 0 {
		t.Error("Expected errors for invalid US Core Patient (missing required elements)")
	}
}

func TestValidateWithMultipleProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multiple profiles test in short mode")
	}

	v, err := New(WithPackage("hl7.fhir.us.core", "6.1.0"))
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}

	// Check if US Core is available
	if v.Registry().GetByURL("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient") == nil {
		t.Skip("US Core Patient profile not available")
	}

	// Patient with multiple profiles
	patient := `{
		"resourceType": "Patient",
		"meta": {
			"profile": [
				"http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient",
				"http://example.org/nonexistent-profile"
			]
		},
		"identifier": [{"system": "http://example.org/mrn", "value": "12345"}],
		"name": [{"family": "Test", "given": ["John"]}],
		"gender": "male"
	}`

	result, err := v.ValidateJSON(context.Background(), patient)
	if err != nil {
		t.Fatalf("ValidateJSON() returned error: %v", err)
	}

	t.Logf("Multiple profiles: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
	t.Logf("Profile used: %s", result.Stats.ProfileURL)

	// Verify first valid profile is stored in stats
	if result.Stats.ProfileURL != "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient" {
		t.Errorf("Expected US Core Patient profile, got %s", result.Stats.ProfileURL)
	}

	// Verify warning emitted for non-existent profile
	foundNotFoundWarning := false
	for _, iss := range result.Issues {
		t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
		if iss.Severity == "warning" && iss.Code == "not-found" {
			foundNotFoundWarning = true
		}
	}
	if !foundNotFoundWarning {
		t.Error("Expected warning for non-existent profile 'http://example.org/nonexistent-profile'")
	}
}

func TestValidateAgainstAllProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping all profiles test in short mode")
	}

	v, err := New(WithPackage("hl7.fhir.us.core", "6.1.0"))
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}

	// Check if both profiles are available
	usCorePatient := v.Registry().GetByURL("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient")
	vitalsigns := v.Registry().GetByURL("http://hl7.org/fhir/StructureDefinition/vitalsigns")
	if usCorePatient == nil || vitalsigns == nil {
		t.Skip("Required profiles not available")
	}

	// A resource that is valid for both profiles would be validated against both
	v2, err := New(
		WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
		WithPackage("hl7.fhir.us.core", "6.1.0"),
	)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Valid patient for US Core
	validPatient := `{
		"resourceType": "Patient",
		"identifier": [{"system": "http://example.org/mrn", "value": "12345"}],
		"name": [{"family": "Test", "given": ["John"]}],
		"gender": "male"
	}`

	result, err := v2.ValidateJSON(context.Background(), validPatient)
	if err != nil {
		t.Fatalf("ValidateJSON() returned error: %v", err)
	}

	t.Logf("Config profile validation: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
	for _, iss := range result.Issues {
		t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
	}

	// Should be valid against US Core Patient (no cardinality errors)
	if result.Stats.ProfileURL != "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient" {
		t.Errorf("Expected US Core Patient profile, got %s", result.Stats.ProfileURL)
	}
}

func TestValidateWithPerCallProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping per-call profile test in short mode")
	}

	// Create a single validator with US Core packages loaded (no config-time profiles)
	v, err := New(WithPackage("hl7.fhir.us.core", "6.1.0"))
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}

	usCoreURL := "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"
	if v.Registry().GetByURL(usCoreURL) == nil {
		t.Skip("US Core Patient profile not available")
	}

	// Patient missing required US Core fields (identifier, name, gender)
	minimalPatient := `{"resourceType": "Patient"}`

	t.Run("without per-call profile validates against core only", func(t *testing.T) {
		result, err := v.ValidateJSON(context.Background(), minimalPatient)
		if err != nil {
			t.Fatalf("ValidateJSON() returned error: %v", err)
		}
		// Core Patient has no required fields beyond resourceType, so no cardinality errors
		if result.Stats.IsCustomProfile {
			t.Error("Expected IsCustomProfile=false when no per-call profile")
		}
		t.Logf("Core-only: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
	})

	t.Run("with per-call profile validates against profile", func(t *testing.T) {
		result, err := v.ValidateJSON(context.Background(), minimalPatient,
			ValidateWithProfile(usCoreURL),
		)
		if err != nil {
			t.Fatalf("ValidateJSON() returned error: %v", err)
		}
		// US Core Patient requires identifier, name, gender — should produce errors
		if !result.Stats.IsCustomProfile {
			t.Error("Expected IsCustomProfile=true when per-call profile is set")
		}
		if result.Stats.ProfileURL != usCoreURL {
			t.Errorf("ProfileURL = %q, want %q", result.Stats.ProfileURL, usCoreURL)
		}
		if result.ErrorCount() == 0 {
			t.Error("Expected errors for missing US Core required fields, got none")
		}
		t.Logf("Per-call profile: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
		for _, iss := range result.Issues {
			t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
		}
	})

	t.Run("per-call profile does not affect subsequent calls", func(t *testing.T) {
		// First call with profile
		_, err := v.Validate(context.Background(), []byte(minimalPatient),
			ValidateWithProfile(usCoreURL),
		)
		if err != nil {
			t.Fatalf("Validate() returned error: %v", err)
		}

		// Second call without profile — should validate against core only
		result, err := v.Validate(context.Background(), []byte(minimalPatient))
		if err != nil {
			t.Fatalf("Validate() returned error: %v", err)
		}
		if result.Stats.IsCustomProfile {
			t.Error("Per-call profile leaked to subsequent call")
		}
	})
}
