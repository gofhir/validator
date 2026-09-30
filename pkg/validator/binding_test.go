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

// A CodeableConcept under a required binding must carry a code: text alone, or codings without
// one, report it (the HL7 validator's "No code provided, and a code is required from the value
// set"). AllergyIntolerance.clinicalStatus is bound required in R4.
func TestBindingRequiredNoCode(t *testing.T) {
	v := getSharedValidator(t)
	base := `{"resourceType":"AllergyIntolerance","id":"a","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},` +
		`"verificationStatus":{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/allergyintolerance-verification","code":"confirmed"}]},` +
		`"patient":{"reference":"Patient/p"},"clinicalStatus":`
	for _, tt := range []struct {
		name, status string
		want         int
	}{
		{"text only", `{"text":"active"}`, 1},
		{"coding without code", `{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/allergyintolerance-clinical"}]}`, 1},
		{"empty coding", `{"coding":[]}`, 1},
		{"code from the value set", `{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/allergyintolerance-clinical","code":"active"}]}`, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.Validate(context.Background(), []byte(base+tt.status+`}`))
			if err != nil {
				t.Fatal(err)
			}
			got := 0
			for _, is := range result.Issues {
				if is.MessageID == "BINDING_REQUIRED_NO_CODE" {
					got++
					if len(is.Expression) == 0 || is.Expression[0] != "AllergyIntolerance.clinicalStatus" {
						t.Errorf("reported at %v", is.Expression)
					}
				}
			}
			if got != tt.want {
				t.Errorf("BINDING_REQUIRED_NO_CODE x%d, want x%d (%v)", got, tt.want, result.Issues)
			}
		})
	}
}
