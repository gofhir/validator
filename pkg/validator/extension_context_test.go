package validator

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const contextExtBase = "https://example.org/fhir/StructureDefinition/ctx-"

// contextExtension is an extension definition with a string value and the given contexts of use
// and context invariants.
func contextExtension(id string, contexts []map[string]string, invariants ...string) []byte {
	def := map[string]any{
		"resourceType": "StructureDefinition", "url": contextExtBase + id, "name": "Ctx" + strings.ReplaceAll(id, "-", ""),
		"status": "active", "fhirVersion": "4.0.1", "kind": "complex-type", "abstract": false, "type": "Extension",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Extension", "derivation": "constraint",
		"differential": map[string]any{"element": []any{
			map[string]any{"id": "Extension", "path": "Extension"},
			map[string]any{"id": "Extension.url", "path": "Extension.url", "fixedUri": contextExtBase + id},
		}},
	}
	if contexts != nil {
		def["context"] = contexts
	}
	if len(invariants) > 0 {
		def["contextInvariant"] = invariants
	}
	b, err := json.Marshal(def)
	if err != nil {
		panic(err)
	}
	return b
}

func elementContext(expression string) []map[string]string {
	return []map[string]string{{"type": "element", "expression": expression}}
}

func fhirpathContext(expression string) []map[string]string {
	return []map[string]string{{"type": "fhirpath", "expression": expression}}
}

// contextExtensions are the extensions the context of use is tested with, one per rule.
var contextExtensions = [][]byte{
	contextExtension("patient", elementContext("Patient")),
	contextExtension("given", elementContext("HumanName.given")),
	contextExtension("string", elementContext("string")),
	contextExtension("element", elementContext("Element")),
	contextExtension("domainresource", elementContext("DomainResource")),
	contextExtension("text", elementContext("DomainResource.text")),
	contextExtension("path", elementContext("Patient.name.given")),
	contextExtension("parent", elementContext("Patient")),
	contextExtension("child", []map[string]string{{"type": "extension", "expression": contextExtBase + "parent"}}),
	contextExtension("versioned-child", []map[string]string{{"type": "extension", "expression": contextExtBase + "parent|1.0.0"}}),
	contextExtension("official", fhirpathContext("Patient.name.where(use = 'official')")),
	contextExtension("official-given", fhirpathContext("Patient.name.where(use = 'official').given")),
	contextExtension("boolean", fhirpathContext("Patient.name.exists()")),
	contextExtension("contained-name", fhirpathContext("%rootResource.contained.ofType(Patient).name")),
	contextExtension("in-unknown", []map[string]string{{"type": "extension", "expression": "https://example.org/unknown-holder"}}),
	contextExtension("family-a", elementContext("HumanName"), "family.startsWith('a')"),
	contextExtension("with-id", elementContext("string"), "id.exists()"),
	contextExtension("with-family", elementContext("HumanName"), "family"),
	contextExtension("date-time", elementContext("Patient.deceased[x]"), "$this is dateTime"),
	contextExtension("two-invariants", elementContext("HumanName"), "family.exists()", "given.exists()"),
	contextExtension("decimal-text", elementContext("Quantity"), "value.toString() = '1.50'"),
	contextExtension("is-date", elementContext("Patient.birthDate"), "$this is date"),
	contextExtension("resolvable", elementContext("Reference"), "resolve().exists()"),
	contextExtension("none", nil),
	contextExtension("gendered", elementContext("Patient"), "gender.exists()"),
}

// An extension is used only on a target its context of use names (defining-extensions.html#context):
// an element context names the element the target instantiates, its type or one of the type's
// ancestors, or its path through data types; an extension context names the extension that holds
// it; and every context invariant holds on the target.
func TestExtensionContextOfUse(t *testing.T) {
	v := profileValidator(t)
	const text = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`
	ext := func(id string) string { return `{"url":"` + contextExtBase + id + `","valueString":"v"}` }
	onRoot := func(id string) string { return `"extension":[` + ext(id) + `]` }
	onName := func(id string) string { return `"name":[{"family":"A","extension":[` + ext(id) + `]}]` }
	onGiven := func(id string) string { return `"name":[{"given":["B"],"_given":[{"extension":[` + ext(id) + `]}]}]` }
	for _, tt := range []struct {
		name, content string
		want          []string // "<message ID> @ <location>"
	}{
		{"a resource on its root", onRoot("patient"), nil},
		{"a resource on an element of it", onName("patient"), []string{"EXTENSION_INVALID_CONTEXT @ Patient.name[0]"}},
		{"a data type's element", onGiven("given"), nil},
		{"a data type's element elsewhere", onName("given"), []string{"EXTENSION_INVALID_CONTEXT @ Patient.name[0]"}},
		{"a primitive type", onGiven("string"), nil},
		{"Element on an element", onName("element"), nil},
		{"Element on a resource's root (B-D2)", onRoot("element"), []string{"EXTENSION_INVALID_CONTEXT @ Patient"}},
		{"an ancestor of the resource's type", onRoot("domainresource"), nil},
		{"an ancestor of the resource's type on an element", onName("domainresource"), []string{"EXTENSION_INVALID_CONTEXT @ Patient.name[0]"}},
		{"the element an element is based on", `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>","extension":[` + ext("text") + `]}`, nil},
		{"a path through a data type (B-D4)", onGiven("path"), nil},
		{"a path through a data type elsewhere", onName("path"), []string{"EXTENSION_INVALID_CONTEXT @ Patient.name[0]"}},
		{"the extension that holds it", `"extension":[{"url":"` + contextExtBase + `parent","extension":[` + ext("child") + `]}]`, nil},
		{"outside the extension it names", onRoot("child"), []string{"EXTENSION_INVALID_CONTEXT @ Patient"}},
		{"the extension a versioned canonical names", `"extension":[{"url":"` + contextExtBase + `parent","extension":[` + ext("versioned-child") + `]}]`, nil},
		{"a node a fhirpath context selects", `"name":[{"use":"official","family":"A","extension":[` + ext("official") + `]}]`, nil},
		{"an equal node a fhirpath context does not select",
			`"name":[{"use":"official","family":"A"}],"contact":[{"name":{"use":"official","family":"A","extension":[` + ext("official") + `]}}]`,
			[]string{"EXTENSION_INVALID_CONTEXT @ Patient.contact[0].name"}},
		{"the second of two equal nodes a fhirpath context selects one of",
			`"name":[{"use":"official","family":"A"},{"use":"usual","family":"A","extension":[` + ext("official") + `]}]`,
			[]string{"EXTENSION_INVALID_CONTEXT @ Patient.name[1]"}},
		{"a primitive's element a fhirpath context selects",
			`"name":[{"use":"usual","given":["B"]},{"use":"official","given":["B","C"],"_given":[null,{"extension":[` + ext("official-given") + `]}]}]`, nil},
		{"a primitive's element a fhirpath context does not select",
			`"name":[{"use":"usual","given":["B","C"],"_given":[null,{"extension":[` + ext("official-given") + `]}]},` +
				`{"use":"official","given":["B","C"],"_given":[{"id":"b"},{"id":"c"}]}]`,
			[]string{"EXTENSION_INVALID_CONTEXT @ Patient.name[0].given[1]"}},
		{"a fhirpath context that selects no node", `"name":[{"family":"A"}],` + onRoot("boolean"), []string{"EXTENSION_INVALID_CONTEXT @ Patient"}},
		{"%rootResource of a contained resource, its container",
			`"contained":[{"resourceType":"Patient","id":"c","name":[{"family":"A","extension":[` + ext("contained-name") + `]}]}],` +
				`"link":[{"other":{"reference":"#c"},"type":"seealso"}],"name":[{"family":"A","extension":[` + ext("contained-name") + `]}]`,
			[]string{"EXTENSION_INVALID_CONTEXT @ Patient.name[0]"}},
		{"inside an extension whose definition is not loaded", `"extension":[{"url":"https://example.org/unknown-holder","extension":[` + ext("in-unknown") + `]}]`, nil},
		{"a context invariant on the element that holds it", `"name":[{"family":"abc","extension":[` + ext("family-a") + `]}]`, nil},
		{"a context invariant with no result does not hold", onName("family-a") + `,"contact":[{"name":{"given":["B"],"extension":[` + ext("family-a") + `]}}]`,
			[]string{"EXTENSION_CONTEXT_INVARIANT @ Patient.contact[0].name", "EXTENSION_CONTEXT_INVARIANT @ Patient.name[0]"}},
		{"a context invariant whose result is not a boolean, not empty", `"name":[{"family":"A","extension":[` + ext("with-family") + `]}]`, nil},
		{"a context invariant whose result is not a boolean, empty", `"name":[{"given":["B"],"extension":[` + ext("with-family") + `]}]`,
			[]string{"EXTENSION_CONTEXT_INVARIANT @ Patient.name[0]"}},
		{"the first of two context invariants that do not hold", `"name":[{"text":"A","extension":[` + ext("two-invariants") + `]}]`,
			[]string{"EXTENSION_CONTEXT_INVARIANT @ Patient.name[0]"}},
		{"a decimal as the JSON spells it",
			`"contained":[{"resourceType":"Observation","id":"o","status":"final","code":{"text":"x"},` +
				`"valueQuantity":{"value":1.50,"extension":[` + ext("decimal-text") + `]}}],` +
				`"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/patient-importance","valueReference":{"reference":"#o"}}]`, nil},
		{"a primitive with no value, typed as the primitive", `"_birthDate":{"extension":[` + ext("is-date") + `]}`, nil},
		{"a context invariant on a choice value, typed as its type", `"deceasedDateTime":"2020-01-01","_deceasedDateTime":{"extension":[` + ext("date-time") + `]}`, nil},
		{"a context invariant on a primitive, with its id", `"name":[{"given":["B"],"_given":[{"id":"g","extension":[` + ext("with-id") + `]}]}]`, nil},
		{"a context invariant on a primitive without an id", onGiven("with-id"), []string{"EXTENSION_CONTEXT_INVARIANT @ Patient.name[0].given[0]"}},
		{"a fhirpath context in a contained resource, from its root",
			`"contained":[{"resourceType":"Patient","id":"c","name":[{"use":"official","family":"A","extension":[` + ext("official") + `]}]}],` +
				`"link":[{"other":{"reference":"#c"},"type":"seealso"}]`, nil},
		{"no context", onRoot("none"), []string{"EXTENSION_INVALID_CONTEXT @ Patient"}},
		{"a context invariant that holds", `"gender":"male",` + onRoot("gendered"), nil},
		{"a context invariant that does not hold", onRoot("gendered"), []string{"EXTENSION_CONTEXT_INVARIANT @ Patient"}},
		{"in a contained resource", `"contained":[{"resourceType":"Patient","id":"c",` + onName("patient") + `}],"link":[{"other":{"reference":"#c"},"type":"seealso"}]`,
			[]string{"EXTENSION_INVALID_CONTEXT @ Patient.contained[0].name[0]"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.content
			if !strings.HasPrefix(content, `"text":`) {
				content = text + "," + content
			}
			res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Patient",`+content+`}`))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.MessageID == "EXTENSION_INVALID_CONTEXT" || is.MessageID == "EXTENSION_CONTEXT_INVARIANT" {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("context issues %q, want %q", got, tt.want)
			}
		})
	}
}

// A context invariant resolves references to the entries of the Bundle validated, as a profile's
// invariant does.
func TestContextInvariantResolvesInTheBundle(t *testing.T) {
	v := profileValidator(t)
	entry := func(fullURL, resource string) string {
		return `{"fullUrl":"` + fullURL + `","resource":` + resource + `,"request":{"method":"POST","url":"x"}}`
	}
	subject := func(reference string) string {
		holds := "" // a resource it contains, for the fragment reference "#c"
		if reference == "#c" {
			holds = `"contained":[{"resourceType":"Patient","id":"c"}],`
		}
		return `{"resourceType":"Observation",` + holds + `"status":"final","code":{"text":"x"},` +
			`"subject":{"reference":"` + reference + `","extension":[{"url":"` + contextExtBase + `resolvable","valueString":"v"}]}}`
	}
	inner := func(reference string) string {
		return `{"resourceType":"Bundle","type":"collection","entry":[` +
			entry("urn:uuid:1b2c3d4e-0000-4000-8000-000000000001", `{"resourceType":"Patient","id":"p2"}`) + `,` +
			entry("urn:uuid:1b2c3d4e-0000-4000-8000-000000000002", subject(reference)) + `]}`
	}
	for _, tt := range []struct {
		name, reference, nested string
		first                   string // the first entry's resource; a Patient when empty
		want                    []string
	}{
		{"an entry of the Bundle", "urn:uuid:9d0a6c5e-3a2b-4c5d-8e7f-001122334455", "", "", nil},
		{"no entry of the Bundle", "urn:uuid:00000000-0000-0000-0000-000000000000", "", "",
			[]string{"EXTENSION_CONTEXT_INVARIANT @ Bundle.entry[1].resource.subject"}},
		{"an entry of the Bundle an entry holds", "", "urn:uuid:1b2c3d4e-0000-4000-8000-000000000001", "", nil},
		{"an entry of the Bundle that holds it", "", "urn:uuid:9d0a6c5e-3a2b-4c5d-8e7f-001122334455", "", nil},
		{"a resource the referring resource contains", "#c", "", "", nil},
		{"a resource another entry contains", "#d", "",
			`{"resourceType":"Observation","contained":[{"resourceType":"Patient","id":"d"}],"status":"final","code":{"text":"x"},"subject":{"reference":"#d"}}`,
			[]string{"EXTENSION_CONTEXT_INVARIANT @ Bundle.entry[1].resource.subject"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			second := subject(tt.reference)
			if tt.nested != "" {
				second = inner(tt.nested)
			}
			first := tt.first
			if first == "" {
				first = `{"resourceType":"Patient","id":"p1"}`
			}
			bundle := `{"resourceType":"Bundle","type":"transaction","entry":[` +
				entry("urn:uuid:9d0a6c5e-3a2b-4c5d-8e7f-001122334455", first) + `,` +
				entry("urn:uuid:5f4e3d2c-1b0a-4987-8654-aabbccddeeff", second) + `]}`
			res, err := v.Validate(context.Background(), []byte(bundle))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.MessageID == "EXTENSION_INVALID_CONTEXT" || is.MessageID == "EXTENSION_CONTEXT_INVARIANT" {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("context issues %q, want %q", got, tt.want)
			}
		})
	}
}

// An extension phase finding is reported once, however many profiles the resource declares: it
// depends on none of them.
func TestExtensionIssuesOncePerValidation(t *testing.T) {
	v := profileValidator(t)
	res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Patient","meta":{"profile":["`+treeA+`","`+treeB+`"]},`+
		`"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},"name":[{"family":"A"}],`+
		`"extension":[{"url":"`+contextExtBase+`none","valueString":"v"},{"url":"https://example.org/unknown","valueString":"v"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, is := range res.Issues {
		if strings.HasPrefix(is.MessageID, "EXTENSION_") {
			counts[is.MessageID]++
		}
	}
	for _, id := range []string{"EXTENSION_INVALID_CONTEXT", "EXTENSION_UNKNOWN"} {
		if counts[id] != 1 {
			t.Errorf("%s reported %d times, want once: %v", id, counts[id], counts)
		}
	}
}
