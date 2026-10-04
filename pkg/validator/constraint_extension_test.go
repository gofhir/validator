package validator

import (
	"context"
	"slices"
	"strings"
	"testing"
)

const (
	startsWithXURL = "https://example.org/fhir/StructureDefinition/starts-with-x"
	shortXURL      = "https://example.org/fhir/StructureDefinition/short-x"
	shortXSliceURL = "https://example.org/fhir/StructureDefinition/short-x-slice"
)

// extensionDefinitions are the extension definitions the constraint walk is tested with:
// starts-with-x requires a string value that starts with "x" (swx-1); short-x, for the same url but
// derived from Extension, not from starts-with-x, requires at most 3 characters (sx-1);
// short-x-slice is a Patient profile with a slice of Patient.extension that declares short-x.
var extensionDefinitions = [][]byte{[]byte(`{"resourceType":"StructureDefinition","url":"` + shortXURL + `",
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
