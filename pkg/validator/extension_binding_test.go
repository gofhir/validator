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
