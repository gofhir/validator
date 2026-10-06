package validator

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// constraintProfile is a profile of Observation with one invariant on value[x], from its
// differential.
func constraintProfile(url, key, expression string) []byte {
	return []byte(`{"resourceType":"StructureDefinition","url":"` + url + `","name":"P","status":"active",` +
		`"fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Observation",` +
		`"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation","derivation":"constraint",` +
		`"differential":{"element":[{"id":"Observation","path":"Observation"},` +
		`{"id":"Observation.value[x]","path":"Observation.value[x]","constraint":[{"key":"` + key + `",` +
		`"severity":"error","human":"` + key + `","expression":"` + strings.ReplaceAll(expression, `"`, `\"`) + `"}]}]}}`)
}

// A profile's invariant reads a value below the resource's root as the JSON writes it: a decimal
// keeps its text (1.50 is not 1.5, json.html#primitive), in the resource validated and in one an
// element holds.
func TestProfileInvariantReadsTheDecimalAsWritten(t *testing.T) {
	const url = "https://example.org/fhir/StructureDefinition/obs-150"
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{
		constraintProfile(url, "x-150", "$this.ofType(Quantity).value.toString() = '1.50'"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	obs := func(value string) string {
		return `{"resourceType":"Observation","meta":{"profile":["` + url + `"]},"status":"final","code":{"text":"x"},` +
			`"valueQuantity":{"value":` + value + `}}`
	}
	for _, tt := range []struct {
		name, resource string
		fails          bool
	}{
		{"the resource validated, 1.50", obs("1.50"), false},
		{"the resource validated, 1.5", obs("1.5"), true},
		{"an entry of a Bundle, 1.50", `{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"urn:uuid:9d0a6c5e-3a2b-4c5d-8e7f-001122334455","resource":` + obs("1.50") + `}]}`, false},
		{"an entry of a Bundle, 1.5", `{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"urn:uuid:9d0a6c5e-3a2b-4c5d-8e7f-001122334455","resource":` + obs("1.5") + `}]}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			if got := failed(res, "x-150"); got != tt.fails {
				t.Errorf("x-150 failed: %v, want %v", got, tt.fails)
			}
		})
	}
}

// resolve() in a profile's invariant looks for a reference in the Bundle that holds the resource
// first, then in the Bundles that hold that one, as the HL7 validator looks.
func TestProfileInvariantResolvesInTheInnermostBundle(t *testing.T) {
	const url = "https://example.org/fhir/StructureDefinition/obs-members"
	sd := []byte(`{"resourceType":"StructureDefinition","url":"` + url + `","name":"M","status":"active",` +
		`"fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Observation",` +
		`"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation","derivation":"constraint",` +
		`"differential":{"element":[{"id":"Observation","path":"Observation","constraint":[{"key":"x-members",` +
		`"severity":"error","human":"members resolve","expression":"hasMember.all(resolve().exists())"}]}]}}`)
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{sd}))
	if err != nil {
		t.Fatal(err)
	}
	entry := func(id, resource string) string {
		return `{"fullUrl":"urn:uuid:00000000-0000-4000-8000-00000000000` + id + `","resource":` + resource + `}`
	}
	member := func(ref string) string {
		return `{"resourceType":"Observation","meta":{"profile":["` + url + `"]},"status":"final","code":{"text":"x"},` +
			`"hasMember":[{"reference":"urn:uuid:00000000-0000-4000-8000-00000000000` + ref + `"}]}`
	}
	const target = `{"resourceType":"Observation","status":"final","code":{"text":"x"}}`
	for _, tt := range []struct {
		name, resource string
		fails          bool
	}{
		{"a sibling entry of the inner Bundle", `{"resourceType":"Bundle","type":"collection","entry":[` +
			entry("1", `{"resourceType":"Bundle","type":"collection","entry":[`+entry("2", target)+`,`+entry("3", member("2"))+`]}`) + `]}`, false},
		{"an entry of the outer Bundle", `{"resourceType":"Bundle","type":"collection","entry":[` + entry("2", target) + `,` +
			entry("1", `{"resourceType":"Bundle","type":"collection","entry":[`+entry("3", member("2"))+`]}`) + `]}`, false},
		{"no entry", `{"resourceType":"Bundle","type":"collection","entry":[` +
			entry("1", `{"resourceType":"Bundle","type":"collection","entry":[`+entry("3", member("9"))+`]}`) + `]}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			if got := failed(res, "x-members"); got != tt.fails {
				t.Errorf("x-members failed: %v, want %v", got, tt.fails)
			}
		})
	}
}

// failed reports whether the invariant key is reported failed in res.
func failed(res *issue.Result, key string) bool {
	for _, is := range res.Issues {
		if is.MessageID == string(issue.DiagConstraintFailed) && strings.Contains(is.Diagnostics, "Constraint failed: "+key+":") {
			return true
		}
	}
	return false
}

// resolve() in a Bundle that a resource with no Bundle around it holds (Parameters) returns what it
// finds as the JSON writes it.
func TestProfileInvariantResolvesTheDecimalInABundleParametersHold(t *testing.T) {
	const url = "https://example.org/fhir/StructureDefinition/obs-member-150"
	sd := []byte(`{"resourceType":"StructureDefinition","url":"` + url + `","name":"M150","status":"active",` +
		`"fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Observation",` +
		`"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation","derivation":"constraint",` +
		`"differential":{"element":[{"id":"Observation","path":"Observation","constraint":[{"key":"x-m150",` +
		`"severity":"error","human":"members are 1.50","expression":"hasMember.all(resolve().value.ofType(Quantity).value.toString() = '1.50')"}]}]}}`)
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{sd}))
	if err != nil {
		t.Fatal(err)
	}
	parameters := func(value string) string {
		return `{"resourceType":"Parameters","parameter":[{"name":"b","resource":{"resourceType":"Bundle","type":"collection","entry":[` +
			`{"fullUrl":"http://example.org/fhir/Observation/o1","resource":{"resourceType":"Observation","id":"o1","status":"final",` +
			`"code":{"text":"x"},"valueQuantity":{"value":` + value + `}}},` +
			`{"fullUrl":"http://example.org/fhir/Observation/o2","resource":{"resourceType":"Observation","id":"o2",` +
			`"meta":{"profile":["` + url + `"]},"status":"final","code":{"text":"x"},"hasMember":[{"reference":"Observation/o1"}]}}]}}]}`
	}
	for value, fails := range map[string]bool{"1.50": false, "1.5": true} {
		res, err := v.Validate(context.Background(), []byte(parameters(value)))
		if err != nil {
			t.Fatal(err)
		}
		if got := failed(res, "x-m150"); got != fails {
			t.Errorf("value %s: x-m150 failed: %v, want %v", value, got, fails)
		}
	}
}

// An invariant reads a decimal in a primitive's "_key" sibling, and in an array item, as the JSON
// writes it: the item at its position, nulls included.
func TestInvariantReadsDecimalsInKeysAndArraysAsWritten(t *testing.T) {
	const ext = "https://example.org/fhir/StructureDefinition/ext-150"
	const ms = "https://example.org/fhir/StructureDefinition/ms-250"
	extension := []byte(`{"resourceType":"StructureDefinition","url":"` + ext + `","name":"E150","status":"active",` +
		`"fhirVersion":"4.0.1","kind":"complex-type","abstract":false,"type":"Extension",` +
		`"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint",` +
		`"context":[{"type":"element","expression":"Element"}],` +
		`"differential":{"element":[{"id":"Extension","path":"Extension"},` +
		`{"id":"Extension.url","path":"Extension.url","fixedUri":"` + ext + `"},` +
		`{"id":"Extension.value[x]","path":"Extension.value[x]","type":[{"code":"decimal"}],` +
		`"constraint":[{"key":"x-e150","severity":"error","human":"1.50","expression":"$this.toString() = '1.50'"}]}]}}`)
	profile := []byte(`{"resourceType":"StructureDefinition","url":"` + ms + `","name":"MS250","status":"active",` +
		`"fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"MolecularSequence",` +
		`"baseDefinition":"http://hl7.org/fhir/StructureDefinition/MolecularSequence","derivation":"constraint",` +
		`"differential":{"element":[{"id":"MolecularSequence","path":"MolecularSequence"},` +
		`{"id":"MolecularSequence.quality.roc.precision","path":"MolecularSequence.quality.roc.precision",` +
		`"constraint":[{"key":"x-ms250","severity":"error","human":"2.50","expression":"$this.toString() = '2.50'"}]}]}}`)
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{extension, profile}))
	if err != nil {
		t.Fatal(err)
	}
	patient := func(value string) string {
		return `{"resourceType":"Patient","birthDate":"2000-01-01","_birthDate":{"extension":[{"url":"` + ext + `","valueDecimal":` + value + `}]}}`
	}
	sequence := `{"resourceType":"MolecularSequence","meta":{"profile":["` + ms + `"]},"coordinateSystem":0,` +
		`"quality":[{"type":"snp","roc":{"precision":[null,2.50],"_precision":[{"id":"x"},null]}}]}`
	for _, tt := range []struct {
		name, resource, key, at string
		fails                   bool
	}{
		{"a decimal in a _key sibling, 1.50", patient("1.50"), "x-e150", "", false},
		{"a decimal in a _key sibling, 1.5", patient("1.5"), "x-e150", "", true},
		{"the array item after a null", sequence, "x-ms250", "MolecularSequence.quality[0].roc.precision[1]", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			if got := failedAt(res, tt.key, tt.at); got != tt.fails {
				t.Errorf("%s failed: %v, want %v", tt.key, got, tt.fails)
			}
		})
	}
}

// failedAt reports whether the invariant key is reported failed in res, at at when it is not "".
func failedAt(res *issue.Result, key, at string) bool {
	for _, is := range res.Issues {
		if is.MessageID != string(issue.DiagConstraintFailed) || !strings.Contains(is.Diagnostics, "Constraint failed: "+key+":") {
			continue
		}
		if at == "" || slices.Contains(is.Expression, at) {
			return true
		}
	}
	return false
}
