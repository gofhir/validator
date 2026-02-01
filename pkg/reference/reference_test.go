package reference

import (
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// mockRegistry creates a minimal registry for testing.
func mockRegistry() *registry.Registry {
	reg := registry.New()
	return reg
}

func TestExtractTypeFromProfile(t *testing.T) {
	v := &Validator{registry: mockRegistry()}

	tests := []struct {
		name     string
		profile  string
		expected string
	}{
		{
			name:     "standard Patient profile",
			profile:  "http://hl7.org/fhir/StructureDefinition/Patient",
			expected: "Patient",
		},
		{
			name:     "standard Organization profile",
			profile:  "http://hl7.org/fhir/StructureDefinition/Organization",
			expected: "Organization",
		},
		{
			name:     "standard Resource profile",
			profile:  "http://hl7.org/fhir/StructureDefinition/Resource",
			expected: "Resource",
		},
		{
			name:     "custom profile URL",
			profile:  "http://example.org/fhir/StructureDefinition/MyPatient",
			expected: "MyPatient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.extractTypeFromProfile(tt.profile)
			if result != tt.expected {
				t.Errorf("extractTypeFromProfile(%q) = %q, want %q", tt.profile, result, tt.expected)
			}
		})
	}
}

func TestGetTargetProfiles(t *testing.T) {
	v := &Validator{registry: mockRegistry()}

	tests := []struct {
		name     string
		elemDef  *registry.ElementDefinition
		expected []string
	}{
		{
			name: "single targetProfile",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code:          "Reference",
						TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"},
					},
				},
			},
			expected: []string{"http://hl7.org/fhir/StructureDefinition/Organization"},
		},
		{
			name: "multiple targetProfiles",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code: "Reference",
						TargetProfile: []string{
							"http://hl7.org/fhir/StructureDefinition/Patient",
							"http://hl7.org/fhir/StructureDefinition/Practitioner",
						},
					},
				},
			},
			expected: []string{
				"http://hl7.org/fhir/StructureDefinition/Patient",
				"http://hl7.org/fhir/StructureDefinition/Practitioner",
			},
		},
		{
			name: "no targetProfile (Reference Any)",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code:          "Reference",
						TargetProfile: nil,
					},
				},
			},
			expected: nil,
		},
		{
			name: "non-Reference type",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code: "string",
					},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.getTargetProfiles(tt.elemDef)
			if len(result) != len(tt.expected) {
				t.Errorf("getTargetProfiles() returned %d profiles, want %d", len(result), len(tt.expected))
				return
			}
			for i, profile := range result {
				if profile != tt.expected[i] {
					t.Errorf("getTargetProfiles()[%d] = %q, want %q", i, profile, tt.expected[i])
				}
			}
		})
	}
}

func TestTypeMatchesProfiles(t *testing.T) {
	v := &Validator{registry: mockRegistry()}

	tests := []struct {
		name         string
		resourceType string
		profiles     []string
		expected     bool
	}{
		{
			name:         "exact match",
			resourceType: "Organization",
			profiles:     []string{"http://hl7.org/fhir/StructureDefinition/Organization"},
			expected:     true,
		},
		{
			name:         "match one of multiple",
			resourceType: "Patient",
			profiles: []string{
				"http://hl7.org/fhir/StructureDefinition/Patient",
				"http://hl7.org/fhir/StructureDefinition/Practitioner",
			},
			expected: true,
		},
		{
			name:         "no match",
			resourceType: "Patient",
			profiles:     []string{"http://hl7.org/fhir/StructureDefinition/Organization"},
			expected:     false,
		},
		{
			name:         "Resource allows any",
			resourceType: "Patient",
			profiles:     []string{"http://hl7.org/fhir/StructureDefinition/Resource"},
			expected:     true,
		},
		{
			name:         "empty profiles allows any",
			resourceType: "Patient",
			profiles:     []string{},
			expected:     false, // Empty slice means no match (but caller should skip validation)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.typeMatchesProfiles(tt.resourceType, tt.profiles)
			if result != tt.expected {
				t.Errorf("typeMatchesProfiles(%q, %v) = %v, want %v",
					tt.resourceType, tt.profiles, result, tt.expected)
			}
		})
	}
}

func TestValidateTargetProfile(t *testing.T) {
	v := &Validator{registry: mockRegistry()}

	tests := []struct {
		name          string
		extractedType string
		refStr        string
		elemDef       *registry.ElementDefinition
		bundleCtx     *BundleContext
		expectError   bool
	}{
		{
			name:          "valid reference - Organization to Organization",
			extractedType: "Organization",
			refStr:        "Organization/123",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code:          "Reference",
						TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"},
					},
				},
			},
			expectError: false,
		},
		{
			name:          "invalid reference - Patient to Organization only",
			extractedType: "Patient",
			refStr:        "Patient/123",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code:          "Reference",
						TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"},
					},
				},
			},
			expectError: true,
		},
		{
			name:          "valid reference - Patient to Patient|Practitioner",
			extractedType: "Patient",
			refStr:        "Patient/123",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code: "Reference",
						TargetProfile: []string{
							"http://hl7.org/fhir/StructureDefinition/Patient",
							"http://hl7.org/fhir/StructureDefinition/Practitioner",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:          "no targetProfile - any type allowed",
			extractedType: "Patient",
			refStr:        "Patient/123",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code:          "Reference",
						TargetProfile: nil,
					},
				},
			},
			expectError: false,
		},
		{
			name:          "empty type - skip validation",
			extractedType: "",
			refStr:        "#contained",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code:          "Reference",
						TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"},
					},
				},
			},
			expectError: false,
		},
		{
			name:          "URN with Bundle context - valid",
			extractedType: "",
			refStr:        "urn:uuid:abc-123",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code:          "Reference",
						TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Patient"},
					},
				},
			},
			bundleCtx: &BundleContext{
				FullURLIndex: map[string]string{
					"urn:uuid:abc-123": "Patient",
				},
			},
			expectError: false,
		},
		{
			name:          "URN with Bundle context - invalid type",
			extractedType: "",
			refStr:        "urn:uuid:abc-123",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{
						Code:          "Reference",
						TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"},
					},
				},
			},
			bundleCtx: &BundleContext{
				FullURLIndex: map[string]string{
					"urn:uuid:abc-123": "Patient",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := issue.NewResult()
			v.validateTargetProfile(tt.extractedType, tt.refStr, tt.elemDef, "Test.reference", tt.bundleCtx, result)

			hasError := result.HasErrors()
			if hasError != tt.expectError {
				t.Errorf("validateTargetProfile() hasError = %v, want %v", hasError, tt.expectError)
				if hasError {
					for _, iss := range result.Issues {
						t.Logf("  Issue: %s", iss.Diagnostics)
					}
				}
			}
		})
	}
}

func TestExtractTypesFromProfiles(t *testing.T) {
	v := &Validator{registry: mockRegistry()}

	tests := []struct {
		name     string
		profiles []string
		expected []string
	}{
		{
			name: "single profile",
			profiles: []string{
				"http://hl7.org/fhir/StructureDefinition/Organization",
			},
			expected: []string{"Organization"},
		},
		{
			name: "multiple profiles",
			profiles: []string{
				"http://hl7.org/fhir/StructureDefinition/Patient",
				"http://hl7.org/fhir/StructureDefinition/Practitioner",
			},
			expected: []string{"Patient", "Practitioner"},
		},
		{
			name: "duplicate types removed",
			profiles: []string{
				"http://hl7.org/fhir/StructureDefinition/Patient",
				"http://example.org/StructureDefinition/Patient", // Same type, different profile
			},
			expected: []string{"Patient"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.extractTypesFromProfiles(tt.profiles)
			if len(result) != len(tt.expected) {
				t.Errorf("extractTypesFromProfiles() returned %d types, want %d", len(result), len(tt.expected))
				return
			}
			for i, typ := range result {
				if typ != tt.expected[i] {
					t.Errorf("extractTypesFromProfiles()[%d] = %q, want %q", i, typ, tt.expected[i])
				}
			}
		})
	}
}
