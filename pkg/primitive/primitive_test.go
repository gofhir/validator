package primitive

import (
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
)

func setupTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()

	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	reg := registry.New()
	if err := reg.LoadFromPackages(packages); err != nil {
		t.Fatalf("Failed to load registry: %v", err)
	}

	return reg
}

func TestValidateStringType(t *testing.T) {
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
			name: "valid string family name",
			resource: `{
				"resourceType": "Patient",
				"name": [{"family": "Smith"}]
			}`,
			expectError: false,
		},
		{
			name: "invalid - number where string expected",
			resource: `{
				"resourceType": "Patient",
				"name": [{"family": 12345}]
			}`,
			expectError: true,
			errorPath:   "Patient.name[0].family",
		},
		{
			name: "invalid - boolean where string expected",
			resource: `{
				"resourceType": "Patient",
				"name": [{"family": true}]
			}`,
			expectError: true,
			errorPath:   "Patient.name[0].family",
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

func TestValidateBooleanType(t *testing.T) {
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
			name: "valid boolean active true",
			resource: `{
				"resourceType": "Patient",
				"active": true
			}`,
			expectError: false,
		},
		{
			name: "valid boolean active false",
			resource: `{
				"resourceType": "Patient",
				"active": false
			}`,
			expectError: false,
		},
		{
			name: "invalid - string where boolean expected",
			resource: `{
				"resourceType": "Patient",
				"active": "true"
			}`,
			expectError: true,
			errorPath:   "Patient.active",
		},
		{
			name: "invalid - number where boolean expected",
			resource: `{
				"resourceType": "Patient",
				"active": 1
			}`,
			expectError: true,
			errorPath:   "Patient.active",
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

func TestValidateDateType(t *testing.T) {
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
			name: "valid date YYYY-MM-DD",
			resource: `{
				"resourceType": "Patient",
				"birthDate": "1990-01-15"
			}`,
			expectError: false,
		},
		{
			name: "valid date YYYY-MM",
			resource: `{
				"resourceType": "Patient",
				"birthDate": "1990-01"
			}`,
			expectError: false,
		},
		{
			name: "valid date YYYY",
			resource: `{
				"resourceType": "Patient",
				"birthDate": "1990"
			}`,
			expectError: false,
		},
		{
			name: "invalid date format",
			resource: `{
				"resourceType": "Patient",
				"birthDate": "15/01/1990"
			}`,
			expectError: true,
			errorPath:   "Patient.birthDate",
		},
		{
			name: "invalid - number where date expected",
			resource: `{
				"resourceType": "Patient",
				"birthDate": 19900115
			}`,
			expectError: true,
			errorPath:   "Patient.birthDate",
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

func TestValidateIntegerType(t *testing.T) {
	reg := setupTestRegistry(t)
	v := New(reg)

	// Observation.component.referenceRange.age.low.value is an integer
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
			name: "valid observation with integer in valueQuantity",
			resource: `{
				"resourceType": "Observation",
				"status": "final",
				"code": {"text": "test"},
				"valueQuantity": {"value": 100}
			}`,
			expectError: false,
		},
		{
			name: "valid observation with decimal in valueQuantity",
			resource: `{
				"resourceType": "Observation",
				"status": "final",
				"code": {"text": "test"},
				"valueQuantity": {"value": 100.5}
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

func TestValidateResourceId(t *testing.T) {
	// Note: Patient.id is typed as "System.String" in the SD, not "id".
	// The "id" type has stricter regex validation, but Resource.id uses a string representation.
	// This test validates the actual SD behavior.
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
			name: "valid id alphanumeric",
			resource: `{
				"resourceType": "Patient",
				"id": "patient123"
			}`,
			expectError: false,
		},
		{
			name: "valid id with dash",
			resource: `{
				"resourceType": "Patient",
				"id": "patient-123"
			}`,
			expectError: false,
		},
		{
			name: "id must be string not number",
			resource: `{
				"resourceType": "Patient",
				"id": 12345
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

func TestValidateUriType(t *testing.T) {
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
			name: "valid uri in identifier system",
			resource: `{
				"resourceType": "Patient",
				"identifier": [{"system": "http://example.org/mrn", "value": "12345"}]
			}`,
			expectError: false,
		},
		{
			name: "valid uri urn:oid format",
			resource: `{
				"resourceType": "Patient",
				"identifier": [{"system": "urn:oid:2.16.840.1.113883.4.1", "value": "12345"}]
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

func TestExtractRegexFromSD(t *testing.T) {
	reg := setupTestRegistry(t)

	tests := []struct {
		typeName      string
		expectPattern bool
	}{
		{"string", true},
		{"boolean", true},
		{"integer", true},
		{"date", true},
		{"dateTime", true},
		{"id", true},
		{"code", true},
		{"uri", true},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			sd := reg.GetByType(tt.typeName)
			if sd == nil {
				t.Skipf("StructureDefinition for %s not found", tt.typeName)
			}

			pattern := extractRegexFromSD(sd)
			if tt.expectPattern && pattern == "" {
				t.Errorf("Expected regex pattern for %s but got empty", tt.typeName)
			}
			if pattern != "" {
				t.Logf("%s regex: %s", tt.typeName, pattern)
			}
		})
	}
}
