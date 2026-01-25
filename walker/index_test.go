package walker

import (
	"testing"

	"github.com/gofhir/validator/service"
)

func TestToFHIRPath_NoIndex(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		path         string
		want         string
	}{
		{
			name:         "empty path",
			resourceType: "Patient",
			path:         "",
			want:         "Patient",
		},
		{
			name:         "simple path",
			resourceType: "Patient",
			path:         "name",
			want:         "Patient.name",
		},
		{
			name:         "path with array index",
			resourceType: "Patient",
			path:         "name[0].family",
			want:         "Patient.name[0].family",
		},
		{
			name:         "path already has resourceType prefix",
			resourceType: "Patient",
			path:         "Patient.name[0].family",
			want:         "Patient.name[0].family",
		},
		{
			name:         "nested path",
			resourceType: "Observation",
			path:         "component[0].code.coding[0].system",
			want:         "Observation.component[0].code.coding[0].system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToFHIRPath(tt.resourceType, tt.path, nil)
			if got != tt.want {
				t.Errorf("ToFHIRPath(%q, %q, nil) = %q, want %q",
					tt.resourceType, tt.path, got, tt.want)
			}
		})
	}
}

func TestToFHIRPath_WithIndex(t *testing.T) {
	// Create a mock StructureDefinition with choice types
	sd := &service.StructureDefinition{
		Type: "Observation",
		Snapshot: []service.ElementDefinition{
			{Path: "Observation"},
			{Path: "Observation.id"},
			{Path: "Observation.status"},
			{
				Path: "Observation.value[x]",
				Types: []service.TypeRef{
					{Code: "Quantity"},
					{Code: "CodeableConcept"},
					{Code: "string"},
					{Code: "boolean"},
				},
			},
			{Path: "Observation.component"},
			{
				Path: "Observation.component.value[x]",
				Types: []service.TypeRef{
					{Code: "Quantity"},
					{Code: "CodeableConcept"},
					{Code: "string"},
				},
			},
		},
	}

	index := BuildElementIndex(sd)

	tests := []struct {
		name         string
		resourceType string
		path         string
		want         string
	}{
		{
			name:         "non-choice type path",
			resourceType: "Observation",
			path:         "status",
			want:         "Observation.status",
		},
		{
			name:         "choice type valueQuantity",
			resourceType: "Observation",
			path:         "valueQuantity",
			want:         "Observation.value.ofType(Quantity)",
		},
		{
			name:         "choice type valueCodeableConcept",
			resourceType: "Observation",
			path:         "valueCodeableConcept",
			want:         "Observation.value.ofType(CodeableConcept)",
		},
		{
			name:         "choice type valueString",
			resourceType: "Observation",
			path:         "valueString",
			want:         "Observation.value.ofType(String)",
		},
		{
			name:         "choice type with nested path",
			resourceType: "Observation",
			path:         "valueCodeableConcept.coding[0].system",
			want:         "Observation.value.ofType(CodeableConcept).coding[0].system",
		},
		{
			name:         "nested choice type in component",
			resourceType: "Observation",
			path:         "component[0].valueQuantity",
			want:         "Observation.component[0].value.ofType(Quantity)",
		},
		{
			name:         "nested choice type with further nesting",
			resourceType: "Observation",
			path:         "component[0].valueCodeableConcept.coding[1].display",
			want:         "Observation.component[0].value.ofType(CodeableConcept).coding[1].display",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToFHIRPath(tt.resourceType, tt.path, index)
			if got != tt.want {
				t.Errorf("ToFHIRPath(%q, %q, index) = %q, want %q",
					tt.resourceType, tt.path, got, tt.want)
			}
		})
	}
}

func TestToFHIRPath_Extension(t *testing.T) {
	// Create a mock StructureDefinition for Extension with value[x]
	// Note: In real FHIR, extensions can be nested and each level has value[x]
	sd := &service.StructureDefinition{
		Type: "Patient",
		Snapshot: []service.ElementDefinition{
			{Path: "Patient"},
			{Path: "Patient.extension"},
			{
				Path: "Patient.extension.value[x]",
				Types: []service.TypeRef{
					{Code: "string"},
					{Code: "CodeableConcept"},
					{Code: "Coding"},
					{Code: "boolean"},
					{Code: "integer"},
				},
			},
			// Nested extension structure (extension.extension also has value[x])
			{Path: "Patient.extension.extension"},
			{
				Path: "Patient.extension.extension.value[x]",
				Types: []service.TypeRef{
					{Code: "string"},
					{Code: "CodeableConcept"},
					{Code: "Coding"},
					{Code: "boolean"},
					{Code: "integer"},
				},
			},
		},
	}

	index := BuildElementIndex(sd)

	tests := []struct {
		name         string
		resourceType string
		path         string
		want         string
	}{
		{
			name:         "extension with valueString",
			resourceType: "Patient",
			path:         "extension[0].valueString",
			want:         "Patient.extension[0].value.ofType(String)",
		},
		{
			name:         "extension with valueCodeableConcept nested",
			resourceType: "Patient",
			path:         "extension[0].valueCodeableConcept.coding[1].system",
			want:         "Patient.extension[0].value.ofType(CodeableConcept).coding[1].system",
		},
		{
			name:         "nested extension",
			resourceType: "Patient",
			path:         "extension[0].extension[1].valueCodeableConcept.coding[0].code",
			want:         "Patient.extension[0].extension[1].value.ofType(CodeableConcept).coding[0].code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToFHIRPath(tt.resourceType, tt.path, index)
			if got != tt.want {
				t.Errorf("ToFHIRPath(%q, %q, index) = %q, want %q",
					tt.resourceType, tt.path, got, tt.want)
			}
		})
	}
}

func TestBuildElementIndex(t *testing.T) {
	sd := &service.StructureDefinition{
		Type: "Patient",
		Snapshot: []service.ElementDefinition{
			{Path: "Patient"},
			{Path: "Patient.id"},
			{Path: "Patient.name"},
			{Path: "Patient.name.family"},
			{
				Path: "Patient.deceased[x]",
				Types: []service.TypeRef{
					{Code: "boolean"},
					{Code: "dateTime"},
				},
			},
		},
	}

	index := BuildElementIndex(sd)

	t.Run("size", func(t *testing.T) {
		// Should have indexed: Patient, Patient.id, Patient.name, Patient.name.family,
		// Patient.deceased[x], id, name, name.family, deceased[x],
		// Patient.deceasedBoolean, Patient.deceasedDateTime, deceasedBoolean, deceasedDateTime
		// Plus short paths and choice variants
		if index.Size() == 0 {
			t.Error("index should not be empty")
		}
	})

	t.Run("get by path", func(t *testing.T) {
		elem := index.Get("Patient.name")
		if elem == nil {
			t.Error("expected to find Patient.name")
		}
	})

	t.Run("get by short path", func(t *testing.T) {
		elem := index.Get("name")
		if elem == nil {
			t.Error("expected to find name via short path")
		}
	})

	t.Run("get choice type variant", func(t *testing.T) {
		elem := index.Get("Patient.deceasedBoolean")
		if elem == nil {
			t.Fatal("expected to find Patient.deceasedBoolean")
		}
		if elem.Path != "Patient.deceased[x]" {
			t.Errorf("expected path Patient.deceased[x], got %s", elem.Path)
		}
	})

	t.Run("get choice type definition", func(t *testing.T) {
		elem := index.GetChoiceTypeDefinition("Patient.deceased")
		if elem == nil {
			t.Error("expected to find choice type definition for Patient.deceased")
		}
	})

	t.Run("root type", func(t *testing.T) {
		if index.RootType() != "Patient" {
			t.Errorf("expected root type Patient, got %s", index.RootType())
		}
	})
}

func TestRemoveArrayIndices(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Patient.name[0].family", "Patient.name.family"},
		{"extension[0].extension[1].valueString", "extension.extension.valueString"},
		{"Patient.identifier", "Patient.identifier"},
		{"a[0][1].b[2]", "a.b"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := RemoveArrayIndices(tt.input)
			if got != tt.want {
				t.Errorf("RemoveArrayIndices(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"Patient.name.family", []string{"Patient", "name", "family"}},
		{"Patient", []string{"Patient"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SplitPath(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("SplitPath(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitPath(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParentPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Patient.name.family", "Patient.name"},
		{"Patient.name", "Patient"},
		{"Patient", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParentPath(tt.input)
			if got != tt.want {
				t.Errorf("ParentPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
