package slicing

import (
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/specs"
)

// loadWith returns a slicing validator over the embedded R4 packages plus the given resources.
func loadWith(t *testing.T, resources ...string) (*Validator, *registry.Registry) {
	t.Helper()
	l := loader.NewLoader("")
	core, err := l.LoadFromEmbeddedData(specs.GetPackages("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	raw := make([][]byte, len(resources))
	for i, r := range resources {
		raw[i] = []byte(r)
	}
	p, err := l.LoadFromResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	reg := registry.New()
	if err := reg.LoadFromPackages(append(core, p)); err != nil {
		t.Fatal(err)
	}
	return New(reg), reg
}

// profileOf builds a constraint profile of a resource type from snapshot elements after the root.
func profileOf(url, typ string, elements ...string) string {
	return `{"resourceType":"StructureDefinition","url":"` + url + `","name":"P","type":"` + typ + `","kind":"resource",
"derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/` + typ + `","snapshot":{"element":[
{"id":"` + typ + `","path":"` + typ + `","min":0,"max":"*"},` + strings.Join(elements, ",") + `]}}`
}

func check(t *testing.T, v *Validator, reg *registry.Registry, url string, cases []struct {
	name, resource string
	want           []string
}) {
	t.Helper()
	sd := reg.GetByURL(url)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := slicingErrors(t, v, sd, tt.resource)
			if !slices.Equal(got, tt.want) {
				t.Errorf("errors %v, want %v", got, tt.want)
			}
		})
	}
}

// Ordered and openAtEnd, per level: the order is compared with the element before (reset by one in
// no slice), openAtEnd is reported once, and the rules of a resliced slice apply to its members.
func TestSlicingRulesPerLevel(t *testing.T) {
	const url = "https://example.org/fhir/StructureDefinition/rules"
	v, reg := loadWith(t, profileOf(url, "Patient",
		`{"id":"Patient.identifier","path":"Patient.identifier","slicing":{"discriminator":[{"type":"value","path":"system"}],"ordered":true,"rules":"open"},"type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:a","path":"Patient.identifier","sliceName":"a","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"value"}],"rules":"closed"},"type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:a.system","path":"Patient.identifier.system","min":1,"max":"1","fixedUri":"urn:a","type":[{"code":"uri"}]}`,
		`{"id":"Patient.identifier:a.value","path":"Patient.identifier.value","min":0,"max":"1","type":[{"code":"string"}]}`,
		`{"id":"Patient.identifier:a/one","path":"Patient.identifier","sliceName":"a/one","min":1,"max":"1","type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:a/one.system","path":"Patient.identifier.system","min":1,"max":"1","fixedUri":"urn:a","type":[{"code":"uri"}]}`,
		`{"id":"Patient.identifier:a/one.value","path":"Patient.identifier.value","min":1,"max":"1","fixedString":"1","type":[{"code":"string"}]}`,
		`{"id":"Patient.identifier:b","path":"Patient.identifier","sliceName":"b","min":0,"max":"1","type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:b.system","path":"Patient.identifier.system","min":1,"max":"1","fixedUri":"urn:b","type":[{"code":"uri"}]}`,
		`{"id":"Patient.identifier:c","path":"Patient.identifier","sliceName":"c","min":0,"max":"*","type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:c.system","path":"Patient.identifier.system","min":1,"max":"1","fixedUri":"urn:c","type":[{"code":"uri"}]}`,
		`{"id":"Patient.name","path":"Patient.name","slicing":{"discriminator":[{"type":"value","path":"use"}],"rules":"openAtEnd"},"type":[{"code":"HumanName"}]}`,
		`{"id":"Patient.name:u","path":"Patient.name","sliceName":"u","min":0,"max":"*","type":[{"code":"HumanName"}]}`,
		`{"id":"Patient.name:u.use","path":"Patient.name.use","min":1,"max":"1","fixedCode":"usual","type":[{"code":"code"}]}`,
	))
	const a1, a2, b, c, x = `{"system":"urn:a","value":"1"}`, `{"system":"urn:a","value":"2"}`, `{"system":"urn:b"}`, `{"system":"urn:c"}`, `{"system":"urn:x"}`
	ids := func(items ...string) string {
		return `{"resourceType":"Patient","identifier":[` + strings.Join(items, ",") + `]}`
	}
	names := func(items ...string) string {
		return `{"resourceType":"Patient","name":[` + strings.Join(items, ",") + `]}`
	}
	const usual, other = `{"use":"usual"}`, `{"use":"official"}`
	check(t, v, reg, url, []struct {
		name, resource string
		want           []string
	}{
		{"in order, a slice repeated", ids(a1, a1, b, c, c), []string{"SLICING_CARDINALITY_MAX @ Patient.identifier:a/one"}},
		{"out of order, compared with the element before", ids(c, a1, b), []string{"SLICING_ORDER @ Patient.identifier[1]"}},
		{"an element in no slice restarts the order", ids(b, x, a1), nil},
		{"a slice over its maximum", ids(a1, b, b), []string{"SLICING_CARDINALITY_MAX @ Patient.identifier:b"}},
		// The reslice counts for its slice, and the slice's own rules apply to its members.
		{"a member of the reslice is a member of the slice", ids(a1), nil},
		{"closed reslicing: a member of the slice in no reslice", ids(a1, a2), []string{"SLICING_NO_MATCH @ Patient.identifier[1]"}},
		{"a required reslice missing from its slice", ids(a2, b), []string{"SLICING_CARDINALITY_MIN @ Patient.identifier:a/one", "SLICING_NO_MATCH @ Patient.identifier[0]"}},
		{"a reslice is not required where its slice is absent", ids(b), nil},
		// openAtEnd: reported once, at the first element in no slice that precedes one in a slice.
		{"openAtEnd, other content before two slices", names(other, usual, usual), []string{"SLICING_OPEN_AT_END @ Patient.name[0]"}},
		{"openAtEnd, interleaved", names(other, usual, other, usual), []string{"SLICING_OPEN_AT_END @ Patient.name[0]"}},
	})
}

// The walk reaches every instance a slicing can govern: through a contentReference (nested
// Questionnaire items), through a primitive's extensions (_birthDate), and into a complex
// extension's own definition (its sub-extension slices).
func TestSlicingWalkReach(t *testing.T) {
	const qURL, pURL, extURL = "https://example.org/fhir/StructureDefinition/q", "https://example.org/fhir/StructureDefinition/p", "https://example.org/fhir/StructureDefinition/race"
	ext := `{"id":"X","path":"X.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]}`
	_ = ext
	complexExt := `{"resourceType":"StructureDefinition","url":"` + extURL + `","name":"Race","type":"Extension","kind":"complex-type",
"derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","snapshot":{"element":[
{"id":"Extension","path":"Extension","min":0,"max":"*"},
{"id":"Extension.extension","path":"Extension.extension","min":1,"max":"*","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]},
{"id":"Extension.extension:text","path":"Extension.extension","sliceName":"text","min":1,"max":"1","type":[{"code":"Extension"}]},
{"id":"Extension.extension:text.url","path":"Extension.extension.url","min":1,"max":"1","fixedUri":"text","type":[{"code":"uri"}]},
{"id":"Extension.extension:text.value[x]","path":"Extension.extension.value[x]","min":1,"max":"1","type":[{"code":"string"}]},
{"id":"Extension.url","path":"Extension.url","min":1,"max":"1","fixedUri":"` + extURL + `","type":[{"code":"uri"}]},
{"id":"Extension.value[x]","path":"Extension.value[x]","min":0,"max":"0","type":[{"code":"string"}]}]}}`
	v, reg := loadWith(t,
		profileOf(qURL, "Questionnaire",
			`{"id":"Questionnaire.item","path":"Questionnaire.item","min":0,"max":"*","type":[{"code":"BackboneElement"}]}`,
			`{"id":"Questionnaire.item.extension","path":"Questionnaire.item.extension","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]}`,
			`{"id":"Questionnaire.item.extension:hidden","path":"Questionnaire.item.extension","sliceName":"hidden","min":1,"max":"1","type":[{"code":"Extension"}]}`,
			`{"id":"Questionnaire.item.extension:hidden.url","path":"Questionnaire.item.extension.url","min":1,"max":"1","fixedUri":"urn:hidden","type":[{"code":"uri"}]}`,
			`{"id":"Questionnaire.item.linkId","path":"Questionnaire.item.linkId","min":1,"max":"1","type":[{"code":"string"}]}`,
			`{"id":"Questionnaire.item.item","path":"Questionnaire.item.item","min":0,"max":"*","contentReference":"#Questionnaire.item"}`),
		profileOf(pURL, "Patient",
			`{"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]}`,
			`{"id":"Patient.extension:race","path":"Patient.extension","sliceName":"race","min":0,"max":"1","type":[{"code":"Extension","profile":["`+extURL+`"]}]}`,
			`{"id":"Patient.birthDate","path":"Patient.birthDate","min":0,"max":"1","type":[{"code":"date"}]}`,
			`{"id":"Patient.birthDate.extension","path":"Patient.birthDate.extension","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]}`,
			`{"id":"Patient.birthDate.extension:bt","path":"Patient.birthDate.extension","sliceName":"bt","min":1,"max":"1","type":[{"code":"Extension"}]}`,
			`{"id":"Patient.birthDate.extension:bt.url","path":"Patient.birthDate.extension.url","min":1,"max":"1","fixedUri":"urn:bt","type":[{"code":"uri"}]}`),
		complexExt,
	)
	hidden := `"extension":[{"url":"urn:hidden"}]`
	check(t, v, reg, qURL, []struct {
		name, resource string
		want           []string
	}{
		{"nested items through the contentReference", `{"resourceType":"Questionnaire","status":"draft","item":[{"linkId":"1",` + hidden + `,"item":[{"linkId":"2",` + hidden + `}]}]}`, nil},
		{"a nested item misses the required extension", `{"resourceType":"Questionnaire","status":"draft","item":[{"linkId":"1",` + hidden + `,"item":[{"linkId":"2"}]}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Questionnaire.item[0].item[0].extension:hidden"}},
	})
	bt := `{"url":"urn:bt","valueDateTime":"1970-01-01T10:00:00Z"}`
	check(t, v, reg, pURL, []struct {
		name, resource string
		want           []string
	}{
		{"a primitive's extension slice present", `{"resourceType":"Patient","birthDate":"1970-01-01","_birthDate":{"extension":[` + bt + `]}}`, nil},
		{"a primitive's extension slice missing", `{"resourceType":"Patient","birthDate":"1970-01-01","_birthDate":{"extension":[{"url":"urn:other","valueString":"x"}]}}`,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.birthDate.extension:bt"}},
		{"a primitive's extension slice twice", `{"resourceType":"Patient","birthDate":"1970-01-01","_birthDate":{"extension":[` + bt + `,` + bt + `]}}`,
			[]string{"SLICING_CARDINALITY_MAX @ Patient.birthDate.extension:bt"}},
		{"a complex extension with its sub-extension", `{"resourceType":"Patient","extension":[{"url":"` + extURL + `","extension":[{"url":"text","valueString":"x"}]}]}`, nil},
		{"a complex extension misses its sub-extension", `{"resourceType":"Patient","extension":[{"url":"` + extURL + `","extension":[{"url":"other","valueString":"x"}]}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.extension[0].extension:text"}},
	})
}

// A slice child required no more than the unsliced element is left to the cardinality phase, so
// a missing child is reported once; a choice value of another type counts as present.
func TestSliceChildrenAgainstTheBase(t *testing.T) {
	const url = "https://example.org/fhir/StructureDefinition/base"
	v, reg := loadWith(t, profileOf(url, "Patient",
		`{"id":"Patient.contact","path":"Patient.contact","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"gender"}],"rules":"open"},"type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.contact.gender","path":"Patient.contact.gender","min":0,"max":"1","type":[{"code":"code"}]}`,
		`{"id":"Patient.contact.name","path":"Patient.contact.name","min":1,"max":"1","type":[{"code":"HumanName"}]}`,
		`{"id":"Patient.contact.organization","path":"Patient.contact.organization","min":0,"max":"1","type":[{"code":"Reference"}]}`,
		`{"id":"Patient.contact:f","path":"Patient.contact","sliceName":"f","min":0,"max":"*","type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.contact:f.gender","path":"Patient.contact.gender","min":1,"max":"1","fixedCode":"female","type":[{"code":"code"}]}`,
		`{"id":"Patient.contact:f.name","path":"Patient.contact.name","min":1,"max":"1","type":[{"code":"HumanName"}]}`,
		`{"id":"Patient.contact:f.name.given","path":"Patient.contact.name.given","min":0,"max":"1","type":[{"code":"string"}]}`,
		`{"id":"Patient.contact:f.organization","path":"Patient.contact.organization","min":1,"max":"1","type":[{"code":"Reference"}]}`,
		`{"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]}`,
		`{"id":"Patient.extension:q","path":"Patient.extension","sliceName":"q","min":0,"max":"1","type":[{"code":"Extension"}]}`,
		`{"id":"Patient.extension:q.url","path":"Patient.extension.url","min":1,"max":"1","fixedUri":"urn:q","type":[{"code":"uri"}]}`,
		`{"id":"Patient.extension:q.value[x]","path":"Patient.extension.value[x]","min":1,"max":"1","type":[{"code":"Quantity"}]}`,
	))
	check(t, v, reg, url, []struct {
		name, resource string
		want           []string
	}{
		// name is required by the base as much as by the slice: the cardinality phase reports it.
		{"a child the base requires as much", `{"resourceType":"Patient","contact":[{"gender":"female","organization":{"reference":"Organization/o"}}]}`, nil},
		{"a child the slice requires more", `{"resourceType":"Patient","contact":[{"gender":"female","name":{"family":"x"}}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.contact[0].organization"}},
		{"a repeating primitive present only through its extensions counts each entry",
			`{"resourceType":"Patient","contact":[{"gender":"female","organization":{"reference":"Organization/o"},"name":{"_given":[{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"masked"}]},{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"masked"}]}]}}]}`,
			[]string{"SLICING_CARDINALITY_MAX @ Patient.contact[0].name.given"}},
		{"a choice value of a type the slice does not allow is present", `{"resourceType":"Patient","extension":[{"url":"urn:q","valueString":"x"}]}`, nil},
	})
}
