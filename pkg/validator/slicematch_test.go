package validator

import (
	"context"
	"strings"
	"testing"
)

// Profiles where only conformance tells two slices apart, for a "profile" discriminator on values
// that are not resources: two profiles of one extension that differ in the type of value[x], and
// two Identifier profiles that differ in the fixed system.
const (
	extS = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/e-string","name":"ES","type":"Extension",
"kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","snapshot":{"element":[
 {"id":"Extension","path":"Extension","min":0,"max":"*"},
 {"id":"Extension.url","path":"Extension.url","min":1,"max":"1","fixedUri":"https://example.org/e","type":[{"code":"uri"}]},
 {"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	extQ = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/e-quantity","name":"EQ","type":"Extension",
"kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","snapshot":{"element":[
 {"id":"Extension","path":"Extension","min":0,"max":"*"},
 {"id":"Extension.url","path":"Extension.url","min":1,"max":"1","fixedUri":"https://example.org/e","type":[{"code":"uri"}]},
 {"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"max":"1","type":[{"code":"Quantity"}]}]}}`
	idMRN = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/id-mrn","name":"IdMrn","type":"Identifier",
"kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Identifier","snapshot":{"element":[
 {"id":"Identifier","path":"Identifier","min":0,"max":"*"},
 {"id":"Identifier.system","path":"Identifier.system","min":1,"max":"1","fixedUri":"urn:mrn","type":[{"code":"uri"}]},
 {"id":"Identifier.value","path":"Identifier.value","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	idTax = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/id-tax","name":"IdTax","type":"Identifier",
"kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Identifier","snapshot":{"element":[
 {"id":"Identifier","path":"Identifier","min":0,"max":"*"},
 {"id":"Identifier.system","path":"Identifier.system","min":1,"max":"1","fixedUri":"urn:tax","type":[{"code":"uri"}]},
 {"id":"Identifier.value","path":"Identifier.value","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	byProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/by-profile","name":"ByProfile","type":"Patient",
"kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","snapshot":{"element":[
 {"id":"Patient","path":"Patient","min":0,"max":"*"},
 {"id":"Patient.extension","path":"Patient.extension","min":0,"max":"*","slicing":{"discriminator":[{"type":"profile","path":"$this"}],"rules":"closed"},"type":[{"code":"Extension"}]},
 {"id":"Patient.extension:s","path":"Patient.extension","sliceName":"s","min":0,"max":"1","type":[{"code":"Extension","profile":["https://example.org/fhir/StructureDefinition/e-string"]}]},
 {"id":"Patient.extension:q","path":"Patient.extension","sliceName":"q","min":1,"max":"1","type":[{"code":"Extension","profile":["https://example.org/fhir/StructureDefinition/e-quantity"]}]},
 {"id":"Patient.identifier","path":"Patient.identifier","min":0,"max":"*","slicing":{"discriminator":[{"type":"profile","path":"$this"}],"rules":"closed"},"type":[{"code":"Identifier"}]},
 {"id":"Patient.identifier:mrn","path":"Patient.identifier","sliceName":"mrn","min":1,"max":"1","type":[{"code":"Identifier","profile":["https://example.org/fhir/StructureDefinition/id-mrn"]}]},
 {"id":"Patient.identifier:tax","path":"Patient.identifier","sliceName":"tax","min":0,"max":"1","type":[{"code":"Identifier","profile":["https://example.org/fhir/StructureDefinition/id-tax"]}]}]}}`
)

func TestProfileDiscriminatorOnValuesThatAreNotResources(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{
		[]byte(extS), []byte(extQ), []byte(idMRN), []byte(idTax), []byte(byProfile),
	}))
	if err != nil {
		t.Fatal(err)
	}
	patient := func(ext, ids string) []byte {
		return []byte(`{"resourceType":"Patient","meta":{"profile":["https://example.org/fhir/StructureDefinition/by-profile"]},` +
			`"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},` +
			`"extension":[` + ext + `],"identifier":[` + ids + `]}`)
	}
	const (
		quantity = `{"url":"https://example.org/e","valueQuantity":{"value":1}}`
		str      = `{"url":"https://example.org/e","valueString":"x"}`
		mrn      = `{"system":"urn:mrn","value":"1"}`
		tax      = `{"system":"urn:tax","value":"2"}`
		other    = `{"system":"urn:other","value":"3"}`
	)
	for _, tt := range []struct {
		name, ext, ids string
		want           []string // slicing errors, "<ID> @ <location>"
	}{
		{"each value in the slice it conforms to", str + "," + quantity, mrn + "," + tax, nil},
		{"the required extension slice is missing", str, mrn,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.extension:q"}},
		{"an identifier that conforms to no profile", quantity, mrn + "," + other,
			[]string{"SLICING_NO_MATCH @ Patient.identifier[1]"}},
		{"the required identifier slice is missing", quantity, tax,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.identifier:mrn"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), patient(tt.ext, tt.ids))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "SLICING_") && is.Severity == "error" {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("slicing errors %v, want %v", got, tt.want)
			}
		})
	}
}
