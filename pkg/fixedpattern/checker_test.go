package fixedpattern

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// element is an element definition with one fixed or pattern property, given as JSON.
func element(t *testing.T, property string) *registry.ElementDefinition {
	t.Helper()
	var sd registry.StructureDefinition
	raw := `{"resourceType":"StructureDefinition","snapshot":{"element":[{"id":"X","path":"X",` + property + `}]}}`
	if err := json.Unmarshal([]byte(raw), &sd); err != nil {
		t.Fatal(err)
	}
	return &sd.Snapshot.Element[0]
}

// A value is checked against a fixed value exactly and against a pattern as a subset, and each
// difference is reported at the element of the value that differs.
func TestCheckValue(t *testing.T) {
	cc := `{"coding":[{"system":"http://loinc.org","code":"1-8"}]}`
	ext := `{"url":"http://e","valueString":"v"}`
	for _, tt := range []struct {
		name, property, value string
		element               string   // the primitive's "_x" sibling
		want                  []string // "<diagnostic> @ <location>"
	}{
		{name: "a fixed primitive", property: `"fixedUri":"http://a"`, value: `"http://a"`},
		{name: "another primitive", property: `"fixedUri":"http://a"`, value: `"http://b"`, want: []string{"FIXED_VALUE_MISMATCH @ X"}},
		{name: "a decimal's precision counts", property: `"fixedDecimal":1.50`, value: `1.5`, want: []string{"FIXED_VALUE_MISMATCH @ X"}},
		{name: "the same decimal", property: `"fixedDecimal":1.50`, value: `1.50`},
		{name: "a fixed primitive with an extension", property: `"fixedCode":"final"`, value: `"final"`,
			element: `{"extension":[{"url":"http://e","valueString":"v"}]}`, want: []string{"FIXED_VALUE_EXTRA @ X"}},
		{name: "a pattern primitive with an extension", property: `"patternCode":"final"`, value: `"final"`,
			element: `{"extension":[{"url":"http://e","valueString":"v"}]}`},
		{name: "a fixed complex value", property: `"fixedCodeableConcept":` + cc, value: cc},
		{name: "an element a fixed value has is missing", property: `"fixedCodeableConcept":` + cc, value: `{"coding":[{"code":"1-8"}]}`,
			want: []string{"FIXED_VALUE_MISSING @ X.coding[0].system"}},
		{name: "an element a fixed value has not", property: `"fixedCodeableConcept":` + cc, value: `{"coding":[{"system":"http://loinc.org","code":"1-8"}],"text":"t"}`,
			want: []string{"FIXED_VALUE_EXTRA @ X.text"}},
		{name: "an extension a fixed value has not, at its holder", property: `"fixedCodeableConcept":` + cc,
			value: `{"coding":[{"system":"http://loinc.org","code":"1-8","extension":[{"url":"http://e","valueString":"v"}]}]}`,
			want:  []string{"FIXED_VALUE_EXTRA @ X.coding[0]"}},
		{name: "a primitive's extension in a fixed value, at the primitive", property: `"fixedCodeableConcept":` + cc,
			value: `{"coding":[{"system":"http://loinc.org","code":"1-8","_code":{"extension":[{"url":"http://e","valueString":"v"}]}}]}`,
			want:  []string{"FIXED_VALUE_EXTRA @ X.coding[0].code"}},
		{name: "an item a fixed array has not", property: `"fixedCodeableConcept":` + cc, value: `{"coding":[{"system":"http://loinc.org","code":"1-8"},{"code":"z"}]}`,
			want: []string{"FIXED_VALUE_EXTRA @ X.coding[1]"}},
		{name: "a pattern as a subset", property: `"patternCodeableConcept":` + cc, value: `{"coding":[{"code":"z"},{"system":"http://loinc.org","code":"1-8","display":"d"}],"text":"t"}`},
		{name: "a pattern array item no item matches", property: `"patternCodeableConcept":` + cc, value: `{"coding":[{"system":"http://loinc.org","code":"2-6"}]}`,
			want: []string{"PATTERN_ITEM_UNMATCHED @ X"}},
		{name: "a pattern's primitive at its element", property: `"patternCoding":{"system":"http://a"}`, value: `{"system":"http://b","code":"c"}`,
			want: []string{"FIXED_VALUE_MISMATCH @ X.system"}},
		{name: "a pattern's element missing", property: `"patternCoding":{"system":"http://a"}`, value: `{"code":"c"}`,
			want: []string{"FIXED_VALUE_MISSING @ X.system"}},
		{name: "a repeating primitive's id and extension in a fixed value, at the item", property: `"fixedHumanName":{"given":["A"]}`,
			value: `{"given":["A"],"_given":[{"id":"g1","extension":[{"url":"http://e","valueString":"v"}]}]}`,
			want:  []string{"FIXED_VALUE_EXTRA @ X.given[0]", "FIXED_VALUE_EXTRA @ X.given[0]"}},
		{name: "a repeating primitive with no id or extensions", property: `"fixedHumanName":{"given":["A","B"]}`,
			value: `{"given":["A","B"],"_given":[null,null]}`},
		{name: "a fixed value's primitive extension is missing", property: `"fixedCoding":{"code":"a","_code":{"extension":[{"url":"http://e","valueString":"v"}]}}`,
			value: `{"code":"a"}`, want: []string{"FIXED_VALUE_MISSING @ X.code"}},
		{name: "a fixed value's primitive extension", property: `"fixedCoding":{"code":"a","_code":{"extension":[{"url":"http://e","valueString":"v"}]}}`,
			value: `{"code":"a","_code":{"extension":[{"url":"http://e","valueString":"v"}]}}`},
		{name: "a fixed value's primitive extension and an id it has not", property: `"fixedCoding":{"code":"a","_code":{"extension":[{"url":"http://e","valueString":"v"}]}}`,
			value: `{"code":"a","_code":{"id":"i","extension":[{"url":"http://e","valueString":"w"}]}}`,
			want:  []string{"FIXED_VALUE_MISMATCH @ X.code.extension[0].valueString", "FIXED_VALUE_EXTRA @ X.code"}},
		{name: "a fixed primitive's extension is missing", property: `"fixedCode":"a","_fixedCode":{"extension":[{"url":"http://e","valueString":"v"}]}`,
			value: `"a"`, want: []string{"FIXED_VALUE_MISSING @ X"}},
		{name: "a fixed primitive's extension", property: `"fixedCode":"a","_fixedCode":{"extension":[{"url":"http://e","valueString":"v"}]}`,
			value: `"a"`, element: `{"extension":[{"url":"http://e","valueString":"v"}]}`},
		{name: "a pattern primitive's extension is missing", property: `"patternCode":"a","_patternCode":{"extension":[{"url":"http://e","valueString":"v"}]}`,
			value: `"a"`, element: `{"extension":[{"url":"http://o","valueString":"v"}]}`, want: []string{"PATTERN_ITEM_UNMATCHED @ X"}},
		{name: "a pattern's primitive item with its extension, met by a later item", property: `"patternHumanName":{"given":["A"],"_given":[{"extension":[` + ext + `]}]}`,
			value: `{"given":["B","A"],"_given":[null,{"extension":[` + ext + `]}]}`},
		{name: "a pattern's primitive item with its extension, no item has both", property: `"patternHumanName":{"given":["A"],"_given":[{"extension":[` + ext + `]}]}`,
			value: `{"given":["B","A"],"_given":[{"extension":[` + ext + `]},null]}`, want: []string{"PATTERN_ITEM_UNMATCHED @ X"}},
		{name: "a primitive's extensions as an array where the pattern has one object", property: `"patternCoding":{"code":"M","_code":{"extension":[` + ext + `]}}`,
			value: `{"code":"M","_code":[{"id":"x"}]}`, want: []string{"FIXED_VALUE_MISMATCH @ X.code"}},
		{name: "a primitive with extensions and no value, against a fixed value", property: `"fixedCode":"a"`,
			element: `{"extension":[` + ext + `]}`, want: []string{"FIXED_VALUE_MISSING @ X", "FIXED_VALUE_EXTRA @ X"}},
		{name: "a primitive with extensions and no value, against a pattern", property: `"patternCode":"a"`,
			element: `{"extension":[` + ext + `]}`, want: []string{"FIXED_VALUE_MISSING @ X"}},
		{name: "a fixed primitive with extensions and no value", property: `"_fixedCode":{"extension":[` + ext + `]}`,
			element: `{"extension":[` + ext + `]}`},
		{name: "a value where a fixed primitive has none", property: `"_fixedCode":{"extension":[` + ext + `]}`,
			value: `"a"`, element: `{"extension":[` + ext + `]}`, want: []string{"FIXED_VALUE_EXTRA @ X"}},
		{name: "a null item with a null \"_x\" item", property: `"fixedCode":"a"`, value: `null`, element: `null`},
		{name: "a fixed primitive's extension that differs, at its own location", property: `"fixedCode":"a","_fixedCode":{"extension":[` + ext + `]}`,
			value: `"a"`, element: `{"extension":[{"url":"http://e","valueString":"w"},` + ext + `]}`,
			want: []string{"FIXED_VALUE_MISMATCH @ X.extension[0].valueString", "FIXED_VALUE_EXTRA @ X.extension[1]"}},
		{name: "a pattern on a value of another kind", property: `"patternCoding":{"system":"http://a"}`, value: `"c"`,
			want: []string{"FIXED_VALUE_MISMATCH @ X"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var raw, el json.RawMessage
			if tt.value != "" {
				raw = json.RawMessage(tt.value)
			}
			if tt.element != "" {
				el = json.RawMessage(tt.element)
			}
			var got []string
			c := NewChecker()
			def := element(t, tt.property)
			if !c.Governs(def) {
				t.Fatal("Governs: false")
			}
			c.CheckValue(def, raw, el, "X", func(id issue.DiagnosticID, _ map[string]any, at string) {
				got = append(got, string(id)+" @ "+at)
			})
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
	if NewChecker().Governs(element(t, `"short":"no fixed value"`)) {
		t.Error("Governs: true for an element with no fixed or pattern value")
	}
}

// A long value is rendered whole, so that two pattern items that differ only after their first
// characters are two issues.
func TestCheckValueRendersWhole(t *testing.T) {
	long := strings.Repeat("D", 120)
	def := element(t, `"patternCodeableConcept":{"coding":[{"system":"http://s1","display":"`+long+`"},{"system":"http://s2","display":"`+long+`"}]}`)
	var patterns []string
	NewChecker().CheckValue(def, json.RawMessage(`{"coding":[{"system":"http://s3"}]}`), nil, "X", func(_ issue.DiagnosticID, params map[string]any, _ string) {
		patterns = append(patterns, params["pattern"].(string))
	})
	if len(patterns) != 2 || patterns[0] == patterns[1] || !strings.Contains(patterns[0], "http://s1") {
		t.Errorf("got %q", patterns)
	}
}
