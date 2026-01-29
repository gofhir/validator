package fixedpattern

import (
	"encoding/json"
	"testing"
)

func TestDeepEqual(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		expected string
		want     bool
	}{
		// Primitives
		{
			name:     "equal strings",
			actual:   `"hello"`,
			expected: `"hello"`,
			want:     true,
		},
		{
			name:     "different strings",
			actual:   `"hello"`,
			expected: `"world"`,
			want:     false,
		},
		{
			name:     "equal numbers",
			actual:   `42`,
			expected: `42`,
			want:     true,
		},
		{
			name:     "different numbers",
			actual:   `42`,
			expected: `43`,
			want:     false,
		},
		{
			name:     "equal booleans",
			actual:   `true`,
			expected: `true`,
			want:     true,
		},
		{
			name:     "different booleans",
			actual:   `true`,
			expected: `false`,
			want:     false,
		},
		{
			name:     "equal nulls",
			actual:   `null`,
			expected: `null`,
			want:     true,
		},

		// URIs (common for Extension.url)
		{
			name:     "equal URIs",
			actual:   `"http://hl7.org/fhir/StructureDefinition/patient-birthPlace"`,
			expected: `"http://hl7.org/fhir/StructureDefinition/patient-birthPlace"`,
			want:     true,
		},
		{
			name:     "different URIs",
			actual:   `"http://example.org/ext"`,
			expected: `"http://hl7.org/fhir/StructureDefinition/patient-birthPlace"`,
			want:     false,
		},

		// Objects
		{
			name:     "equal simple objects",
			actual:   `{"system": "http://loinc.org", "code": "12345"}`,
			expected: `{"system": "http://loinc.org", "code": "12345"}`,
			want:     true,
		},
		{
			name:     "equal objects different key order",
			actual:   `{"code": "12345", "system": "http://loinc.org"}`,
			expected: `{"system": "http://loinc.org", "code": "12345"}`,
			want:     true,
		},
		{
			name:     "different objects",
			actual:   `{"system": "http://loinc.org", "code": "12345"}`,
			expected: `{"system": "http://snomed.info/sct", "code": "12345"}`,
			want:     false,
		},
		{
			name:     "object with extra property",
			actual:   `{"system": "http://loinc.org", "code": "12345", "display": "Test"}`,
			expected: `{"system": "http://loinc.org", "code": "12345"}`,
			want:     false, // For fixed, must be exactly equal
		},

		// Arrays
		{
			name:     "equal arrays",
			actual:   `["a", "b", "c"]`,
			expected: `["a", "b", "c"]`,
			want:     true,
		},
		{
			name:     "arrays different order",
			actual:   `["c", "b", "a"]`,
			expected: `["a", "b", "c"]`,
			want:     false, // Order matters for fixed
		},
		{
			name:     "arrays different length",
			actual:   `["a", "b"]`,
			expected: `["a", "b", "c"]`,
			want:     false,
		},

		// Complex nested structures (CodeableConcept)
		{
			name: "equal CodeableConcept",
			actual: `{
				"coding": [{"system": "http://loinc.org", "code": "2085-9", "display": "HDL"}],
				"text": "HDL Cholesterol"
			}`,
			expected: `{
				"coding": [{"system": "http://loinc.org", "code": "2085-9", "display": "HDL"}],
				"text": "HDL Cholesterol"
			}`,
			want: true,
		},
		{
			name: "different CodeableConcept code",
			actual: `{
				"coding": [{"system": "http://loinc.org", "code": "2085-9"}],
				"text": "HDL"
			}`,
			expected: `{
				"coding": [{"system": "http://loinc.org", "code": "9999-9"}],
				"text": "HDL"
			}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeepEqual(json.RawMessage(tt.actual), json.RawMessage(tt.expected))
			if got != tt.want {
				t.Errorf("DeepEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsPattern(t *testing.T) {
	tests := []struct {
		name    string
		actual  string
		pattern string
		want    bool
	}{
		// Primitives (same as DeepEqual)
		{
			name:    "equal strings",
			actual:  `"hello"`,
			pattern: `"hello"`,
			want:    true,
		},
		{
			name:    "different strings",
			actual:  `"hello"`,
			pattern: `"world"`,
			want:    false,
		},

		// Objects - pattern matching (partial)
		{
			name:    "pattern subset of actual",
			actual:  `{"system": "http://loinc.org", "code": "12345", "display": "Test"}`,
			pattern: `{"system": "http://loinc.org"}`,
			want:    true, // Pattern only requires system
		},
		{
			name:    "pattern with multiple properties",
			actual:  `{"system": "http://loinc.org", "code": "12345", "display": "Test"}`,
			pattern: `{"system": "http://loinc.org", "code": "12345"}`,
			want:    true,
		},
		{
			name:    "pattern property missing in actual",
			actual:  `{"system": "http://loinc.org"}`,
			pattern: `{"system": "http://loinc.org", "code": "12345"}`,
			want:    false, // Actual doesn't have 'code'
		},
		{
			name:    "pattern property value differs",
			actual:  `{"system": "http://snomed.info/sct", "code": "12345"}`,
			pattern: `{"system": "http://loinc.org"}`,
			want:    false,
		},

		// Arrays - all pattern items must be found
		{
			name:    "array contains pattern item",
			actual:  `[{"system": "http://loinc.org", "code": "12345"}, {"system": "http://snomed.info/sct", "code": "999"}]`,
			pattern: `[{"system": "http://loinc.org"}]`,
			want:    true, // First item matches pattern
		},
		{
			name:    "array doesn't contain pattern item",
			actual:  `[{"system": "http://snomed.info/sct", "code": "999"}]`,
			pattern: `[{"system": "http://loinc.org"}]`,
			want:    false,
		},
		{
			name:    "array contains all pattern items",
			actual:  `[{"system": "http://loinc.org"}, {"system": "http://snomed.info/sct"}]`,
			pattern: `[{"system": "http://loinc.org"}, {"system": "http://snomed.info/sct"}]`,
			want:    true,
		},
		{
			name:    "array missing one pattern item",
			actual:  `[{"system": "http://loinc.org"}]`,
			pattern: `[{"system": "http://loinc.org"}, {"system": "http://snomed.info/sct"}]`,
			want:    false,
		},

		// Nested patterns (CodeableConcept with coding array)
		{
			name: "CodeableConcept matches pattern",
			actual: `{
				"coding": [
					{"system": "http://loinc.org", "code": "2085-9", "display": "HDL"},
					{"system": "http://snomed.info/sct", "code": "123"}
				],
				"text": "HDL Cholesterol"
			}`,
			pattern: `{"coding": [{"system": "http://loinc.org"}]}`,
			want:    true, // Has a coding with the required system
		},
		{
			name: "CodeableConcept missing required coding",
			actual: `{
				"coding": [{"system": "http://snomed.info/sct", "code": "123"}],
				"text": "Something"
			}`,
			pattern: `{"coding": [{"system": "http://loinc.org"}]}`,
			want:    false, // No LOINC coding present
		},

		// Real-world: Observation code pattern
		{
			name: "Observation code pattern match",
			actual: `{
				"coding": [
					{"system": "http://loinc.org", "code": "85354-9", "display": "Blood pressure panel"},
					{"system": "http://example.org", "code": "bp"}
				]
			}`,
			pattern: `{"coding": [{"system": "http://loinc.org", "code": "85354-9"}]}`,
			want:    true,
		},
		{
			name: "Observation code pattern wrong code",
			actual: `{
				"coding": [{"system": "http://loinc.org", "code": "12345-6"}]
			}`,
			pattern: `{"coding": [{"system": "http://loinc.org", "code": "85354-9"}]}`,
			want:    false,
		},

		// Null handling
		{
			name:    "nil pattern always matches",
			actual:  `{"anything": "value"}`,
			pattern: "",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pattern json.RawMessage
			if tt.pattern != "" {
				pattern = json.RawMessage(tt.pattern)
			}

			got := ContainsPattern(json.RawMessage(tt.actual), pattern)
			if got != tt.want {
				t.Errorf("ContainsPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}
