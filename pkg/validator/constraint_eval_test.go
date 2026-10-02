package validator

import (
	"context"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
)

const evalProfileURL = "https://example.org/fhir/StructureDefinition/eval-patient"

// evalProfile carries two invariants that cannot be evaluated on a Patient with two names:
// startsWith() takes one string, and name.family gives two.
var evalProfile = []byte(`{
	"resourceType": "StructureDefinition",
	"url": "` + evalProfileURL + `",
	"name": "EvalPatient",
	"status": "active",
	"type": "Patient",
	"kind": "resource",
	"abstract": false,
	"derivation": "constraint",
	"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
	"differential": {
		"element": [
			{"id": "Patient", "path": "Patient", "constraint": [
				{"key": "eval-err", "severity": "error", "human": "family starts with A",
				 "expression": "name.family.startsWith('A')", "source": "` + evalProfileURL + `"},
				{"key": "eval-warn", "severity": "warning", "human": "family starts with Al",
				 "expression": "name.family.startsWith('Al')", "source": "` + evalProfileURL + `"}
			]}
		]
	}
}`)

// An invariant that cannot be evaluated is not satisfied: it is reported as failed, at its own
// severity, as the HL7 validator does (InstanceValidator.checkInvariant treats an exception from
// the FHIRPath engine as a failed invariant).
func TestConstraintThatCannotBeEvaluatedFails(t *testing.T) {
	v := profileValidator(t)
	patient := func(names string) []byte {
		return []byte(`{"resourceType":"Patient","meta":{"profile":["` + evalProfileURL + `"]},` +
			`"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},` +
			`"name":[` + names + `]}`)
	}
	for _, tt := range []struct {
		name, names string
		want        []string // "<severity> <key>", constraint issues of the two invariants
		unevaluated bool     // the failures say the invariant could not be evaluated
	}{
		{"both hold", `{"family":"Alba"}`, nil, false},
		{"both evaluate and fail", `{"family":"Zapata"}`, []string{"error eval-err", "warning eval-warn"}, false},
		{"neither can be evaluated", `{"family":"Alba"},{"family":"Zapata"}`, []string{"error eval-err", "warning eval-warn"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), patient(tt.names))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.MessageID == string(issue.DiagConstraintEvalError) {
					t.Errorf("%s reported: %s", is.MessageID, is.Diagnostics)
				}
				if is.MessageID != string(issue.DiagConstraintFailed) || !strings.Contains(is.Diagnostics, "eval-") {
					continue
				}
				key := "eval-err"
				if strings.Contains(is.Diagnostics, "eval-warn") {
					key = "eval-warn"
				}
				got = append(got, string(is.Severity)+" "+key)
				if strings.Contains(is.Diagnostics, "could not be evaluated") != tt.unevaluated {
					t.Errorf("%s: says it could not be evaluated = %v, want %v", is.Diagnostics, !tt.unevaluated, tt.unevaluated)
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("constraint issues %v, want %v", got, tt.want)
			}
		})
	}
}
