package slicing

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/internal/testfhir"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// obsProfile slices Observation.component by code (a pattern), and inside each component slice its
// code.coding by code again, with a required slice fixing the code: the blood pressure shape. The
// choice value[x] is sliced by type, and the Quantity slice requires unit. The identifier slicing
// is ordered, and the category slicing is openAtEnd.
const obsProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/obs","name":"Obs",
"type":"Observation","kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation",
"snapshot":{"element":[
 {"id":"Observation","path":"Observation","min":0,"max":"*"},
 {"id":"Observation.identifier","path":"Observation.identifier","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"system"}],"ordered":true,"rules":"open"},"type":[{"code":"Identifier"}]},
 {"id":"Observation.identifier:first","path":"Observation.identifier","sliceName":"first","min":0,"max":"1","type":[{"code":"Identifier"}]},
 {"id":"Observation.identifier:first.system","path":"Observation.identifier.system","min":1,"max":"1","fixedUri":"urn:first","type":[{"code":"uri"}]},
 {"id":"Observation.identifier:second","path":"Observation.identifier","sliceName":"second","min":0,"max":"1","type":[{"code":"Identifier"}]},
 {"id":"Observation.identifier:second.system","path":"Observation.identifier.system","min":1,"max":"1","fixedUri":"urn:second","type":[{"code":"uri"}]},
 {"id":"Observation.category","path":"Observation.category","min":0,"max":"*","slicing":{"discriminator":[{"type":"pattern","path":"$this"}],"rules":"openAtEnd"},"type":[{"code":"CodeableConcept"}]},
 {"id":"Observation.category:vs","path":"Observation.category","sliceName":"vs","min":0,"max":"1","patternCodeableConcept":{"coding":[{"code":"vs"}]},"type":[{"code":"CodeableConcept"}]},
 {"id":"Observation.value[x]","path":"Observation.value[x]","min":0,"max":"1","slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"open"},"type":[{"code":"Quantity"},{"code":"string"}]},
 {"id":"Observation.value[x]:valueQuantity","path":"Observation.value[x]","sliceName":"valueQuantity","min":0,"max":"1","type":[{"code":"Quantity"}]},
 {"id":"Observation.value[x]:valueQuantity.unit","path":"Observation.value[x].unit","min":1,"max":"1","type":[{"code":"string"}]},
 {"id":"Observation.component","path":"Observation.component","min":0,"max":"*","slicing":{"discriminator":[{"type":"pattern","path":"code"}],"rules":"open"},"type":[{"code":"BackboneElement"}]},
 {"id":"Observation.component.code","path":"Observation.component.code","min":1,"max":"1","type":[{"code":"CodeableConcept"}]},
 {"id":"Observation.component:sys","path":"Observation.component","sliceName":"sys","min":1,"max":"1","type":[{"code":"BackboneElement"}]},
 {"id":"Observation.component:sys.code","path":"Observation.component.code","min":1,"max":"1","patternCodeableConcept":{"text":"sys"},"type":[{"code":"CodeableConcept"}]},
 {"id":"Observation.component:sys.code.coding","path":"Observation.component.code.coding","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"code"}],"rules":"open"},"type":[{"code":"Coding"}]},
 {"id":"Observation.component:sys.code.coding:S","path":"Observation.component.code.coding","sliceName":"S","min":1,"max":"1","type":[{"code":"Coding"}]},
 {"id":"Observation.component:sys.code.coding:S.code","path":"Observation.component.code.coding.code","min":1,"max":"1","fixedCode":"S","type":[{"code":"code"}]},
 {"id":"Observation.component:dia","path":"Observation.component","sliceName":"dia","min":1,"max":"1","type":[{"code":"BackboneElement"}]},
 {"id":"Observation.component:dia.code","path":"Observation.component.code","min":1,"max":"1","patternCodeableConcept":{"text":"dia"},"type":[{"code":"CodeableConcept"}]},
 {"id":"Observation.component:dia.code.coding","path":"Observation.component.code.coding","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"code"}],"rules":"open"},"type":[{"code":"Coding"}]},
 {"id":"Observation.component:dia.code.coding:D","path":"Observation.component.code.coding","sliceName":"D","min":1,"max":"1","type":[{"code":"Coding"}]},
 {"id":"Observation.component:dia.code.coding:D.code","path":"Observation.component.code.coding.code","min":1,"max":"1","fixedCode":"D","type":[{"code":"code"}]}
]}}`

func treeSetup(t *testing.T) (*Validator, *registry.StructureDefinition) {
	t.Helper()
	l := loader.NewLoader("")
	p, err := l.LoadFromResources([][]byte{[]byte(obsProfile)})
	if err != nil {
		t.Fatal(err)
	}
	reg := testfhir.Registry(t, "4.0.1")
	if err := reg.LoadFromPackages([]*loader.Package{p}); err != nil {
		t.Fatal(err)
	}
	return New(reg), reg.GetByURL("https://example.org/fhir/StructureDefinition/obs")
}

func slicingErrors(t *testing.T, v *Validator, sd *registry.StructureDefinition, resource string) []string {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(resource), &data); err != nil {
		t.Fatal(err)
	}
	result := issue.NewResult()
	v.ValidateData(data, sd, result)
	var out []string
	for _, is := range result.Issues {
		if is.Severity == issue.SeverityError {
			out = append(out, is.MessageID+" @ "+strings.Join(is.Expression, ","))
		}
	}
	slices.Sort(out)
	return out
}

func TestSlicingPerParentInstance(t *testing.T) {
	v, sd := treeSetup(t)
	const both = `"component":[{"code":{"text":"sys","coding":[{"code":"S"}]}},{"code":{"text":"dia","coding":[{"code":"D"}]}}]`
	for _, tt := range []struct {
		name, resource string
		want           []string
	}{
		// Each component's coding slices count that component's codings only (plan A, D5).
		{"each component has its own required coding", `{"resourceType":"Observation",` + both + `}`, nil},
		{"a component misses its required coding, reported at that component",
			`{"resourceType":"Observation","component":[{"code":{"text":"sys","coding":[{"code":"X"}]}},{"code":{"text":"dia","coding":[{"code":"D"}]}}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Observation.component[0].code.coding:S"}},
		{"a sliced element absent under a present parent still needs its required slices",
			`{"resourceType":"Observation","component":[{"code":{"text":"sys"}},{"code":{"text":"dia","coding":[{"code":"D"}]}}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Observation.component[0].code.coding:S"}},
		{"a required component slice is missing",
			`{"resourceType":"Observation","component":[{"code":{"text":"dia","coding":[{"code":"D"}]}}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Observation.component:sys"}},
		// D-4: an element of an earlier slice after one of a later slice.
		{"ordered, in order", `{"resourceType":"Observation",` + both + `,"identifier":[{"system":"urn:first"},{"system":"urn:second"}]}`, nil},
		{"ordered, out of order", `{"resourceType":"Observation",` + both + `,"identifier":[{"system":"urn:second"},{"system":"urn:first"}]}`,
			[]string{"SLICING_ORDER @ Observation.identifier[1]"}},
		// D-5: content in no slice before a slice.
		{"openAtEnd, other content at the end", `{"resourceType":"Observation",` + both + `,"category":[{"coding":[{"code":"vs"}]},{"text":"other"}]}`, nil},
		{"openAtEnd, other content first", `{"resourceType":"Observation",` + both + `,"category":[{"text":"other"},{"coding":[{"code":"vs"}]}]}`,
			[]string{"SLICING_OPEN_AT_END @ Observation.category[0]"}},
		// A type slice of a choice element is decided by the JSON key, and governs its children.
		{"choice type slice, child missing", `{"resourceType":"Observation",` + both + `,"valueQuantity":{"value":1}}`,
			[]string{"SLICING_CARDINALITY_MIN @ Observation.valueQuantity.unit"}},
		{"choice of another type", `{"resourceType":"Observation",` + both + `,"valueString":"x"}`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := slicingErrors(t, v, sd, tt.resource); !slices.Equal(got, tt.want) {
				t.Errorf("errors %v, want %v", got, tt.want)
			}
		})
	}
}
