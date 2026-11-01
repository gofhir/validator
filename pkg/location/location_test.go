package location

import (
	"strconv"
	"strings"
	"testing"
)

func TestFind(t *testing.T) {
	jsonData := []byte(`{
  "resourceType": "Patient",
  "identifier": [
    {
      "system": "http://example.org",
      "value": "12345"
    }
  ],
  "name": [
    {
      "family": "Smith",
      "given": ["John", "James"]
    }
  ]
}`)

	tests := []struct {
		name       string
		fhirPath   string
		wantLine   int
		wantColumn int
		wantNil    bool
	}{
		{
			name:       "the resource itself",
			fhirPath:   "Patient",
			wantLine:   1,
			wantColumn: 2,
		},
		{
			name:       "root field",
			fhirPath:   "Patient.resourceType",
			wantLine:   2,
			wantColumn: 17,
		},
		{
			name:       "simple field",
			fhirPath:   "Patient.identifier",
			wantLine:   3,
			wantColumn: 15,
		},
		{
			name:       "array element",
			fhirPath:   "Patient.identifier[0]",
			wantLine:   4,
			wantColumn: 5,
		},
		{
			name:       "nested field in array",
			fhirPath:   "Patient.identifier[0].system",
			wantLine:   5,
			wantColumn: 15,
		},
		{
			name:       "another nested field",
			fhirPath:   "Patient.identifier[0].value",
			wantLine:   6,
			wantColumn: 14,
		},
		{
			name:       "second array",
			fhirPath:   "Patient.name[0].family",
			wantLine:   11,
			wantColumn: 15,
		},
		{
			name:       "nested array element",
			fhirPath:   "Patient.name[0].given[0]",
			wantLine:   12,
			wantColumn: 17,
		},
		{
			// At the item, not at the comma before it.
			name:       "nested array second element",
			fhirPath:   "Patient.name[0].given[1]",
			wantLine:   12,
			wantColumn: 25,
		},
		{
			name:     "non-existent field",
			fhirPath: "Patient.nonexistent",
			wantNil:  true,
		},
		{
			name:     "out of bounds index",
			fhirPath: "Patient.identifier[5]",
			wantNil:  true,
		},
		{
			name:     "empty path",
			fhirPath: "",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := Find(jsonData, tt.fhirPath)

			if tt.wantNil {
				if loc != nil {
					t.Errorf("Find(%q) = %+v, want nil", tt.fhirPath, loc)
				}
				return
			}

			if loc == nil {
				t.Fatalf("Find(%q) = nil, want line %d col %d", tt.fhirPath, tt.wantLine, tt.wantColumn)
			}

			if loc.Line != tt.wantLine {
				t.Errorf("Find(%q).Line = %d, want %d", tt.fhirPath, loc.Line, tt.wantLine)
			}
			if loc.Column != tt.wantColumn {
				t.Errorf("Find(%q).Column = %d, want %d", tt.fhirPath, loc.Column, tt.wantColumn)
			}
		})
	}
}

func TestParseFHIRPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"Patient.identifier", []string{"identifier"}},
		{"identifier", []string{"identifier"}},
		{"Patient.identifier[0]", []string{"identifier", "0"}},
		{"Patient.identifier[0].value", []string{"identifier", "0", "value"}},
		{"Bundle.entry[0].resource.id", []string{"entry", "0", "resource", "id"}},
		{"name[0].given[1]", []string{"name", "0", "given", "1"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := parseFHIRPath(tt.path)
			if len(got) != len(tt.expected) {
				t.Errorf("parseFHIRPath(%q) = %v, want %v", tt.path, got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseFHIRPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestLineCols(t *testing.T) {
	input := []byte("line1\nline2\nline3")
	//                01234 5 67890 1 23456

	tests := []struct {
		offset   int
		wantLine int
		wantCol  int
	}{
		{0, 1, 1},
		{4, 1, 5},
		{5, 1, 6}, // newline char
		{6, 2, 1},
		{11, 2, 6},
		{12, 3, 1},
		{17, 3, 6},
	}

	offsets := map[string]int{}
	for _, tt := range tests {
		offsets[strconv.Itoa(tt.offset)] = tt.offset
	}
	got := map[string]lineCol{}
	for _, lc := range lineCols(input, offsets) {
		got[lc.key] = lc
	}
	for _, tt := range tests {
		lc := got[strconv.Itoa(tt.offset)]
		if lc.line != tt.wantLine || lc.col != tt.wantCol {
			t.Errorf("offset %d at (%d, %d), want (%d, %d)", tt.offset, lc.line, lc.col, tt.wantLine, tt.wantCol)
		}
	}
}

func TestFindMinifiedJSON(t *testing.T) {
	// Minified JSON (no whitespace)
	jsonData := []byte(`{"resourceType":"Patient","identifier":[{"system":"http://example.org","value":"12345"}]}`)

	tests := []struct {
		fhirPath   string
		wantLine   int
		wantColumn int
	}{
		{"Patient", 1, 2},
		{"Patient.resourceType", 1, 16},
		{"Patient.identifier", 1, 39},
		{"Patient.identifier[0]", 1, 41},
		{"Patient.identifier[0].system", 1, 50},
		{"Patient.identifier[0].value", 1, 79},
	}

	for _, tt := range tests {
		t.Run(tt.fhirPath, func(t *testing.T) {
			loc := Find(jsonData, tt.fhirPath)
			if loc == nil {
				t.Fatalf("Find(%q) = nil", tt.fhirPath)
			}
			if loc.Line != tt.wantLine {
				t.Errorf("Find(%q).Line = %d, want %d", tt.fhirPath, loc.Line, tt.wantLine)
			}
			if loc.Column != tt.wantColumn {
				t.Errorf("Find(%q).Column = %d, want %d", tt.fhirPath, loc.Column, tt.wantColumn)
			}
		})
	}
}

func TestFindBundleEntry(t *testing.T) {
	jsonData := []byte(`{
  "resourceType": "Bundle",
  "entry": [
    {
      "fullUrl": "urn:uuid:123",
      "resource": {
        "resourceType": "Patient",
        "id": "123"
      }
    }
  ]
}`)

	tests := []struct {
		fhirPath string
		wantLine int
	}{
		{"Bundle.entry", 3},
		{"Bundle.entry[0]", 4},
		{"Bundle.entry[0].fullUrl", 5},
		{"Bundle.entry[0].resource", 6},
		{"Bundle.entry[0].resource.resourceType", 7},
		{"Bundle.entry[0].resource.id", 8},
	}

	for _, tt := range tests {
		t.Run(tt.fhirPath, func(t *testing.T) {
			loc := Find(jsonData, tt.fhirPath)
			if loc == nil {
				t.Fatalf("Find(%q) = nil", tt.fhirPath)
			}
			if loc.Line != tt.wantLine {
				t.Errorf("Find(%q).Line = %d, want %d", tt.fhirPath, loc.Line, tt.wantLine)
			}
		})
	}
}

// A path names a property of the object it is in: not a string value equal to its name, and not
// a property of the same name in an object nested before it.
func TestFindNamesTheProperty(t *testing.T) {
	jsonData := []byte(`{
  "resourceType": "Patient",
  "meta": {"id": "m1"},
  "id": "p1",
  "telecom": [{"use": "name"}],
  "contact": [{"relationship": [{"text": "name"}]}],
  "name": [{"family": "A"}]
}`)
	for _, tt := range []struct {
		path      string
		line, col int
	}{
		{"Patient.id", 4, 7},
		{"Patient.meta.id", 3, 16},
		{"Patient.name", 7, 9},
		{"Patient.name[0].family", 7, 21},
		{"Patient.name:official", 7, 9},
	} {
		loc := Find(jsonData, tt.path)
		if loc == nil || loc.Line != tt.line || loc.Column != tt.col {
			t.Errorf("Find(%q) = %+v, want line %d col %d", tt.path, loc, tt.line, tt.col)
		}
	}
}

// FindAll locates each path where Find does, in one pass, and leaves out the paths it cannot find.
func TestFindAll(t *testing.T) {
	jsonData := []byte(`{
  "resourceType": "Bundle",
  "entry": [
    {"fullUrl": "urn:uuid:1", "resource": {"resourceType": "Patient", "name": [{"given": ["A", "B"]}]}},
    {"fullUrl": "urn:uuid:2", "resource": {"resourceType": "Patient", "contact": [{"gender": "male"}]}}
  ]
}`)
	paths := []string{
		"Bundle.entry", "Bundle.entry[1]", "Bundle.entry[0].resource.name[0].given[1]",
		"Bundle.entry[1].resource.contact[0]", "Bundle.entry[1].resource", "Bundle.entry[0].resource.name[0].given[1]",
		"Bundle.entry[2]", "Bundle.entry[0].resource.contact", "Observation.value[x]", "", "Bundle",
	}
	got := FindAll(jsonData, paths)
	for _, p := range paths {
		want := Find(jsonData, p)
		if (got[p] == nil) != (want == nil) || (want != nil && *got[p] != *want) {
			t.Errorf("FindAll[%q] = %+v, Find = %+v", p, got[p], want)
		}
	}
	if len(got) != 6 {
		t.Errorf("located %d paths, want the 6 that exist: %v", len(got), got)
	}
	if loc := got["Bundle.entry[1].resource.contact[0]"]; loc == nil || loc.Line != 5 {
		t.Errorf("Bundle.entry[1].resource.contact[0] at %+v, want line 5", loc)
	}
}

// An array item after the first is located at the item, on its own line, not at the comma that
// ends the line before it.
func TestFindItemAfterAComma(t *testing.T) {
	jsonData := []byte(`{
  "resourceType": "Patient",
  "name": [
    {
      "family": "A"
    },
    {
      "family": "B"
    } , {"family": "C"}
  ]
}`)
	for _, tt := range []struct {
		path      string
		line, col int
	}{
		{"Patient.name[0]", 4, 5},
		{"Patient.name[1]", 7, 5},
		{"Patient.name[2]", 9, 9},
	} {
		loc := Find(jsonData, tt.path)
		if loc == nil || loc.Line != tt.line || loc.Column != tt.col {
			t.Errorf("Find(%q) = %+v, want line %d col %d", tt.path, loc, tt.line, tt.col)
		}
	}
}

// json.html#primitive: el id y las Extension de un primitivo viven en la propiedad con el mismo
// nombre y prefijo "_" (los ítems de un arreglo, alineados por índice, con null donde no hay), y
// la ruta FHIRPath de lo que contienen no lleva el "_". Una ruta al primitivo mismo sigue
// ubicando la propiedad del valor.
func TestFindUnderPrimitiveSibling(t *testing.T) {
	// at es el offset (0-indexado) que se espera: justo después de la clave, o al inicio del ítem.
	after := func(src, key string) int { return strings.Index(src, key) + len(key) }
	at := strings.Index
	tests := []struct {
		name, src, path string
		offset          int // -1: no se ubica
	}{
		{"object primitive", `{"resourceType":"Patient","birthDate":"2000","_birthDate":{"extension":[{"url":"u","valueString":"x"}]}}`,
			"Patient.birthDate.extension[0].valueString", -2},
		{"the primitive itself keeps the value", `{"resourceType":"Patient","birthDate":"2000","_birthDate":{"id":"a"}}`,
			"Patient.birthDate", -3},
		{"array primitive", `{"resourceType":"Patient","name":[{"given":["A","B"],"_given":[null,{"extension":[{"url":"u"}]}]}]}`,
			"Patient.name[0].given[1].extension[0]", -4},
		{"nested", `{"resourceType":"Patient","name":[{"family":"F","_family":{"extension":[{"url":"u","valueString":"x"}]}}]}`,
			"Patient.name[0].family.extension[0].valueString", -5},
		{"only the underscore property", `{"resourceType":"Patient","_birthDate":{"extension":[{"url":"u"}]}}`,
			"Patient.birthDate.extension[0]", -6},
		{"the literal key wins", `{"resourceType":"Patient","_a":{"b":1},"a":{"b":2}}`,
			"Patient.a.b", -7},
		{"in neither", `{"resourceType":"Patient","birthDate":"2000","_birthDate":{"id":"a"}}`,
			"Patient.birthDate.extension[0]", -1},
		{"literal _id", `{"resourceType":"Patient","_id":{"extension":[{"url":"u"}]}}`,
			"Patient._id", -8},
		{"literal _code", `{"resourceType":"Observation","code":{"text":"t"},"_code":{"id":"a"}}`,
			"Observation._code", -9},
		{"literal _birthDate path", `{"resourceType":"Patient","birthDate":"2000","_birthDate":{"extension":[{"url":"u","valueString":"x"}]}}`,
			"Patient._birthDate.extension[0].valueString", -10},
		{"literal _given item", `{"resourceType":"Patient","name":[{"given":["A","B"],"_given":[null,{"id":"i"}]}]}`,
			"Patient.name[0]._given[1]", -11},
		{"null item has no children", `{"resourceType":"Patient","name":[{"given":["A","B"],"_given":[null,{"id":"i"}]}]}`,
			"Patient.name[0].given[0].id", -1},
	}
	want := map[string]func(src string) int{
		"object primitive":                     func(s string) int { return after(s, `"valueString"`) },
		"the primitive itself keeps the value": func(s string) int { return after(s, `"birthDate"`) },
		"array primitive":                      func(s string) int { return at(s, `{"extension":[{"url":"u"}]}`) + len(`{"extension":[`) },
		"nested":                               func(s string) int { return after(s, `"valueString"`) },
		"only the underscore property":         func(s string) int { return at(s, `{"url":"u"}`) },
		"the literal key wins":                 func(s string) int { return after(s, `"b"`+`:2`) - len(`:2`) },
		"literal _id":                          func(s string) int { return after(s, `"_id"`) },
		"literal _code":                        func(s string) int { return after(s, `"_code"`) },
		"literal _birthDate path":              func(s string) int { return after(s, `"valueString"`) },
		"literal _given item":                  func(s string) int { return at(s, `{"id":"i"}`) },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := Find([]byte(tt.src), tt.path)
			if tt.offset == -1 {
				if loc != nil {
					t.Fatalf("Find(%q) = %+v, want nil", tt.path, loc)
				}
				return
			}
			if loc == nil {
				t.Fatalf("Find(%q) = nil", tt.path)
			}
			if loc.Line != 1 || loc.Column != want[tt.name](tt.src)+1 {
				t.Errorf("Find(%q) = %+v, want column %d", tt.path, loc, want[tt.name](tt.src)+1)
			}
		})
	}
}
