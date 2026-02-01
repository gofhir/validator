package validator

import (
	"context"
	"os"
	"testing"
)

func TestReferenceValidation(t *testing.T) {
	v := getSharedValidator(t)

	tests := []struct {
		name           string
		file           string
		expectErrors   int
		expectWarnings int
	}{
		{
			name:           "valid-relative-reference",
			file:           "../../testdata/m9-references/valid-relative-reference.json",
			expectErrors:   0,
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "valid-absolute-reference",
			file:           "../../testdata/m9-references/valid-absolute-reference.json",
			expectErrors:   0,
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "valid-fragment-reference",
			file:           "../../testdata/m9-references/valid-fragment-reference.json",
			expectErrors:   0,
			expectWarnings: 2, // dom-6 for Patient and contained Organization (no narrative)
		},
		{
			name:           "valid-urn-uuid",
			file:           "../../testdata/m9-references/valid-urn-uuid.json",
			expectErrors:   0,
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "valid-logical-reference",
			file:           "../../testdata/m9-references/valid-logical-reference.json",
			expectErrors:   0,
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "invalid-format",
			file:           "../../testdata/m9-references/invalid-format.json",
			expectErrors:   1, // Invalid reference format
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "invalid-target-type",
			file:           "../../testdata/m9-references/invalid-target-type.json",
			expectErrors:   1, // Reference target type doesn't match targetProfile
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "invalid-type-mismatch",
			file:           "../../testdata/m9-references/invalid-type-mismatch.json",
			expectErrors:   1, // type element doesn't match reference
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "valid-bundle-uuid",
			file:           "../../testdata/m9-references/valid-bundle-uuid.json",
			expectErrors:   0,
			expectWarnings: 0, // Bundle doesn't have dom-6
		},
		{
			name:           "invalid-bundle-uuid-not-found",
			file:           "../../testdata/m9-references/invalid-bundle-uuid-not-found.json",
			expectErrors:   0, // HL7 validator emits WARNING for URN not found
			expectWarnings: 1, // UUID reference not locally contained within Bundle
		},
		{
			name:           "invalid-bundle-uuid-format",
			file:           "../../testdata/m9-references/invalid-bundle-uuid-format.json",
			expectErrors:   0, // HL7 validator doesn't validate UUID format strictly
			expectWarnings: 1, // URN reference not found in Bundle (like HL7)
		},
		{
			name:           "invalid-bundle-uuid-wrong-type",
			file:           "../../testdata/m9-references/invalid-bundle-uuid-wrong-type.json",
			expectErrors:   1, // Observation.subject references Organization (not allowed)
			expectWarnings: 0,
		},
		{
			name:           "valid-bundle-fullurl-id",
			file:           "../../testdata/m9-references/valid-bundle-fullurl-id.json",
			expectErrors:   0,
			expectWarnings: 0,
		},
		{
			name:           "invalid-bundle-fullurl-mismatch",
			file:           "../../testdata/m9-references/invalid-bundle-fullurl-mismatch.json",
			expectErrors:   1, // fullUrl doesn't match resource.id
			expectWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("Failed to read file: %v", err)
			}

			result, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}

			t.Logf("Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
			for _, issue := range result.Issues {
				t.Logf("  [%s] %s @ %v", issue.Severity, issue.Diagnostics, issue.Expression)
			}

			if result.ErrorCount() != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectErrors, result.ErrorCount())
			}
			if result.WarningCount() != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d", tt.expectWarnings, result.WarningCount())
			}
		})
	}
}

func TestInvalidUUIDFormats(t *testing.T) {
	v := getSharedValidator(t)

	data, err := os.ReadFile("../../testdata/m9-references/invalid-uuid-formats.json")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	result, err := v.Validate(context.Background(), data)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	t.Logf("Total Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())

	// HL7 validator behavior: does NOT validate UUID format strictly.
	// All urn:uuid: references are accepted as valid format.
	// Invalid UUIDs just get "not in bundle" warning (same as valid UUIDs not found).
	//
	// Test UUIDs (all accepted as valid format by HL7):
	// 1. urn:uuid:123 - too short (not in bundle)
	// 2. urn:uuid:61ebe359bfdc46138bf2c5e300945f0a - no dashes (not in bundle)
	// 3. urn:uuid:GGGGGGGG-GGGG-GGGG-GGGG-GGGGGGGGGGGG - invalid hex chars (not in bundle)
	// 4. urn:uuid:61ebe359-bfdc-4613-8bf2 - incomplete (not in bundle)
	// 5. urn:uuid:61ebe359-bfdc-4613-8bf2-c5e300945f0a-extra - extra segment (not in bundle)
	// 6. urn:uuid:61EBE359-BFDC-4613-8BF2-C5E300945F0A - uppercase valid (not in bundle)

	expectedInvalidFormats := 0 // HL7 doesn't validate UUID format
	expectedNotInBundle := 6    // All 6 references not found in Bundle

	invalidFormatCount := 0
	notInBundleCount := 0

	for _, issue := range result.Issues {
		t.Logf("  [%s] %s @ %v", issue.Severity, issue.Diagnostics, issue.Expression)

		if issue.MessageID == "REFERENCE_INVALID_FORMAT" {
			invalidFormatCount++
		}
		if issue.MessageID == "REFERENCE_NOT_IN_BUNDLE" {
			notInBundleCount++
		}
	}

	if invalidFormatCount != expectedInvalidFormats {
		t.Errorf("Expected %d invalid format errors, got %d", expectedInvalidFormats, invalidFormatCount)
	}

	if notInBundleCount != expectedNotInBundle {
		t.Errorf("Expected %d 'not in bundle' warnings, got %d", expectedNotInBundle, notInBundleCount)
	}
}
