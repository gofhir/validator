package validator

import (
	"context"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/terminology"
)

// An extension whose Extension.value[x] is bound required to a ValueSet. The
// binding lives in the extension's own StructureDefinition, which is why this path
// used to carry its own copy of binding validation.
const boundExtensionSD = `{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/StructureDefinition/bound-ext",
  "name": "BoundExt",
  "status": "active",
  "kind": "complex-type",
  "abstract": false,
  "type": "Extension",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Extension",
  "derivation": "constraint",
  "context": [{"type": "element", "expression": "Patient"}],
  "snapshot": {
    "element": [
      {"id": "Extension", "path": "Extension", "min": 0, "max": "1"},
      {
        "id": "Extension.url",
        "path": "Extension.url",
        "min": 1, "max": "1",
        "type": [{"code": "uri"}],
        "fixedUri": "http://example.org/StructureDefinition/bound-ext"
      },
      {
        "id": "Extension.value[x]",
        "path": "Extension.value[x]",
        "min": 1, "max": "1",
        "type": [{"code": "Coding"}],
        "binding": {
          "strength": "required",
          "valueSet": "http://example.org/ValueSet/bound"
        }
      }
    ]
  }
}`

func resourceWithBoundExtension(display string) []byte {
	return []byte(`{
	  "resourceType": "Patient",
	  "extension": [{
	    "url": "http://example.org/StructureDefinition/bound-ext",
	    "valueCoding": {
	      "system": "http://example.org/CodeSystem/cs",
	      "code": "good",
	      "display": "` + display + `"
	    }
	  }]
	}`)
}

// TestExtensionBindingUsesTheElementPath covers what routing extension values
// through the binding validator gained. This path used to carry its own copy of the
// logic and had fallen behind on checks the specification requires.
//
// Subtests share one validator: each New() reloads the full package set, which under
// CI's -race and coverage dominates this package's test budget.
func TestExtensionBindingUsesTheElementPath(t *testing.T) {
	v, auth := authorityValidator(t)
	ctx := context.Background()

	t.Run("checks Coding.display", func(t *testing.T) {
		// The copy in pkg/extension never looked at the display, so a wrong one inside
		// an extension went unreported while the same display on any other element
		// errored.
		auth.set(terminology.Valid, terminology.MembershipIncluded)
		auth.setDisplay("Good")

		result, err := v.Validate(ctx, resourceWithBoundExtension("Totally Wrong"))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if issueFor(t, result, issue.DiagBindingDisplayMismatch) == nil {
			t.Errorf("a wrong display inside an extension must be reported, as it is "+
				"anywhere else; issues: %v", result.Issues)
		}
	})

	t.Run("honors the unresolved policy", func(t *testing.T) {
		// The extension path treated an undecidable binding as nothing at all, so
		// WithUnresolvedPolicy had no effect there. The shared fixture is built with
		// UnresolvedError.
		auth.set(terminology.Unresolved, terminology.MembershipUnknown)

		result, err := v.Validate(ctx, resourceWithBoundExtension("Good"))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		got := issueFor(t, result, issue.DiagBindingUnresolved)
		if got == nil {
			t.Fatalf("an unresolvable binding on an extension value must be reported "+
				"under UnresolvedError; issues: %v", result.Issues)
		}
		if got.Severity != issue.SeverityError {
			t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityError)
		}
	})

	t.Run("reports a required violation", func(t *testing.T) {
		// Behavior that already worked and must survive the unification.
		auth.set(terminology.Invalid, terminology.MembershipIncluded)

		result, err := v.Validate(ctx, resourceWithBoundExtension("Good"))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		got := issueFor(t, result, issue.DiagBindingRequired)
		if got == nil {
			t.Fatalf("a required binding violation on an extension value must error; "+
				"issues: %v", result.Issues)
		}
		if got.Severity != issue.SeverityError {
			t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityError)
		}
	})

	t.Run("surfaces the backend explanation", func(t *testing.T) {
		// CodeResult.Message used to be discarded, dropping the part a user can act on.
		auth.set(terminology.Invalid, terminology.MembershipIncluded)
		auth.setMessage("code was retired in the 2024 edition")

		result, err := v.Validate(ctx, resourceWithBoundExtension("Good"))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		got := issueFor(t, result, issue.DiagBindingRequired)
		if got == nil {
			t.Fatalf("expected a binding violation; issues: %v", result.Issues)
		}
		if !strings.Contains(got.Diagnostics, "retired in the 2024 edition") {
			t.Errorf("the backend's explanation is missing from the diagnostic: %q",
				got.Diagnostics)
		}
	})
}

// codedValueResource carries a Coding on Observation.code, which is bound example —
// weak enough that binding validation skips it entirely.
const codedValueResource = `{
  "resourceType": "Observation",
  "status": "final",
  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
  "code": {
    "coding": [
      {
        "system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
        "code": "THIS-CODE-DOES-NOT-EXIST"
      }
    ]
  }
}`

// TestCodeSystemCheckIsIndependentOfBinding is the gap the reference comparison found.
//
// A Coding naming a code that does not exist in the system it declares is wrong
// regardless of binding strength, but the check used to live inside binding validation
// — which returns early for anything weaker than extensible. Since most clinical codes
// in FHIR are bound example (Observation.code, Condition.code, Procedure.code), we were
// accepting codes absent from CodeSystems we hold.
//
// HL7 validator_cli 6.9.12 reports this as an error on the .code child.
func TestCodeSystemCheckIsIndependentOfBinding(t *testing.T) {
	v := getSharedValidator(t)

	result, err := v.Validate(context.Background(), []byte(codedValueResource))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagCodeNotInCodeSystem)
	if got == nil {
		t.Fatalf("a code absent from its own CodeSystem must be reported even under an "+
			"example binding; issues: %v", result.Issues)
	}
	if got.Severity != issue.SeverityError {
		t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityError)
	}
	// Reported on the code itself, as the reference does: a CodeableConcept with several
	// codings has to say which one is wrong.
	if len(got.Expression) == 0 || !strings.HasSuffix(got.Expression[0], ".code") {
		t.Errorf("expression = %v, want the .code child", got.Expression)
	}
}

// TestExternalSystemIsInformationalNotWarning keeps the check from becoming noise. A
// known external vocabulary is expected to need a terminology server, and the reference
// reports nothing at all because it resolves these against tx.fhir.org.
func TestExternalSystemIsInformationalNotWarning(t *testing.T) {
	v := getSharedValidator(t)

	loinc := []byte(`{
	  "resourceType": "Observation",
	  "status": "final",
	  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
	  "code": {"coding": [{"system": "http://loinc.org", "code": "29463-7"}]}
	}`)

	result, err := v.Validate(context.Background(), loinc)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagBindingCannotValidate)
	if got == nil {
		t.Fatalf("an unresolvable external system should be noted; issues: %v", result.Issues)
	}
	if got.Severity != issue.SeverityInformation {
		t.Errorf("severity = %s, want %s: a vocabulary that needs a server is not a defect",
			got.Severity, issue.SeverityInformation)
	}
	if issueFor(t, result, issue.DiagCodeNotInCodeSystem) != nil {
		t.Error("an unresolvable code must not be reported as invalid")
	}
}
