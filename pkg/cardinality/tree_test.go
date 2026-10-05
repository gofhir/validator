package cardinality

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/internal/testfhir"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
)

// treeProfile is a Patient profile with two slices of contact whose children the base element
// does not unroll: the "f" slice requires organization. An instance that is not in that slice must
// not be held to it (D2: children were looked up by path, and one slice's governed every contact).
const treeProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/tree","name":"Tree",
"type":"Patient","kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient",
"snapshot":{"element":[
 {"id":"Patient","path":"Patient","min":0,"max":"*"},
 {"id":"Patient.contact","path":"Patient.contact","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"gender"}],"rules":"open"},"type":[{"code":"BackboneElement"}]},
 {"id":"Patient.contact:f","path":"Patient.contact","sliceName":"f","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
 {"id":"Patient.contact:f.gender","path":"Patient.contact.gender","min":1,"max":"1","fixedCode":"female","type":[{"code":"code"}]},
 {"id":"Patient.contact:f.organization","path":"Patient.contact.organization","min":1,"max":"1","type":[{"code":"Reference"}]},
 {"id":"Patient.deceased[x]","path":"Patient.deceased[x]","min":1,"max":"1","type":[{"code":"boolean"},{"code":"dateTime"}]},
 {"id":"Patient.name","path":"Patient.name","min":0,"max":"*","type":[{"code":"HumanName"}]},
 {"id":"Patient.name.family","path":"Patient.name.family","min":1,"max":"1","type":[{"code":"string"}]}
]}}`

func treeRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	l := loader.NewLoader("")
	p, err := l.LoadFromResources([][]byte{[]byte(treeProfile)})
	if err != nil {
		t.Fatal(err)
	}
	reg := testfhir.Registry(t, "4.0.1")
	if err := reg.LoadFromPackages([]*loader.Package{p}); err != nil {
		t.Fatal(err)
	}
	return reg
}

func cardinalityErrors(t *testing.T, v *Validator, sd *registry.StructureDefinition, resource string) []string {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(resource), &data); err != nil {
		t.Fatal(err)
	}
	res := v.ValidateData(data, sd)
	defer issue.ReleaseResult(res)
	out := make([]string, 0, len(res.Issues))
	for _, is := range res.Issues {
		out = append(out, is.MessageID+" @ "+strings.Join(is.Expression, ","))
	}
	slices.Sort(out)
	return out
}

func TestCardinalityOnTheTree(t *testing.T) {
	reg := treeRegistry(t)
	v := New(reg)
	sd := reg.GetByURL("https://example.org/fhir/StructureDefinition/tree")
	for _, tt := range []struct {
		name, resource string
		want           []string
	}{
		{"a contact outside the slice is not held to the slice's children (D2)",
			`{"resourceType":"Patient","deceasedBoolean":false,"contact":[{"gender":"male"}]}`, nil},
		{"a required choice is present under a typed name",
			`{"resourceType":"Patient","deceasedDateTime":"2020"}`, nil},
		{"a required choice with a value of a type it does not allow is present (the wrong type is another phase's)",
			`{"resourceType":"Patient","deceasedString":"x"}`, nil},
		{"a required choice is missing",
			`{"resourceType":"Patient"}`, []string{"CARDINALITY_MIN @ Patient.deceased[x]"}},
		{"a required primitive present only through its extensions",
			`{"resourceType":"Patient","deceasedBoolean":true,"name":[{"_family":{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"masked"}]}}]}`, nil},
		{"a repeating primitive present only through its extensions counts each entry",
			`{"resourceType":"Patient","deceasedBoolean":true,"name":[{"family":"A","_given":[{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"masked"}]},null]}]}`, nil},
		{"children of each array item are checked at their index",
			`{"resourceType":"Patient","deceasedBoolean":true,"name":[{"family":"A"},{"given":["B"]}]}`,
			[]string{"CARDINALITY_MIN @ Patient.name[1].family"}},
		{"a choice value is checked against its type",
			`{"resourceType":"Patient","deceasedBoolean":true,"contact":[{"period":{"start":"2020"},"telecom":[{"system":"phone","value":"1"}]}]}`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := cardinalityErrors(t, v, sd, tt.resource); !slices.Equal(got, tt.want) {
				t.Errorf("errors %v, want %v", got, tt.want)
			}
		})
	}
}

// Types are walked through their base definitions, the one being walked included.
func TestCardinalityThroughTypes(t *testing.T) {
	reg := treeRegistry(t)
	v := New(reg)
	patient := reg.GetByType("Patient")
	for _, tt := range []struct {
		name, resource string
		want           []string
	}{
		{"a nested extension is an Extension too",
			`{"resourceType":"Patient","extension":[{"url":"http://example.org/x","extension":[{"valueString":"a"}]}]}`,
			[]string{"CARDINALITY_MIN @ Patient.extension[0].extension[0].url"}},
		{"a multi-type choice value is checked against the type its key names",
			`{"resourceType":"Patient","extension":[{"url":"http://example.org/x","valueSignature":{}}]}`,
			[]string{"CARDINALITY_MIN @ Patient.extension[0].valueSignature.type", "CARDINALITY_MIN @ Patient.extension[0].valueSignature.when", "CARDINALITY_MIN @ Patient.extension[0].valueSignature.who"}},
		{"a resource inside an element is left to the walker, and reported once",
			`{"resourceType":"Patient","contained":[{"resourceType":"Observation","id":"o","code":{"text":"x"}}]}`,
			[]string{"CARDINALITY_MIN @ Patient.contained[0].status"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := cardinalityErrors(t, v, patient, tt.resource); !slices.Equal(got, tt.want) {
				t.Errorf("errors %v, want %v", got, tt.want)
			}
		})
	}
}

// A contentReference's children are checked (D6): Questionnaire.item.item is defined by
// #Questionnaire.item in R4 and by the absolute form in R5, and both require linkId and type.
func TestCardinalityFollowsContentReference(t *testing.T) {
	nested := `{"resourceType":"Questionnaire","status":"draft","item":[{"linkId":"1","type":"group","item":[{"text":"no linkId"}]}]}`
	for _, version := range []string{"4.0.1", "5.0.0"} {
		t.Run(version, func(t *testing.T) {
			reg := testfhir.Registry(t, version)
			got := cardinalityErrors(t, New(reg), reg.GetByType("Questionnaire"), nested)
			want := []string{"CARDINALITY_MIN @ Questionnaire.item[0].item[0].linkId", "CARDINALITY_MIN @ Questionnaire.item[0].item[0].type"}
			if !slices.Equal(got, want) {
				t.Errorf("errors %v, want %v", got, want)
			}
		})
	}
}
