package validator

import (
	"context"
	"os"
	"testing"
)

func TestConstraintValidation(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}

	tests := []struct {
		name           string
		file           string
		expectErrors   int
		expectWarnings int
	}{
		{
			name:           "valid-patient-no-contained-meta",
			file:           "../../testdata/m10-constraints/valid-patient-no-contained-meta.json",
			expectErrors:   0,
			expectWarnings: 2, // dom-6 for Patient and contained Organization (no narrative)
		},
		{
			name:           "invalid-contained-has-versionid",
			file:           "../../testdata/m10-constraints/invalid-contained-has-versionid.json",
			expectErrors:   1, // dom-4 violation
			expectWarnings: 2, // dom-6 for Patient and contained Organization (no narrative)
		},
		{
			name:           "invalid-contained-has-lastupdated",
			file:           "../../testdata/m10-constraints/invalid-contained-has-lastupdated.json",
			expectErrors:   1, // dom-4 violation
			expectWarnings: 2, // dom-6 for Patient and contained Organization (no narrative)
		},
		{
			name:           "valid-patient-with-organization-contact",
			file:           "../../testdata/m10-constraints/valid-patient-with-organization-contact.json",
			expectErrors:   0,
			expectWarnings: 2, // dom-6 for Patient and contained Organization (no narrative)
		},
		{
			name:           "invalid-contained-bad-telecom",
			file:           "../../testdata/m10-constraints/invalid-contained-bad-telecom.json",
			expectErrors:   1, // Invalid telecom.system binding in contained Organization
			expectWarnings: 2, // dom-6 for Patient and contained Organization (no narrative)
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
