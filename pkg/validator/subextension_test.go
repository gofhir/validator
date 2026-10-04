package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
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
