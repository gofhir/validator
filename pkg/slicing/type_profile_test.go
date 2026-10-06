package slicing

import (
	"slices"
	"testing"

	"github.com/gofhir/validator/v2/internal/testfhir"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// idProfile is an Identifier profile that slices its own extension, closed, by url: a datatype
// profile with slicing of its own.
const idProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/id-sliced","name":"IdSliced",
"type":"Identifier","kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Identifier",
"snapshot":{"element":[
 {"id":"Identifier","path":"Identifier","min":0,"max":"*"},
 {"id":"Identifier.extension","path":"Identifier.extension","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"closed"},"type":[{"code":"Extension"}]},
 {"id":"Identifier.extension:only","path":"Identifier.extension","sliceName":"only","min":0,"max":"1","type":[{"code":"Extension"}]},
 {"id":"Identifier.extension:only.url","path":"Identifier.extension.url","min":1,"max":"1","fixedUri":"https://example.org/only","type":[{"code":"uri"}]},
 {"id":"Identifier.system","path":"Identifier.system","min":0,"max":"1","type":[{"code":"uri"}]},
 {"id":"Identifier.value","path":"Identifier.value","min":0,"max":"1","type":[{"code":"string"}]}
]}}`

// patProfile declares that profile on Patient.identifier, an element that is not a slice and whose
// snapshot does not unroll its children.
const patProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/pat-typed","name":"PatTyped",
"type":"Patient","kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient",
"snapshot":{"element":[
 {"id":"Patient","path":"Patient","min":0,"max":"*"},
 {"id":"Patient.identifier","path":"Patient.identifier","min":0,"max":"*","type":[{"code":"Identifier","profile":["https://example.org/fhir/StructureDefinition/id-sliced"]}]}
]}}`

// The slicing a datatype profile defines applies to a value of an element whose type declares that
// profile (plan B, L1), as the HL7 validator applies it.
func TestSlicingOfATypeProfile(t *testing.T) {
	l := loader.NewLoader("")
	p, err := l.LoadFromResources([][]byte{[]byte(idProfile), []byte(patProfile)})
	if err != nil {
		t.Fatal(err)
	}
	reg := testfhir.Registry(t, "4.0.1")
	if err := reg.LoadFromPackages([]*loader.Package{p}); err != nil {
		t.Fatal(err)
	}
	v := New(reg)
	sd := reg.GetByURL("https://example.org/fhir/StructureDefinition/pat-typed")
	for _, tt := range []struct {
		name, identifier string
		want             []string
	}{
		{"the slice's extension", `{"extension":[{"url":"https://example.org/only","valueString":"x"}],"value":"1"}`, nil},
		{"an extension the closed slicing does not allow", `{"extension":[{"url":"https://example.org/other","valueString":"x"}],"value":"1"}`,
			[]string{"SLICING_NO_MATCH @ Patient.identifier[0].extension[0]"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := slicingErrors(t, v, sd, `{"resourceType":"Patient","identifier":[`+tt.identifier+`]}`)
			if !slices.Equal(got, tt.want) {
				t.Errorf("slicing errors %v, want %v", got, tt.want)
			}
		})
	}
}
