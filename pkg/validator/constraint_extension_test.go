package validator

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

const (
	startsWithXURL = "https://example.org/fhir/StructureDefinition/starts-with-x"
	shortXURL      = "https://example.org/fhir/StructureDefinition/short-x"
	shortXSliceURL = "https://example.org/fhir/StructureDefinition/short-x-slice"
	requiresAURL   = "https://example.org/fhir/StructureDefinition/requires-a"
	requiresASlice = "https://example.org/fhir/StructureDefinition/requires-a-slice"
	birthTimeSlice = "https://example.org/fhir/StructureDefinition/birth-time-slice"
	birthDateExt   = "https://example.org/fhir/StructureDefinition/birth-date-ext"
	birthDateValue = "https://example.org/fhir/StructureDefinition/birth-date-value"
)

// extensionDefinitions are the extension definitions the constraint walk is tested with:
// starts-with-x requires a string value that starts with "x" (swx-1); short-x, for the same url but
// derived from Extension, not from starts-with-x, requires at most 3 characters (sx-1);
// short-x-slice is a Patient profile with a slice of Patient.extension that declares short-x.
var extensionDefinitions = [][]byte{[]byte(`{"resourceType":"StructureDefinition","url":"` + birthDateValue + `",
"name":"BirthDateValue","status":"active","fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Patient",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint",
"differential":{"element":[{"id":"Patient.birthDate.value","path":"Patient.birthDate.value","min":1}]}}`),
	[]byte(`{"resourceType":"StructureDefinition","url":"` + birthTimeSlice + `",
"name":"BirthTimeSlice","status":"active","fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Patient",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint",
"differential":{"element":[
 {"id":"Patient.birthDate.extension","path":"Patient.birthDate.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"}},
 {"id":"Patient.birthDate.extension:birthTime","path":"Patient.birthDate.extension","sliceName":"birthTime","min":1,"max":"1",
  "type":[{"code":"Extension","profile":["http://hl7.org/fhir/StructureDefinition/patient-birthTime"]}]}]}}`),
	[]byte(`{"resourceType":"StructureDefinition","url":"` + birthDateExt + `",
"name":"BirthDateExt","status":"active","fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Patient",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint",
"differential":{"element":[{"id":"Patient.birthDate.extension","path":"Patient.birthDate.extension","min":1}]}}`),
	[]byte(`{"resourceType":"StructureDefinition","url":"` + requiresASlice + `",
"name":"RequiresASlice","status":"active","fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Patient",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint",
"differential":{"element":[
 {"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"}},
 {"id":"Patient.extension:req","path":"Patient.extension","sliceName":"req","type":[{"code":"Extension","profile":["` + requiresAURL + `"]}]}]}}`),
	[]byte(`{"resourceType":"StructureDefinition","url":"` + requiresAURL + `",
"name":"RequiresA","status":"active","fhirVersion":"4.0.1","kind":"complex-type","abstract":false,
"context":[{"type":"element","expression":"Element"}],"type":"Extension",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint",
"differential":{"element":[
 {"id":"Extension","path":"Extension"},
 {"id":"Extension.extension","path":"Extension.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"}},
 {"id":"Extension.extension:a","path":"Extension.extension","sliceName":"a","min":1,"max":"1"},
 {"id":"Extension.extension:a.url","path":"Extension.extension.url","fixedUri":"a"},
 {"id":"Extension.extension:a.value[x]","path":"Extension.extension.value[x]","type":[{"code":"string"}]},
 {"id":"Extension.url","path":"Extension.url","fixedUri":"` + requiresAURL + `"},
 {"id":"Extension.value[x]","path":"Extension.value[x]","max":"0"}]}}`),
	[]byte(`{"resourceType":"StructureDefinition","url":"` + shortXURL + `",
"name":"ShortX","status":"active","fhirVersion":"4.0.1","kind":"complex-type","abstract":false,
"context":[{"type":"element","expression":"Element"}],"type":"Extension",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint",
"differential":{"element":[
 {"id":"Extension","path":"Extension","constraint":[{"key":"sx-1","severity":"error","human":"sx-1",
  "expression":"(value as string).length() <= 3","source":"` + shortXURL + `"}]},
 {"id":"Extension.url","path":"Extension.url","fixedUri":"` + startsWithXURL + `"},
 {"id":"Extension.value[x]","path":"Extension.value[x]","type":[{"code":"string"}]}]}}`),
	[]byte(`{"resourceType":"StructureDefinition","url":"` + shortXSliceURL + `",
"name":"ShortXSlice","status":"active","fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Patient",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint",
"differential":{"element":[
 {"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"}},
 {"id":"Patient.extension:short","path":"Patient.extension","sliceName":"short","type":[{"code":"Extension","profile":["` + shortXURL + `"]}]}]}}`),
	[]byte(`{"resourceType":"StructureDefinition","url":"` + startsWithXURL + `",
"name":"StartsWithX","status":"active","fhirVersion":"4.0.1","kind":"complex-type","abstract":false,
"context":[{"type":"element","expression":"Element"}],"type":"Extension",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint",
"differential":{"element":[
 {"id":"Extension","path":"Extension","constraint":[{"key":"swx-1","severity":"error","human":"swx-1",
  "expression":"(value as string).startsWith('x')","source":"` + startsWithXURL + `"}]},
 {"id":"Extension.url","path":"Extension.url","fixedUri":"` + startsWithXURL + `"},
 {"id":"Extension.value[x]","path":"Extension.value[x]","type":[{"code":"string"}]}]}}`)}

// An extension is checked against the definition its url names wherever it is used, with or
// without a profile, as the HL7 validator checks it (plan B, PR B4a).
func TestExtensionDefinitionGovernsTheExtension(t *testing.T) {
	v := profileValidator(t)
	const text = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`
	ext := func(value string) string { return `{"url":"` + startsWithXURL + `","valueString":"` + value + `"}` }
	for _, tt := range []struct {
		name, resource string
		want           []string
	}{{
		name:     "on a resource, a data type and a primitive",
		resource: `{"resourceType":"Patient",` + text + `,"extension":[` + ext("y") + `],"name":[{"family":"A","extension":[` + ext("z") + `]}],"birthDate":"2000-01-01","_birthDate":{"extension":[` + ext("w") + `]}}`,
		want:     []string{"Patient.birthDate.extension[0]", "Patient.extension[0]", "Patient.name[0].extension[0]"},
	}, {
		name:     "in a Bundle entry",
		resource: `{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"urn:uuid:9b6b1d38-3f5f-4c1e-9d3e-6a4b8f0c2e11","resource":{"resourceType":"Patient",` + text + `,"extension":[` + ext("y") + `]}}]}`,
		want:     []string{"Bundle.entry[0].resource.extension[0]"},
	}, {
		name:     "a modifier extension",
		resource: `{"resourceType":"Patient",` + text + `,"modifierExtension":[` + ext("y") + `]}`,
		want:     []string{"Patient.modifierExtension[0]"},
	}, {
		// The slice's profile, short-x, governs the extension, and so does the definition its url
		// names, starts-with-x, which short-x does not derive from: each one's invariant fails.
		name:     "a slice that declares another profile for the url",
		resource: `{"resourceType":"Patient","meta":{"profile":["` + shortXSliceURL + `"]},` + text + `,"extension":[` + ext("yyyy") + `]}`,
		want:     []string{"Patient.extension[0]", "sx-1 Patient.extension[0]"},
	}, {
		name:     "an extension that meets its definition",
		resource: `{"resourceType":"Patient",` + text + `,"extension":[` + ext("xy") + `]}`,
	}, {
		// An Attachment's url names no definition of its own type, even when it names a
		// StructureDefinition.
		name:     "a url that names a definition of another type",
		resource: `{"resourceType":"Patient",` + text + `,"photo":[{"contentType":"image/png","url":"` + startsWithXURL + `"}]}`,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				switch {
				case strings.Contains(is.Diagnostics, "swx-1:"):
					got = append(got, strings.Join(is.Expression, ","))
				case strings.Contains(is.Diagnostics, "sx-1:"):
					got = append(got, "sx-1 "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("swx-1 reported at %q, want %q", got, tt.want)
			}
		})
	}
}

// An extension's cardinality and slicing are checked against the definition its url names, with or
// without a profile (plan B, PR B4c): requires-a requires a sub-extension a and allows no value.
func TestExtensionDefinitionGivesTheStructure(t *testing.T) {
	v := profileValidator(t)
	const text = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`
	for _, tt := range []struct {
		name, profile, extension string
		element                  string   // where the extension is, with %s for it; Patient.extension when ""
		want                     []string // "<message ID> @ <location>"
	}{{
		name:      "a required sub-extension that is missing",
		extension: `{"url":"` + requiresAURL + `","extension":[{"url":"b","valueString":"1"}]}`,
		want:      []string{"SLICING_CARDINALITY_MIN @ Patient.extension[0].extension:a"},
	}, {
		name:      "a value the definition does not allow",
		extension: `{"url":"` + requiresAURL + `","valueString":"v","extension":[{"url":"a","valueString":"1"}]}`,
		want:      []string{"CARDINALITY_MAX @ Patient.extension[0].value[x]"},
	}, {
		name:      "on a primitive",
		element:   `"birthDate":"2000-01-01","_birthDate":{"extension":[` + "%s" + `]}`,
		extension: `{"url":"` + requiresAURL + `","valueString":"v","extension":[{"url":"b","valueString":"1"}]}`,
		want: []string{"CARDINALITY_MAX @ Patient.birthDate.extension[0].value[x]",
			"SLICING_CARDINALITY_MIN @ Patient.birthDate.extension[0].extension:a"},
	}, {
		name:      "in a data type",
		element:   `"name":[{"family":"A","extension":[` + "%s" + `]}]`,
		extension: `{"url":"` + requiresAURL + `","valueString":"v","extension":[{"url":"b","valueString":"1"}]}`,
		want: []string{"CARDINALITY_MAX @ Patient.name[0].extension[0].value[x]",
			"SLICING_CARDINALITY_MIN @ Patient.name[0].extension[0].extension:a"},
	}, {
		// The slice declares the definition the url names: the value's children are checked
		// against it once, not once more for the slice.
		name:      "in a slice that declares the same definition",
		profile:   requiresASlice,
		extension: `{"url":"` + requiresAURL + `","valueString":"v","extension":[{"url":"a","valueString":"1"}]}`,
		want:      []string{"CARDINALITY_MAX @ Patient.extension[0].value[x]"},
	}, {
		name:      "an extension that meets its definition",
		extension: `{"url":"` + requiresAURL + `","extension":[{"url":"a","valueString":"1"}]}`,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			meta := ""
			if tt.profile != "" {
				meta = `"meta":{"profile":["` + tt.profile + `"]},`
			}
			element := `"extension":[` + tt.extension + `]`
			if tt.element != "" {
				element = fmt.Sprintf(tt.element, tt.extension)
			}
			res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Patient",`+meta+text+`,`+element+`}`))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "CARDINALITY_") || strings.HasPrefix(is.MessageID, "SLICING_") {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("structure issues %q, want %q", got, tt.want)
			}
		})
	}
}

// A profile that requires an extension on a primitive is checked even when the primitive has no
// "_key" sibling to hold it, as the HL7 validator checks it.
func TestRequiredExtensionOnAPrimitive(t *testing.T) {
	v := profileValidator(t)
	const text = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`
	const birthTime = `"_birthDate":{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/patient-birthTime","valueDateTime":"2000-01-01T10:00:00Z"}]}`
	for _, tt := range []struct {
		name, profile, sibling string
		want                   []string
	}{
		{"a required slice", birthTimeSlice, "", []string{"SLICING_CARDINALITY_MIN @ Patient.birthDate.extension:birthTime"}},
		{"a required slice that is there", birthTimeSlice, birthTime, nil},
		{"a required extension", birthDateExt, "", []string{"CARDINALITY_MIN @ Patient.birthDate.extension"}},
		{"a required extension that is there", birthDateExt, birthTime, nil},
		// The value is the primitive's own property (json.html#primitive); the HL7 validator
		// counts it as absent (declared divergence B-D1).
		{"a required value that is there", birthDateValue, "", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sibling := ""
			if tt.sibling != "" {
				sibling = "," + tt.sibling
			}
			res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Patient","meta":{"profile":["`+tt.profile+`"]},`+
				text+`,"birthDate":"2000-01-01"`+sibling+`}`))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "CARDINALITY_") || strings.HasPrefix(is.MessageID, "SLICING_") {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("issues %q, want %q", got, tt.want)
			}
		})
	}
}

// A profile that requires a primitive's value is met by the primitive's own property, and not by
// its extensions (json.html#primitive).
func TestRequiredValueOfAPrimitive(t *testing.T) {
	v := profileValidator(t)
	const text = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`
	res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Patient","meta":{"profile":["`+birthDateValue+`"]},`+text+
		`,"_birthDate":{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"unknown"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range res.Issues {
		if strings.HasPrefix(is.MessageID, "CARDINALITY_") {
			got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
		}
	}
	if want := []string{"CARDINALITY_MIN @ Patient.birthDate.value"}; !slices.Equal(got, want) {
		t.Errorf("issues %q, want %q", got, want)
	}
}
