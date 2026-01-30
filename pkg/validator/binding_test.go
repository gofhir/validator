package validator

import (
	"context"
	"os"
	"testing"
)

func TestBindingValidation(t *testing.T) {
	v := getSharedValidator(t)

	tests := []struct {
		name           string
		file           string
		expectErrors   int
		expectWarnings int
	}{
		{
			name:           "text-only-extensible-binding",
			file:           "../../testdata/m7-bindings/valid-codeableconcept-text-only.json",
			expectErrors:   0,
			expectWarnings: 2, // Text-only warning + dom-6 (no narrative)
		},
		{
			name:           "display-mismatch",
			file:           "../../testdata/m7-bindings/invalid-display-mismatch.json",
			expectErrors:   1, // Display mismatch is an error
			expectWarnings: 1, // dom-6 (no narrative)
		},
		{
			name:           "display-correct",
			file:           "../../testdata/m7-bindings/valid-display-correct.json",
			expectErrors:   0,
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
