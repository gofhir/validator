package cardinality

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
)

var (
	sharedRegistry     *registry.Registry
	errSharedRegistry  error
	sharedRegistryOnce sync.Once
)

func initSharedRegistry() {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		errSharedRegistry = fmt.Errorf("cannot load FHIR packages: %w", err)
		return
	}

	reg := registry.New()
	if err := reg.LoadFromPackages(packages); err != nil {
		errSharedRegistry = fmt.Errorf("failed to load registry: %w", err)
		return
	}

	sharedRegistry = reg
}

func setupTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()

	sharedRegistryOnce.Do(initSharedRegistry)

	if errSharedRegistry != nil {
		t.Skipf("%v", errSharedRegistry)
	}

	return sharedRegistry
}

func TestValidateObservationRequiredElements(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Observation")
	if sd == nil {
		t.Fatal("Observation StructureDefinition not found")
	}

	tests := []struct {
		name        string
		resource    string
		expectError bool
		errorPath   string
	}{
		{
			name: "valid observation with required elements",
			resource: `{
				"resourceType": "Observation",
				"status": "final",
				"code": {"text": "Blood pressure"}
			}`,
			expectError: false,
		},
		{
			name: "missing status",
			resource: `{
				"resourceType": "Observation",
				"code": {"text": "Blood pressure"}
			}`,
			expectError: true,
			errorPath:   "Observation.status",
		},
		{
			name: "missing code",
			resource: `{
				"resourceType": "Observation",
				"status": "final"
			}`,
			expectError: true,
			errorPath:   "Observation.code",
		},
		{
			name: "missing both status and code",
			resource: `{
				"resourceType": "Observation"
			}`,
			expectError: true,
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

func TestValidatePatientCommunicationRequired(t *testing.T) {
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
			name: "patient without communication is valid",
			resource: `{
				"resourceType": "Patient",
				"name": [{"family": "Smith"}]
			}`,
			expectError: false,
		},
		{
			name: "patient with communication but missing language",
			resource: `{
				"resourceType": "Patient",
				"communication": [{"preferred": true}]
			}`,
			expectError: true,
			errorPath:   "Patient.communication[0].language",
		},
		{
			name: "patient with valid communication",
			resource: `{
				"resourceType": "Patient",
				"communication": [{"language": {"text": "English"}}]
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

func TestValidateMaxCardinality(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	// Patient.active has max=1
	sd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	if sd == nil {
		t.Fatal("Patient StructureDefinition not found")
	}

	// Note: max cardinality for non-array elements is enforced by JSON structure
	// For array elements, we need to check the count
	// Patient.name has max="*", so let's use Patient.multipleBirth[x] which is max=1

	tests := []struct {
		name        string
		resource    string
		expectError bool
	}{
		{
			name: "single active value is valid",
			resource: `{
				"resourceType": "Patient",
				"active": true
			}`,
			expectError: false,
		},
		{
			name: "multiple names are valid (max=*)",
			resource: `{
				"resourceType": "Patient",
				"name": [
					{"family": "Smith"},
					{"family": "Jones"},
					{"family": "Williams"}
				]
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

func TestValidateMaxZeroProhibited(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	// Get the Patient SD and create a modified copy where photo has max="0" (prohibited).
	baseSd := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	if baseSd == nil {
		t.Fatal("Patient StructureDefinition not found")
	}

	// Deep-copy snapshot elements so we don't mutate the registry
	elements := make([]registry.ElementDefinition, len(baseSd.Snapshot.Element))
	copy(elements, baseSd.Snapshot.Element)
	for i := range elements {
		if elements[i].Path == "Patient.photo" {
			elements[i].Max = "0"
		}
	}
	profileSD := &registry.StructureDefinition{
		URL:  "http://example.org/StructureDefinition/no-photo-patient",
		Type: "Patient",
		Kind: "resource",
		Snapshot: &registry.Snapshot{
			Element: elements,
		},
	}

	tests := []struct {
		name        string
		resource    string
		expectError bool
		errorPath   string
	}{
		{
			name: "patient without photo is valid",
			resource: `{
				"resourceType": "Patient"
			}`,
			expectError: false,
		},
		{
			name: "patient with photo violates max=0",
			resource: `{
				"resourceType": "Patient",
				"photo": [{"contentType": "image/png"}]
			}`,
			expectError: true,
			errorPath:   "Patient.photo",
		},
		{
			name: "patient with photo still valid against base SD",
			resource: `{
				"resourceType": "Patient",
				"photo": [{"contentType": "image/png"}]
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the profile SD for prohibition tests, base SD for the last test
			sd := profileSD
			if tt.name == "patient with photo still valid against base SD" {
				sd = baseSd
			}

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

func TestGetElementName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"Patient.name", "name"},
		{"Patient.deceased[x]", "deceased[x]"},
		{"Observation.component.code", "code"},
		{"Patient", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getElementName(tt.path)
			if result != tt.expected {
				t.Errorf("getElementName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestValidateNestedRequiredElements(t *testing.T) {
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
			name: "link without other and type should fail",
			resource: `{
				"resourceType": "Patient",
				"link": [{}]
			}`,
			expectError: true,
		},
		{
			name: "link with other and type should pass",
			resource: `{
				"resourceType": "Patient",
				"link": [{
					"other": {"reference": "Patient/123"},
					"type": "seealso"
				}]
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
