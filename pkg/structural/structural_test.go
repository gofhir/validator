package structural

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
)

var (
	sharedOnce     sync.Once
	sharedRegistry *registry.Registry
	errSharedLoad  error
)

func setupTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()

	sharedOnce.Do(func() {
		l := loader.NewLoader("")
		packages, err := l.LoadVersion("4.0.1")
		if err != nil {
			errSharedLoad = err
			return
		}

		reg := registry.New()
		if err := reg.LoadFromPackages(packages); err != nil {
			errSharedLoad = err
			return
		}
		sharedRegistry = reg
	})

	if errSharedLoad != nil {
		t.Skipf("Cannot load FHIR packages: %v", errSharedLoad)
	}

	return sharedRegistry
}

func TestValidateValidPatient(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	if sd == nil {
		t.Fatal("Patient StructureDefinition not found")
	}

	// Valid minimal patient
	resource := []byte(`{
		"resourceType": "Patient",
		"id": "123",
		"active": true
	}`)

	result := v.Validate(resource, sd)

	if result.HasErrors() {
		t.Errorf("Expected no errors for valid patient, got %d:", result.ErrorCount())
		for _, iss := range result.Issues {
			t.Logf("  - [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
		}
	}
}

func TestValidateUnknownElement(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	if sd == nil {
		t.Fatal("Patient StructureDefinition not found")
	}

	// Patient with unknown element
	resource := []byte(`{
		"resourceType": "Patient",
		"id": "123",
		"unknownElement": "value"
	}`)

	result := v.Validate(resource, sd)

	if !result.HasErrors() {
		t.Error("Expected error for unknown element, got none")
	}

	// Check that the error mentions the unknown element
	found := false
	for _, iss := range result.Issues {
		if iss.Diagnostics == "Unknown element 'unknownElement'" {
			found = true
			if len(iss.Expression) == 0 || iss.Expression[0] != "Patient.unknownElement" {
				t.Errorf("Expected expression 'Patient.unknownElement', got %v", iss.Expression)
			}
			break
		}
	}
	if !found {
		t.Error("Expected 'Unknown element' error not found")
		for _, iss := range result.Issues {
			t.Logf("  - [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
		}
	}
}

func TestValidateChoiceType(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	if sd == nil {
		t.Fatal("Patient StructureDefinition not found")
	}

	tests := []struct {
		name        string
		resource    string
		expectError bool
	}{
		{
			name: "valid deceasedBoolean",
			resource: `{
				"resourceType": "Patient",
				"deceasedBoolean": false
			}`,
			expectError: false,
		},
		{
			name: "valid deceasedDateTime",
			resource: `{
				"resourceType": "Patient",
				"deceasedDateTime": "2020-01-01"
			}`,
			expectError: false,
		},
		{
			name: "valid multipleBirthBoolean",
			resource: `{
				"resourceType": "Patient",
				"multipleBirthBoolean": true
			}`,
			expectError: false,
		},
		{
			name: "valid multipleBirthInteger",
			resource: `{
				"resourceType": "Patient",
				"multipleBirthInteger": 2
			}`,
			expectError: false,
		},
		{
			name: "invalid choice type - deceasedString not allowed",
			resource: `{
				"resourceType": "Patient",
				"deceasedString": "yes"
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate([]byte(tt.resource), sd)

			if tt.expectError && !result.HasErrors() {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && result.HasErrors() {
				t.Errorf("Expected no error but got %d:", result.ErrorCount())
				for _, iss := range result.Issues {
					t.Logf("  - [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
				}
			}
		})
	}
}

func TestValidateNestedElements(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	if sd == nil {
		t.Fatal("Patient StructureDefinition not found")
	}

	tests := []struct {
		name        string
		resource    string
		expectError bool
		errorPath   string
	}{
		{
			name: "valid nested HumanName",
			resource: `{
				"resourceType": "Patient",
				"name": [{"family": "Smith", "given": ["John"]}]
			}`,
			expectError: false,
		},
		{
			name: "unknown element in HumanName",
			resource: `{
				"resourceType": "Patient",
				"name": [{"family": "Smith", "unknownField": "value"}]
			}`,
			expectError: true,
			errorPath:   "Patient.name[0].unknownField",
		},
		{
			name: "valid nested Identifier",
			resource: `{
				"resourceType": "Patient",
				"identifier": [{"system": "http://example.org", "value": "123"}]
			}`,
			expectError: false,
		},
		{
			name: "unknown element in Identifier",
			resource: `{
				"resourceType": "Patient",
				"identifier": [{"system": "http://example.org", "badField": "123"}]
			}`,
			expectError: true,
			errorPath:   "Patient.identifier[0].badField",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate([]byte(tt.resource), sd)

			if tt.expectError {
				if !result.HasErrors() {
					t.Error("Expected error but got none")
					return
				}
				// Check for expected path
				if tt.errorPath != "" {
					found := false
					for _, iss := range result.Issues {
						if len(iss.Expression) > 0 && iss.Expression[0] == tt.errorPath {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error at path '%s' not found", tt.errorPath)
						for _, iss := range result.Issues {
							t.Logf("  - [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
						}
					}
				}
			} else if result.HasErrors() {
				t.Errorf("Expected no error but got %d:", result.ErrorCount())
				for _, iss := range result.Issues {
					t.Logf("  - [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
				}
			}
		})
	}
}

func TestFindMatchingChoiceType(t *testing.T) {
	// Create a mock ElementDefinition with types
	elemDef := &registry.ElementDefinition{
		Type: []registry.Type{
			{Code: "boolean"},
			{Code: "dateTime"},
			{Code: "CodeableConcept"},
			{Code: "Quantity"},
		},
	}

	tests := []struct {
		suffix   string
		expected string
	}{
		{"Boolean", "boolean"},   // Case insensitive match
		{"boolean", "boolean"},   // Exact match
		{"DateTime", "dateTime"}, // Case insensitive
		{"dateTime", "dateTime"}, // Exact
		{"CodeableConcept", "CodeableConcept"},
		{"codeableconcept", "CodeableConcept"}, // Case insensitive
		{"Quantity", "Quantity"},
		{"String", ""},  // Not in types
		{"Invalid", ""}, // Not in types
	}

	for _, tt := range tests {
		t.Run(tt.suffix, func(t *testing.T) {
			result := findMatchingChoiceType(elemDef, tt.suffix)
			if result != tt.expected {
				t.Errorf("findMatchingChoiceType(%q) = %q, want %q", tt.suffix, result, tt.expected)
			}
		})
	}
}

func TestValidateObservationChoiceTypes(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Observation")
	if sd == nil {
		t.Skip("Observation StructureDefinition not found")
	}

	tests := []struct {
		name        string
		resource    string
		expectError bool
	}{
		{
			name: "valid valueQuantity",
			resource: `{
				"resourceType": "Observation",
				"status": "final",
				"code": {"text": "test"},
				"valueQuantity": {"value": 100, "unit": "mg"}
			}`,
			expectError: false,
		},
		{
			name: "valid valueString",
			resource: `{
				"resourceType": "Observation",
				"status": "final",
				"code": {"text": "test"},
				"valueString": "positive"
			}`,
			expectError: false,
		},
		{
			name: "valid valueCodeableConcept",
			resource: `{
				"resourceType": "Observation",
				"status": "final",
				"code": {"text": "test"},
				"valueCodeableConcept": {"text": "Normal"}
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate([]byte(tt.resource), sd)

			if tt.expectError && !result.HasErrors() {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && result.HasErrors() {
				t.Errorf("Expected no error but got %d:", result.ErrorCount())
				for _, iss := range result.Issues {
					t.Logf("  - [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
				}
			}
		})
	}
}

// TestValidateChoiceTypeExclusivityDeterministic reproduces issue #59:
// matchChoiceType ignored the element-level scope, so on a resource with two
// choice types sharing the same baseName at different levels (Observation
// value[x] vs component.value[x]), the mutual-exclusion detection was
// non-deterministic. Running the same validation many times must ALWAYS flag
// the violation; before the fix, some iterations missed it due to Go's
// randomized map iteration order.
func TestValidateChoiceTypeExclusivityDeterministic(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Observation")
	if sd == nil {
		t.Skip("Observation StructureDefinition not found")
	}

	// Two variants of value[x] present at the root -> must always be rejected.
	resource := []byte(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"text": "test"},
		"valueQuantity": {"value": 42, "unit": "mg"},
		"valueString": "forty-two"
	}`)

	const iterations = 500
	for i := range iterations {
		result := v.Validate(resource, sd)
		if !hasMutualExclusionError(result) {
			t.Fatalf("iteration %d: expected choice-type mutual-exclusion error, got none. Issues:\n%s",
				i, formatIssues(result))
		}
	}
}

// TestValidateChoiceTypeComponentScope ensures a value[x] inside component is
// resolved against component.value[x] and a single root value[x] is still
// accepted (no false positive from cross-level baseName collisions).
func TestValidateChoiceTypeComponentScope(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Observation")
	if sd == nil {
		t.Skip("Observation StructureDefinition not found")
	}

	// Single value[x] at root AND a single value[x] inside a component: valid.
	resource := []byte(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"text": "test"},
		"valueString": "positive",
		"component": [
			{
				"code": {"text": "sub"},
				"valueQuantity": {"value": 1, "unit": "mg"}
			}
		]
	}`)

	const iterations = 500
	for i := range iterations {
		result := v.Validate(resource, sd)
		if hasMutualExclusionError(result) {
			t.Fatalf("iteration %d: unexpected mutual-exclusion error for single value[x] per level. Issues:\n%s",
				i, formatIssues(result))
		}
	}

	// Two variants inside component -> must always be rejected.
	bad := []byte(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"text": "test"},
		"component": [
			{
				"code": {"text": "sub"},
				"valueQuantity": {"value": 1, "unit": "mg"},
				"valueString": "one"
			}
		]
	}`)
	for i := range iterations {
		result := v.Validate(bad, sd)
		if !hasMutualExclusionError(result) {
			t.Fatalf("iteration %d: expected mutual-exclusion error for two component value[x], got none. Issues:\n%s",
				i, formatIssues(result))
		}
	}
}

func hasMutualExclusionError(result *issue.Result) bool {
	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagStructureChoiceMutualExclusion) {
			return true
		}
	}
	return false
}

func formatIssues(result *issue.Result) string {
	var b strings.Builder
	for _, iss := range result.Issues {
		fmt.Fprintf(&b, "  - [%s] %s @ %v\n", iss.Severity, iss.Diagnostics, iss.Expression)
	}
	return b.String()
}

func TestValidateBundleEntryResource(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Bundle")
	if sd == nil {
		t.Skip("Bundle StructureDefinition not found")
	}

	tests := []struct {
		name        string
		resource    string
		expectError bool
		errorPath   string // Path where error should occur if expected
	}{
		{
			name: "valid Bundle with Patient entry",
			resource: `{
				"resourceType": "Bundle",
				"type": "collection",
				"entry": [
					{
						"resource": {
							"resourceType": "Patient",
							"id": "123",
							"active": true,
							"name": [{"family": "Smith", "given": ["John"]}]
						}
					}
				]
			}`,
			expectError: false,
		},
		{
			name: "valid Bundle with Observation entry",
			resource: `{
				"resourceType": "Bundle",
				"type": "collection",
				"entry": [
					{
						"resource": {
							"resourceType": "Observation",
							"status": "final",
							"code": {"text": "test"},
							"valueString": "positive"
						}
					}
				]
			}`,
			expectError: false,
		},
		{
			name: "Bundle entry with unknown element in embedded resource",
			resource: `{
				"resourceType": "Bundle",
				"type": "collection",
				"entry": [
					{
						"resource": {
							"resourceType": "Patient",
							"unknownElement": "value"
						}
					}
				]
			}`,
			expectError: true,
			errorPath:   "Bundle.entry[0].resource.unknownElement",
		},
		{
			name: "Bundle entry with multiple resources",
			resource: `{
				"resourceType": "Bundle",
				"type": "collection",
				"entry": [
					{
						"resource": {
							"resourceType": "Patient",
							"active": true
						}
					},
					{
						"resource": {
							"resourceType": "Observation",
							"status": "final",
							"code": {"text": "test"}
						}
					}
				]
			}`,
			expectError: false,
		},
		{
			name: "valid Bundle with ServiceRequest entry",
			resource: `{
				"resourceType": "Bundle",
				"type": "collection",
				"entry": [
					{
						"resource": {
							"resourceType": "ServiceRequest",
							"status": "active",
							"intent": "order",
							"priority": "routine",
							"subject": {"reference": "Patient/123"}
						}
					}
				]
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate([]byte(tt.resource), sd)

			if tt.expectError {
				if !result.HasErrors() {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorPath != "" {
					found := false
					for _, iss := range result.Issues {
						if len(iss.Expression) > 0 && iss.Expression[0] == tt.errorPath {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error at path '%s' not found", tt.errorPath)
						for _, iss := range result.Issues {
							t.Logf("  - [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
						}
					}
				}
			} else if result.HasErrors() {
				t.Errorf("Expected no error but got %d:", result.ErrorCount())
				for _, iss := range result.Issues {
					t.Logf("  - [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
				}
			}
		})
	}
}
