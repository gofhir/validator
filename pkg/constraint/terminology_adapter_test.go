package constraint

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/terminology"
)

func setupTermRegistry(t *testing.T) *terminology.Registry {
	t.Helper()
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}
	termReg := terminology.NewRegistry()
	if err := termReg.LoadFromPackages(packages); err != nil {
		t.Fatalf("Failed to load terminology: %v", err)
	}
	return termReg
}

func TestFhirpathTermService_MemberOf(t *testing.T) {
	termReg := setupTermRegistry(t)
	svc := &fhirpathTermService{termRegistry: termReg}
	ctx := context.Background()

	tests := []struct {
		name     string
		code     any
		vsURL    string
		expected bool
	}{
		{
			name:     "code string in administrative-gender VS",
			code:     map[string]any{"code": "male"},
			vsURL:    "http://hl7.org/fhir/ValueSet/administrative-gender",
			expected: true,
		},
		{
			name:     "code string NOT in administrative-gender VS",
			code:     map[string]any{"code": "invalid-code-xyz"},
			vsURL:    "http://hl7.org/fhir/ValueSet/administrative-gender",
			expected: false,
		},
		{
			name: "coding in observation-status VS",
			code: map[string]any{
				"system": "http://hl7.org/fhir/observation-status",
				"code":   "final",
			},
			vsURL:    "http://hl7.org/fhir/ValueSet/observation-status",
			expected: true,
		},
		{
			name: "coding NOT in observation-status VS",
			code: map[string]any{
				"system": "http://hl7.org/fhir/observation-status",
				"code":   "nonexistent",
			},
			vsURL:    "http://hl7.org/fhir/ValueSet/observation-status",
			expected: false,
		},
		{
			name: "CodeableConcept with valid coding",
			code: map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://hl7.org/fhir/observation-status",
						"code":   "final",
					},
				},
			},
			vsURL:    "http://hl7.org/fhir/ValueSet/observation-status",
			expected: true,
		},
		{
			name: "CodeableConcept with no valid coding",
			code: map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://hl7.org/fhir/observation-status",
						"code":   "nonexistent",
					},
				},
			},
			vsURL:    "http://hl7.org/fhir/ValueSet/observation-status",
			expected: false,
		},
		{
			name:     "unknown ValueSet returns false",
			code:     map[string]any{"code": "male"},
			vsURL:    "http://example.org/nonexistent-vs",
			expected: false,
		},
		{
			name:     "nil code returns false",
			code:     "not-a-map",
			vsURL:    "http://hl7.org/fhir/ValueSet/administrative-gender",
			expected: false,
		},
		{
			name:     "empty code returns false",
			code:     map[string]any{},
			vsURL:    "http://hl7.org/fhir/ValueSet/administrative-gender",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.MemberOf(ctx, tt.code, tt.vsURL)
			if err != nil {
				t.Fatalf("MemberOf returned error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("MemberOf() = %v, want %v", result, tt.expected)
			}
		})
	}
}
