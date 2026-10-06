package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// A complex extension's parts: a relative url must be one its definition declares (an error
// otherwise, as the HL7 validator reports Extension_EXT_SubExtension_Invalid); an absolute url
// names an extension defined separately, validated against its own definition.
func TestSubExtensions(t *testing.T) {
	v := getSharedValidator(t)
	nationality := func(part string) []byte {
		return []byte(`{"resourceType":"Patient","extension":[{"url":"http://hl7.org/fhir/StructureDefinition/patient-nationality",` +
			`"extension":[` + part + `]}],"gender":"female"}`)
	}
	const at = "Patient.extension[0].extension[0]"
	for _, tt := range []struct {
		name, part string
		want       issue.DiagnosticID // at the part, or "" for none
	}{
		{"a declared part", `{"url":"code","valueCodeableConcept":{"coding":[{"system":"urn:iso:std:iso:3166","code":"CL"}]}}`, ""},
		{"a relative url the definition does not declare", `{"url":"nope","valueString":"x"}`, issue.DiagExtensionSubExtensionInvalid},
		{"an absolute url defined separately", `{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"unknown"}`, ""},
		{"an absolute url, validated against its own definition", `{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueString":"x"}`, issue.DiagExtensionInvalidValueType},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), nationality(tt.part))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				for _, e := range is.Expression {
					if len(e) >= len(at) && e[:len(at)] == at && is.Severity != issue.SeverityInformation {
						got = append(got, is.MessageID)
					}
				}
			}
			switch {
			case tt.want == "" && len(got) != 0:
				t.Errorf("issues at the part: %v, want none", got)
			case tt.want != "" && (len(got) != 1 || got[0] != string(tt.want)):
				t.Errorf("issues at the part: %v, want %s", got, tt.want)
			}
			if tt.want == issue.DiagExtensionSubExtensionInvalid {
				if is := issueFor(t, res, tt.want); is == nil || is.Severity != issue.SeverityError {
					t.Errorf("severity: %v, want error", is)
				}
			}
		})
	}
}

// A part with an absolute url whose definition's context is the extension that holds it (context of
// type extension) is allowed there, and only there.
func TestSubExtensionInItsExtensionContext(t *testing.T) {
	const nationality = "http://hl7.org/fhir/StructureDefinition/patient-nationality"
	const url = "https://example.org/fhir/StructureDefinition/in-nationality"
	sd := `{"resourceType":"StructureDefinition","url":"` + url + `","name":"InNationality","status":"draft",` +
		`"fhirVersion":"4.0.1","kind":"complex-type","abstract":false,"type":"Extension",` +
		`"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint",` +
		`"context":[{"type":"extension","expression":"` + nationality + `|4.0.1"}],` +
		`"snapshot":{"element":[` +
		`{"id":"Extension","path":"Extension","min":0,"max":"*"},` +
		`{"id":"Extension.url","path":"Extension.url","min":1,"max":"1","type":[{"code":"uri"}],"fixedUri":"` + url + `"},` +
		`{"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{[]byte(sd)}))
	if err != nil {
		t.Fatal(err)
	}
	part := `{"url":"` + url + `","valueString":"x"}`
	for _, tt := range []struct {
		name, resource, at string
		want               bool // the context is reported invalid
	}{
		{"inside the extension its context names",
			`{"resourceType":"Patient","extension":[{"url":"` + nationality + `","extension":[` + part + `]}]}`,
			"Patient.extension[0].extension[0]", false},
		{"on the resource",
			`{"resourceType":"Patient","extension":[` + part + `]}`,
			"Patient", true}, // where the HL7 validator reports it: the element that holds the extension
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			got := issueFor(t, res, issue.DiagExtensionInvalidContext)
			if (got != nil) != tt.want {
				t.Errorf("context reported invalid: %v, want %v", got, tt.want)
			}
			if got != nil && (len(got.Expression) == 0 || got.Expression[0] != tt.at) {
				t.Errorf("at %v, want %s", got.Expression, tt.at)
			}
		})
	}
}

// A part its definition declares is not reported as undeclared when the snapshot has no value[x]
// element for it.
func TestDeclaredPartWithoutValueElement(t *testing.T) {
	const parent = "https://example.org/fhir/StructureDefinition/no-value-elements"
	sd := `{"resourceType":"StructureDefinition","url":"` + parent + `","name":"NoValueElements","status":"draft",` +
		`"fhirVersion":"4.0.1","kind":"complex-type","abstract":false,"type":"Extension",` +
		`"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint",` +
		`"context":[{"type":"element","expression":"Element"}],` +
		`"snapshot":{"element":[` +
		`{"id":"Extension","path":"Extension","min":0,"max":"*"},` +
		`{"id":"Extension.extension","path":"Extension.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"min":0,"max":"*"},` +
		`{"id":"Extension.extension:a","path":"Extension.extension","sliceName":"a","min":0,"max":"1"},` +
		`{"id":"Extension.extension:a.url","path":"Extension.extension.url","min":1,"max":"1","type":[{"code":"uri"}],"fixedUri":"a"},` +
		`{"id":"Extension.url","path":"Extension.url","min":1,"max":"1","type":[{"code":"uri"}],"fixedUri":"` + parent + `"},` +
		`{"id":"Extension.value[x]","path":"Extension.value[x]","min":0,"max":"0"}]}}`
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{[]byte(sd)}))
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Patient","extension":[{"url":"`+parent+`","extension":[{"url":"a","valueString":"x"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := issueFor(t, res, issue.DiagExtensionSubExtensionInvalid); got != nil {
		t.Errorf("a declared part reported undeclared: %s", got.Diagnostics)
	}
}
