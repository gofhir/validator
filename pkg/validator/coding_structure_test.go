package validator

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// A Coding is only interpretable as a pair. These tests cover the two ways it can fail to
// be one, and the case that must stay silent.
//
// All three were verified against HL7 validator_cli 6.9.12 before being written here, so
// the expectations are the reference's, not ours.

// TestCodingWithNoSystemIsWarned covers a code that names no vocabulary. It is a warning
// rather than an error because the concept may still be carried by CodeableConcept.text.
func TestCodingWithNoSystemIsWarned(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{
	  "resourceType": "Observation",
	  "status": "final",
	  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
	  "code": {"coding": [{"code": "1234-5", "display": "Some code"}]}
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagCodingNoSystem)
	if got == nil {
		t.Fatalf("a Coding with no system cannot be placed in any vocabulary and must be "+
			"reported; issues: %v", result.Issues)
	}
	if got.Severity != issue.SeverityWarning {
		t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityWarning)
	}
	// On the Coding, not a child: the defect is the absence of one of its two halves.
	if len(got.Expression) == 0 || !strings.HasSuffix(got.Expression[0], ".coding[0]") {
		t.Errorf("expression = %v, want the Coding itself", got.Expression)
	}
}

// TestCodingWithNoSystemIsWarnedWithoutACode checks the condition is the missing system
// alone. A Coding carrying only a display is just as unplaceable, and the reference
// reports the same warning there.
func TestCodingWithNoSystemIsWarnedWithoutACode(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{
	  "resourceType": "Observation",
	  "status": "final",
	  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
	  "code": {"coding": [{"display": "Just a display"}]}
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if issueFor(t, result, issue.DiagCodingNoSystem) == nil {
		t.Errorf("a Coding with neither system nor code must still be reported; issues: %v",
			result.Issues)
	}
}

// A Coding that names a system and no code is allowed (Coding.code 0..1): nothing is reported of
// it, as HL7 6.10.2 reports nothing, but under a required binding, which asks for a code
// (BINDING_REQUIRED_NO_CODE, decided by the specification; HL7 reports nothing for a Coding).
func TestCodingWithSystemButNoCode(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithPackageTgz(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "packages", "acme.unknowncs-0.1.0.tgz")))
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name, resource string
		want           issue.DiagnosticID // "" for none
	}{
		{"no binding requires a code", `{"resourceType": "Observation", "status": "final",
		  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
		  "code": {"coding": [{"system": "http://loinc.org", "display": "d"}]}}`, ""},
		{"a required binding", `{"resourceType": "Location", "id": "l",
		  "meta": {"profile": ["http://acme.local/fhir/StructureDefinition/loc-req"]},
		  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
		  "operationalStatus": {"system": "http://acme.local/cs/known"}}`, issue.DiagBindingRequiredNoCode},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if issueFor(t, result, issue.DiagnosticID("CODING_NO_CODE")) != nil {
				t.Errorf("CODING_NO_CODE reported: %v", result.Issues)
			}
			switch got := issueFor(t, result, tt.want); {
			case tt.want == "":
			case got == nil:
				t.Errorf("want %s; issues: %v", tt.want, result.Issues)
			case got.Severity != issue.SeverityError:
				t.Errorf("severity %s, want error", got.Severity)
			}
		})
	}
}

// TestCodeableConceptWithOnlyTextIsSilent guards the boundary. Text alone is a legitimate
// way to express a concept — FHIR allows it precisely when no code fits — so neither rule
// may fire when there is no coding at all. The reference is silent here too.
func TestCodeableConceptWithOnlyTextIsSilent(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{
	  "resourceType": "Observation",
	  "status": "final",
	  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
	  "code": {"text": "free text only"}
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	for _, id := range []issue.DiagnosticID{issue.DiagCodingNoSystem, issue.DiagnosticID("CODING_NO_CODE")} {
		if issueFor(t, result, id) != nil {
			t.Errorf("%s fired on a CodeableConcept with no coding; text alone is valid", id)
		}
	}
}
