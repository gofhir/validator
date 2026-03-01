package validator

import (
	"context"
	"os"
	"testing"
)

func TestConstraintValidation(t *testing.T) {
	v := getSharedValidator(t)

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
		// Nested element constraint tests (pat-1 on Patient.contact).
		{
			name:           "invalid-patient-contact-no-details",
			file:           "../../testdata/m10-constraints/invalid-patient-contact-no-details.json",
			expectErrors:   1, // pat-1: contact has no name, telecom, address, or organization
			expectWarnings: 1, // extensible binding on contact.relationship
		},
		{
			name:           "valid-patient-contact-with-name",
			file:           "../../testdata/m10-constraints/valid-patient-contact-with-name.json",
			expectErrors:   0,
			expectWarnings: 1, // extensible binding on contact.relationship
		},
		{
			name:           "invalid-patient-contact-mixed",
			file:           "../../testdata/m10-constraints/invalid-patient-contact-mixed.json",
			expectErrors:   1, // pat-1: second contact has no name/telecom/address/organization
			expectWarnings: 1, // extensible binding on contact[1].relationship
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
