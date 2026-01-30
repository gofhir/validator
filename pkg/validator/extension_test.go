package validator

import (
	"context"
	"os"
	"testing"
)

func TestExtensionValidation(t *testing.T) {
	v := getSharedValidator(t)

	tests := []struct {
		name           string
		file           string
		expectErrors   int
		expectWarnings int
	}{
		{
			name:           "valid-patient-birthplace",
			file:           "../../testdata/m8-extensions/valid-patient-birthplace.json",
			expectErrors:   0,
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "unknown-extension-warning",
			file:           "../../testdata/m8-extensions/warning-unknown-extension.json",
			expectErrors:   0,
			expectWarnings: 2, // Unknown extension + dom-6 (no narrative)
		},
		{
			name:           "invalid-wrong-value-type",
			file:           "../../testdata/m8-extensions/invalid-wrong-value-type.json",
			expectErrors:   1, // Wrong value type is an error
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "invalid-wrong-context",
			file:           "../../testdata/m8-extensions/invalid-wrong-context.json",
			expectErrors:   1, // Wrong context is an error
			expectWarnings: 1, // dom-6 (no narrative)
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
