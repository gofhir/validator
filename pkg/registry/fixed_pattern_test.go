package registry

import (
	"encoding/json"
	"testing"
)

func TestElementDefinition_GetFixed(t *testing.T) {
	tests := []struct {
		name           string
		json           string
		wantExists     bool
		wantTypeSuffix string
		wantValue      string
	}{
		{
			name:           "fixedUri",
			json:           `{"path": "Extension.url", "fixedUri": "http://example.org/ext"}`,
			wantExists:     true,
			wantTypeSuffix: "Uri",
			wantValue:      `"http://example.org/ext"`,
		},
		{
			name:           "fixedCode",
			json:           `{"path": "Observation.status", "fixedCode": "final"}`,
			wantExists:     true,
			wantTypeSuffix: "Code",
			wantValue:      `"final"`,
		},
		{
			name:           "fixedBoolean",
			json:           `{"path": "Group.actual", "fixedBoolean": true}`,
			wantExists:     true,
			wantTypeSuffix: "Boolean",
			wantValue:      `true`,
		},
		{
			name:           "fixedInteger",
			json:           `{"path": "Element.count", "fixedInteger": 42}`,
			wantExists:     true,
			wantTypeSuffix: "Integer",
			wantValue:      `42`,
		},
		{
			name:           "fixedCodeableConcept",
			json:           `{"path": "Observation.code", "fixedCodeableConcept": {"coding": [{"system": "http://loinc.org", "code": "12345"}]}}`,
			wantExists:     true,
			wantTypeSuffix: "CodeableConcept",
			wantValue:      `{"coding": [{"system": "http://loinc.org", "code": "12345"}]}`,
		},
		{
			name:       "no fixed value",
			json:       `{"path": "Patient.name", "min": 0, "max": "*"}`,
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ed ElementDefinition
			if err := json.Unmarshal([]byte(tt.json), &ed); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			ed.raw = json.RawMessage(tt.json)

			value, typeSuffix, exists := ed.GetFixed()

			if exists != tt.wantExists {
				t.Errorf("exists = %v, want %v", exists, tt.wantExists)
			}
			if !tt.wantExists {
				return
			}
			if typeSuffix != tt.wantTypeSuffix {
				t.Errorf("typeSuffix = %q, want %q", typeSuffix, tt.wantTypeSuffix)
			}

			// Compare values by normalizing JSON
			var gotVal, wantVal interface{}
			if err := json.Unmarshal(value, &gotVal); err != nil {
				t.Fatalf("unmarshal got value: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantValue), &wantVal); err != nil {
				t.Fatalf("unmarshal want value: %v", err)
			}

			gotJSON, _ := json.Marshal(gotVal)
			wantJSON, _ := json.Marshal(wantVal)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("value = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestElementDefinition_GetPattern(t *testing.T) {
	tests := []struct {
		name           string
		json           string
		wantExists     bool
		wantTypeSuffix string
		wantValue      string
	}{
		{
			name:           "patternCoding",
			json:           `{"path": "Observation.code.coding", "patternCoding": {"system": "http://loinc.org"}}`,
			wantExists:     true,
			wantTypeSuffix: "Coding",
			wantValue:      `{"system": "http://loinc.org"}`,
		},
		{
			name:           "patternCodeableConcept",
			json:           `{"path": "Observation.code", "patternCodeableConcept": {"coding": [{"system": "http://loinc.org"}]}}`,
			wantExists:     true,
			wantTypeSuffix: "CodeableConcept",
			wantValue:      `{"coding": [{"system": "http://loinc.org"}]}`,
		},
		{
			name:           "patternString",
			json:           `{"path": "Patient.name.family", "patternString": "Smith"}`,
			wantExists:     true,
			wantTypeSuffix: "String",
			wantValue:      `"Smith"`,
		},
		{
			name:       "no pattern value",
			json:       `{"path": "Patient.name", "min": 0, "max": "*"}`,
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ed ElementDefinition
			if err := json.Unmarshal([]byte(tt.json), &ed); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			ed.raw = json.RawMessage(tt.json)

			value, typeSuffix, exists := ed.GetPattern()

			if exists != tt.wantExists {
				t.Errorf("exists = %v, want %v", exists, tt.wantExists)
			}
			if !tt.wantExists {
				return
			}
			if typeSuffix != tt.wantTypeSuffix {
				t.Errorf("typeSuffix = %q, want %q", typeSuffix, tt.wantTypeSuffix)
			}

			// Compare values by normalizing JSON
			var gotVal, wantVal interface{}
			if err := json.Unmarshal(value, &gotVal); err != nil {
				t.Fatalf("unmarshal got value: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantValue), &wantVal); err != nil {
				t.Fatalf("unmarshal want value: %v", err)
			}

			gotJSON, _ := json.Marshal(gotVal)
			wantJSON, _ := json.Marshal(wantVal)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("value = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestSnapshot_UnmarshalJSON_PreservesRaw(t *testing.T) {
	snapshotJSON := `{
		"element": [
			{"path": "Extension.url", "fixedUri": "http://example.org/ext"},
			{"path": "Extension.value[x]", "min": 0}
		]
	}`

	var snapshot Snapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(snapshot.Element) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(snapshot.Element))
	}

	// First element should have fixedUri
	value, typeSuffix, exists := snapshot.Element[0].GetFixed()
	if !exists {
		t.Error("expected fixedUri to exist in first element")
	}
	if typeSuffix != "Uri" {
		t.Errorf("typeSuffix = %q, want 'Uri'", typeSuffix)
	}
	if string(value) != `"http://example.org/ext"` {
		t.Errorf("value = %s, want '\"http://example.org/ext\"'", value)
	}

	// Second element should not have fixed
	_, _, exists = snapshot.Element[1].GetFixed()
	if exists {
		t.Error("expected no fixed value in second element")
	}
}
