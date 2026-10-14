package reference

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/internal/bundleref"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
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
			rc := &refContext{bundle: tt.bundleCtx}
			target, targetType := v.targetOf(tt.refStr, rc)
			if tt.extractedType != "" {
				targetType = tt.extractedType
			}
			v.validateTargetProfile(targetType, target, tt.refStr, tt.elemDef, "Test.reference", rc, result)

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

// A reference is allowed by the aggregation modes of its element by where its target is found:
// "contained", among the referring resource's contained resources; "bundled", among the entries of
// the Bundle it is in; "referenced", anywhere outside it.
func TestValidateAggregation(t *testing.T) {
	v := &Validator{registry: mockRegistry()}
	patient := map[string]any{
		"resourceType": "Patient", "id": "123",
		"contained": []any{map[string]any{"resourceType": "Patient", "id": "pat-1"}},
	}
	bundle := map[string]any{
		"resourceType": "Bundle", "type": "collection",
		"entry": []any{
			map[string]any{"fullUrl": "http://ex.org/fhir/Patient/123", "resource": patient},
			map[string]any{"fullUrl": "urn:uuid:abc-123", "resource": map[string]any{"resourceType": "Patient", "id": "x"}},
		},
	}
	ctx := bundleref.EnsureIndexes(context.Background())
	inBundle := &refContext{ctx: ctx, contained: NewContainedContext(patient), root: patient,
		container: bundle, fullURL: "http://ex.org/fhir/Patient/123"}
	alone := &refContext{ctx: ctx, contained: NewContainedContext(patient), root: patient}
	modes := func(m ...string) *registry.ElementDefinition {
		return &registry.ElementDefinition{Type: []registry.Type{{Code: "Reference", Aggregation: m}}}
	}

	tests := []struct {
		name        string
		refStr      string
		elemDef     *registry.ElementDefinition
		rc          *refContext
		expectError bool
	}{
		{"no aggregation: any reference", "Patient/999", modes(), inBundle, false},
		{"contained: a contained resource", "#pat-1", modes("contained"), inBundle, false},
		{"contained: a fragment not found", "#nope", modes("contained"), inBundle, true},
		{"contained: a relative reference", "Organization/123", modes("contained"), inBundle, true},
		{"referenced: a relative reference", "Patient/999", modes("referenced"), alone, false},
		{"referenced: a fragment", "#pat-1", modes("referenced"), inBundle, true},
		{"bundled: a urn found in the Bundle", "urn:uuid:abc-123", modes("bundled"), inBundle, false},
		{"bundled: a relative reference found in the Bundle", "Patient/123", modes("bundled"), inBundle, false},
		{"bundled: a urn not in the Bundle", "urn:uuid:zzz", modes("bundled"), inBundle, true},
		{"bundled: a relative reference not in the Bundle", "Patient/999", modes("bundled"), inBundle, true},
		{"bundled: no Bundle", "Patient/123", modes("bundled"), alone, true},
		{"contained or referenced: a relative reference", "Patient/999", modes("contained", "referenced"), inBundle, false},
		{"contained or referenced: a fragment", "#pat-1", modes("contained", "referenced"), inBundle, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := issue.NewResult()
			v.validateAggregation(tt.refStr, tt.elemDef, "Test.ref", tt.rc, result)

			hasError := result.HasErrors()
			if hasError != tt.expectError {
				t.Errorf("validateAggregation(%q) hasError = %v, want %v", tt.refStr, hasError, tt.expectError)
				for _, iss := range result.Issues {
					t.Logf("  Issue: [%s] %s", iss.Severity, iss.Diagnostics)
				}
			}
		})
	}
}

func TestGetAggregationModes(t *testing.T) {
	v := &Validator{registry: mockRegistry()}

	tests := []struct {
		name     string
		elemDef  *registry.ElementDefinition
		expected []string
	}{
		{
			name: "no aggregation",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{{Code: "Reference"}},
			},
			expected: nil,
		},
		{
			name: "single mode",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{{Code: "Reference", Aggregation: []string{"contained"}}},
			},
			expected: []string{"contained"},
		},
		{
			name: "multiple modes",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{{Code: "Reference", Aggregation: []string{"contained", "referenced"}}},
			},
			expected: []string{"contained", "referenced"},
		},
		{
			name: "non-Reference type ignored",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{{Code: "string", Aggregation: []string{"contained"}}},
			},
			expected: nil,
		},
		{
			name: "deduplicates across multiple Reference types",
			elemDef: &registry.ElementDefinition{
				Type: []registry.Type{
					{Code: "Reference", Aggregation: []string{"contained"}},
					{Code: "Reference", Aggregation: []string{"contained", "referenced"}},
				},
			},
			expected: []string{"contained", "referenced"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.getAggregationModes(tt.elemDef)
			if len(result) != len(tt.expected) {
				t.Errorf("getAggregationModes() = %v, want %v", result, tt.expected)
				return
			}
			for i, mode := range result {
				if mode != tt.expected[i] {
					t.Errorf("getAggregationModes()[%d] = %q, want %q", i, mode, tt.expected[i])
				}
			}
		})
	}
}

func TestNewContainedContext(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]any
		wantIDs  map[string]string
	}{
		{
			name:     "no contained array",
			resource: map[string]any{"resourceType": "Patient"},
			wantIDs:  map[string]string{},
		},
		{
			name: "empty contained array",
			resource: map[string]any{
				"resourceType": "Observation",
				"contained":    []any{},
			},
			wantIDs: map[string]string{},
		},
		{
			name: "single contained resource",
			resource: map[string]any{
				"resourceType": "Observation",
				"contained": []any{
					map[string]any{"resourceType": "Patient", "id": "pat-1"},
				},
			},
			wantIDs: map[string]string{"pat-1": "Patient"},
		},
		{
			name: "multiple contained resources",
			resource: map[string]any{
				"resourceType": "Observation",
				"contained": []any{
					map[string]any{"resourceType": "Patient", "id": "pat-1"},
					map[string]any{"resourceType": "Organization", "id": "org-1"},
				},
			},
			wantIDs: map[string]string{
				"pat-1": "Patient",
				"org-1": "Organization",
			},
		},
		{
			name: "contained without id is skipped",
			resource: map[string]any{
				"resourceType": "Observation",
				"contained": []any{
					map[string]any{"resourceType": "Patient"},
				},
			},
			wantIDs: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContainedContext(tt.resource)
			if ctx == nil {
				t.Fatal("NewContainedContext returned nil")
			}
			if len(ctx.IDIndex) != len(tt.wantIDs) {
				t.Errorf("IDIndex has %d entries, want %d", len(ctx.IDIndex), len(tt.wantIDs))
			}
			for id, wantType := range tt.wantIDs {
				gotType, found := ctx.IDIndex[id]
				if !found {
					t.Errorf("IDIndex missing key %q", id)
				} else if gotType != wantType {
					t.Errorf("IDIndex[%q] = %q, want %q", id, gotType, wantType)
				}
			}
		})
	}
}
