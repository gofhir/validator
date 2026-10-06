package validator

import (
	"context"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

const bornProfileURL = "https://example.org/fhir/StructureDefinition/born-patient"

// bornProfile constrains a primitive element, Patient.birthDate, on its value.
var bornProfile = []byte(`{
	"resourceType": "StructureDefinition",
	"url": "` + bornProfileURL + `",
	"name": "BornPatient",
	"status": "active",
	"type": "Patient",
	"kind": "resource",
	"abstract": false,
	"derivation": "constraint",
	"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
	"differential": {
		"element": [
			{"id": "Patient", "path": "Patient"},
			{"id": "Patient.birthDate", "path": "Patient.birthDate", "constraint": [
				{"key": "born-1900s", "severity": "error", "human": "born in the 1900s",
				 "expression": "toString().startsWith('19')", "source": "` + bornProfileURL + `"}
			]}
		]
	}
}`)

// A constraint on a primitive element is evaluated with the element's value as its focus.
func TestConstraintOnAPrimitiveSeesItsValue(t *testing.T) {
	v := profileValidator(t)
	for _, tt := range []struct {
		birthDate string
		wantFail  bool
	}{
		{"1990-01-01", false},
		{"2001-01-01", true},
	} {
		t.Run(tt.birthDate, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Patient","meta":{"profile":["`+bornProfileURL+`"]},`+
				`"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},`+
				`"birthDate":"`+tt.birthDate+`"}`))
			if err != nil {
				t.Fatal(err)
			}
			failed := false
			for _, is := range res.Issues {
				if !strings.Contains(is.Diagnostics, "born-1900s") {
					continue
				}
				if is.MessageID != string(issue.DiagConstraintFailed) || is.Severity != issue.SeverityError {
					t.Errorf("%s %s: %s, want a failed constraint", is.Severity, is.MessageID, is.Diagnostics)
					continue
				}
				failed = true
				if got := strings.Join(is.Expression, ","); got != "Patient.birthDate" {
					t.Errorf("location %s, want Patient.birthDate", got)
				}
			}
			if failed != tt.wantFail {
				t.Errorf("born-1900s failed = %v, want %v", failed, tt.wantFail)
			}
		})
	}
}
