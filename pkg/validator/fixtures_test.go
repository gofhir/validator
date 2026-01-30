package validator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestM0Fixtures(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m0-infrastructure"

	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		{
			name:        "valid-patient-minimal",
			file:        "valid-patient-minimal.json",
			expectError: false,
		},
	}

	runFixtureTests(t, v, fixturesDir, tests)
}

func TestM1Fixtures(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m1-structural"

	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		{
			name:        "valid-patient-with-name",
			file:        "valid-patient-with-name.json",
			expectError: false,
		},
		{
			name:        "valid-patient-choice-types",
			file:        "valid-patient-choice-types.json",
			expectError: false,
		},
		{
			name:        "invalid-patient-unknown-element",
			file:        "invalid-patient-unknown-element.json",
			expectError: true,
		},
		{
			name:        "invalid-patient-bad-choice-type",
			file:        "invalid-patient-bad-choice-type.json",
			expectError: true,
		},
		{
			name:        "invalid-patient-unknown-nested",
			file:        "invalid-patient-unknown-nested.json",
			expectError: true,
		},
	}

	runFixtureTests(t, v, fixturesDir, tests)
}

func TestM2Fixtures(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m2-cardinality"

	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		{
			name:        "valid-observation-with-required",
			file:        "valid-observation-with-required.json",
			expectError: false,
		},
		{
			name:        "invalid-observation-missing-status",
			file:        "invalid-observation-missing-status.json",
			expectError: true,
		},
		{
			name:        "invalid-patient-communication-no-language",
			file:        "invalid-patient-communication-no-language.json",
			expectError: true,
		},
		{
			name:        "valid-patient-with-link",
			file:        "valid-patient-with-link.json",
			expectError: false,
		},
	}

	runFixtureTests(t, v, fixturesDir, tests)
}

func TestM3Fixtures(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m3-primitive"

	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		{
			name:        "valid-patient-types",
			file:        "valid-patient-types.json",
			expectError: false,
		},
		{
			name:        "invalid-patient-wrong-type-boolean",
			file:        "invalid-patient-wrong-type-boolean.json",
			expectError: true,
		},
		{
			name:        "invalid-patient-wrong-type-string",
			file:        "invalid-patient-wrong-type-string.json",
			expectError: true,
		},
		{
			name:        "invalid-patient-bad-date",
			file:        "invalid-patient-bad-date.json",
			expectError: true,
		},
		// Note: Patient.id is typed as "string" not "id" in R4 SD (ADR-015)
		// So invalid id format won't be detected at Patient.id level
		{
			name:        "invalid-patient-id-format",
			file:        "invalid-patient-id-format.json",
			expectError: false, // Patient.id is string type, not id type
		},
		{
			name:        "invalid-patient-uri-whitespace",
			file:        "invalid-patient-uri-whitespace.json",
			expectError: true,
		},
	}

	runFixtureTests(t, v, fixturesDir, tests)
}

func TestM4Fixtures(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m4-complex"

	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		{
			name:        "valid-patient-humanname",
			file:        "valid-patient-humanname.json",
			expectError: false,
		},
		{
			name:        "invalid-humanname-unknown-element",
			file:        "invalid-humanname-unknown-element.json",
			expectError: true,
		},
		{
			name:        "invalid-period-wrong-type",
			file:        "invalid-period-wrong-type.json",
			expectError: true,
		},
		{
			name:        "valid-patient-identifier",
			file:        "valid-patient-identifier.json",
			expectError: false,
		},
		{
			name:        "valid-observation-quantity",
			file:        "valid-observation-quantity.json",
			expectError: false,
		},
		{
			name:        "invalid-quantity-unknown-element",
			file:        "invalid-quantity-unknown-element.json",
			expectError: true,
		},
		// Mixed arrays: some elements valid, some invalid
		{
			name:        "mixed-array-humanname",
			file:        "mixed-array-humanname.json",
			expectError: true, // Has 3 unknown elements across 2 array entries
		},
		{
			name:        "mixed-array-identifier",
			file:        "mixed-array-identifier.json",
			expectError: true, // Has 3 unknown elements across 2 array entries
		},
		{
			name:        "mixed-array-coding",
			file:        "mixed-array-coding.json",
			expectError: true, // Has 3 unknown elements across 2 coding entries
		},
		{
			name:        "nested-mixed-errors",
			file:        "nested-mixed-errors.json",
			expectError: true, // Errors at multiple nesting levels
		},
		// BackboneElement arrays
		{
			name:        "mixed-array-patient-contact",
			file:        "mixed-array-patient-contact.json",
			expectError: true,
		},
		{
			name:        "mixed-array-patient-communication",
			file:        "mixed-array-patient-communication.json",
			expectError: true,
		},
		{
			name:        "mixed-array-observation-component",
			file:        "mixed-array-observation-component.json",
			expectError: true,
		},
		{
			name:        "mixed-array-patient-link",
			file:        "mixed-array-patient-link.json",
			expectError: true,
		},
	}

	runFixtureTests(t, v, fixturesDir, tests)
}

func TestM5Fixtures(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m5-coding"

	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		{
			name:        "valid-coding-complete",
			file:        "valid-coding-complete.json",
			expectError: false,
		},
		{
			name:        "valid-coding-minimal",
			file:        "valid-coding-minimal.json",
			expectError: false,
		},
		{
			name:        "invalid-coding-unknown-element",
			file:        "invalid-coding-unknown-element.json",
			expectError: true,
		},
		{
			name:        "invalid-coding-wrong-type-system",
			file:        "invalid-coding-wrong-type-system.json",
			expectError: true,
		},
		{
			name:        "invalid-coding-wrong-type-userselected",
			file:        "invalid-coding-wrong-type-userselected.json",
			expectError: true,
		},
	}

	runFixtureTests(t, v, fixturesDir, tests)
}

func TestM6Fixtures(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m6-codeableconcept"

	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		{
			name:        "valid-codeableconcept-full",
			file:        "valid-codeableconcept-full.json",
			expectError: false,
		},
		{
			name:        "valid-codeableconcept-text-only",
			file:        "valid-codeableconcept-text-only.json",
			expectError: false,
		},
		{
			name:        "invalid-codeableconcept-unknown-element",
			file:        "invalid-codeableconcept-unknown-element.json",
			expectError: true,
		},
		{
			name:        "invalid-codeableconcept-wrong-type-text",
			file:        "invalid-codeableconcept-wrong-type-text.json",
			expectError: true,
		},
		{
			name:        "invalid-codeableconcept-mixed-coding-errors",
			file:        "invalid-codeableconcept-mixed-coding-errors.json",
			expectError: true,
		},
	}

	runFixtureTests(t, v, fixturesDir, tests)
}

func TestM7Fixtures(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m7-bindings"

	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		// Required bindings (Patient.gender)
		{
			name:        "valid-patient-gender",
			file:        "valid-patient-gender.json",
			expectError: false,
		},
		{
			name:        "valid-patient-all-genders",
			file:        "valid-patient-all-genders.json",
			expectError: false,
		},
		{
			name:        "invalid-patient-gender",
			file:        "invalid-patient-gender.json",
			expectError: true, // Required binding violated
		},
		// Required bindings (Observation.status)
		{
			name:        "valid-observation-status",
			file:        "valid-observation-status.json",
			expectError: false,
		},
		{
			name:        "invalid-observation-status",
			file:        "invalid-observation-status.json",
			expectError: true, // Required binding violated
		},
		// Required bindings in complex types (Identifier.use)
		{
			name:        "valid-identifier-use",
			file:        "valid-identifier-use.json",
			expectError: false,
		},
		{
			name:        "invalid-identifier-use",
			file:        "invalid-identifier-use.json",
			expectError: true, // Required binding violated
		},
	}

	runFixtureTests(t, v, fixturesDir, tests)
}

// TestMixedArrayValidation demonstrates how the validator handles arrays
// with mixed valid/invalid elements - verifying recursive validation.
func TestMixedArrayValidation(t *testing.T) {
	v := getSharedValidator(t)

	fixturesDir := "../../testdata/m4-complex"

	mixedArrayTests := []struct {
		name          string
		file          string
		expectedCount int
		description   string
	}{
		{
			name:          "mixed-array-humanname",
			file:          "mixed-array-humanname.json",
			expectedCount: 3, // unknownField, badElement, alsoInvalid
			description:   "Patient.name array with 4 elements: 2 valid, 2 with unknown fields",
		},
		{
			name:          "mixed-array-identifier",
			file:          "mixed-array-identifier.json",
			expectedCount: 3, // invalidProp, unknownElement, anotherBad
			description:   "Patient.identifier array with 4 elements: 2 valid, 2 with unknown fields",
		},
		{
			name:          "mixed-array-coding",
			file:          "mixed-array-coding.json",
			expectedCount: 3, // badField, invalidElement, alsoInvalid
			description:   "Observation.code.coding array with 4 elements: 2 valid, 2 with unknown fields",
		},
		{
			name:          "nested-mixed-errors",
			file:          "nested-mixed-errors.json",
			expectedCount: 2, // unknownInCoding (deep), badInPeriod (nested)
			description:   "Multiple nesting levels: error in Identifier.type.coding and HumanName.period",
		},
		// BackboneElement arrays
		{
			name:          "mixed-array-patient-contact",
			file:          "mixed-array-patient-contact.json",
			expectedCount: 3, // unknownContactField, invalidElement, anotherBadField
			description:   "Patient.contact BackboneElement array: 4 elements, 2 with unknown fields",
		},
		{
			name:          "mixed-array-patient-communication",
			file:          "mixed-array-patient-communication.json",
			expectedCount: 3, // badField, unknownElement, alsoInvalid
			description:   "Patient.communication BackboneElement array: 4 elements, 2 with unknown fields",
		},
		{
			name:          "mixed-array-observation-component",
			file:          "mixed-array-observation-component.json",
			expectedCount: 4, // invalidComponentField, unknownElement, badField, anotherInvalid
			description:   "Observation.component BackboneElement array: 4 elements, 2 with unknown fields",
		},
		{
			name:          "mixed-array-patient-link",
			file:          "mixed-array-patient-link.json",
			expectedCount: 3, // invalidLinkField, unknownElement, badField
			description:   "Patient.link BackboneElement array: 4 elements, 2 with unknown fields",
		},
	}

	for _, tt := range mixedArrayTests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(fixturesDir, tt.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read fixture %s: %v", tt.file, err)
			}

			result, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}

			t.Logf("\n=== %s ===", tt.description)
			t.Logf("Errors found: %d (expected: %d)", result.ErrorCount(), tt.expectedCount)

			// Show each error with its path
			for i, issue := range result.Issues {
				if issue.Severity == "error" {
					t.Logf("  [%d] %s", i+1, issue.Expression)
					t.Logf("      %s", issue.Diagnostics)
				}
			}

			if result.ErrorCount() != tt.expectedCount {
				t.Errorf("Expected %d errors but got %d", tt.expectedCount, result.ErrorCount())
			}
		})
	}
}

// runFixtureTests runs a set of fixture tests against the validator.
func runFixtureTests(t *testing.T, v *Validator, fixturesDir string, tests []struct {
	name        string
	file        string
	expectError bool
}) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(fixturesDir, tt.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read fixture %s: %v", tt.file, err)
			}

			result, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}

			if tt.expectError && !result.HasErrors() {
				t.Errorf("Expected errors but got none")
			}
			if !tt.expectError && result.HasErrors() {
				t.Errorf("Expected no errors but got %d:", result.ErrorCount())
				for _, issue := range result.Issues {
					t.Logf("  - [%s] %s @ %v", issue.Severity, issue.Diagnostics, issue.Expression)
				}
			}

			t.Logf("Result: %d errors, %d warnings, phases=%d",
				result.ErrorCount(), result.WarningCount(), result.Stats.PhasesRun)
			if result.Stats != nil {
				profileType := "core"
				if result.Stats.IsCustomProfile {
					profileType = "custom"
				}
				t.Logf("Stats: resourceType=%s, size=%d bytes, profile=%s (%s), duration=%.3fms",
					result.Stats.ResourceType,
					result.Stats.ResourceSize,
					profileType,
					result.Stats.ProfileURL,
					result.Stats.DurationMs(),
				)
			}

			// Log issues for debugging
			if len(result.Issues) > 0 {
				t.Logf("Issues:")
				for _, issue := range result.Issues {
					t.Logf("  - [%s] %s @ %v", issue.Severity, issue.Diagnostics, issue.Expression)
				}
			}
		})
	}
}
