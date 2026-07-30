package validator

import (
	"context"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
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

// TestCodingWithSystemButNoCodeIsError is the more severe half. Naming a vocabulary and
// then saying nothing in it is not a judgement call the reader can recover from, unlike a
// missing system where text may still carry the meaning.
func TestCodingWithSystemButNoCodeIsError(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{
	  "resourceType": "Observation",
	  "status": "final",
	  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
	  "code": {"coding": [{"system": "http://loinc.org", "display": "d"}]}
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagCodingNoCode)
	if got == nil {
		t.Fatalf("a Coding naming a system with no code must be reported; issues: %v",
			result.Issues)
	}
	if got.Severity != issue.SeverityError {
		t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityError)
	}
	// The system belongs in the message — it says which vocabulary was left unanswered.
	if !strings.Contains(got.Diagnostics, "http://loinc.org") {
		t.Errorf("diagnostics = %q, want the system named", got.Diagnostics)
	}
	// LOINC is external and unresolvable here, but that is a separate matter: an absent
	// code is decidable without consulting any terminology source.
	if issueFor(t, result, issue.DiagBindingCannotValidate) != nil {
		t.Error("an absent code needs no terminology lookup, so it must not be reported " +
			"as merely unvalidatable")
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

	for _, id := range []issue.DiagnosticID{issue.DiagCodingNoSystem, issue.DiagCodingNoCode} {
		if issueFor(t, result, id) != nil {
			t.Errorf("%s fired on a CodeableConcept with no coding; text alone is valid", id)
		}
	}
}
