package validator

import (
	"context"
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

// TestExtensionBindingChecksDisplay is the point of routing extension values through
// the binding validator: this path had its own copy of the logic, which never checked
// Coding.display. A wrong display inside an extension went unreported.
func TestExtensionBindingChecksDisplay(t *testing.T) {
	auth := &localizingAuthority{displays: map[string]string{"": "Good"}}
	v, err := New(
		WithVersion("4.0.1"),
		WithConformanceResources([][]byte{[]byte(boundExtensionSD)}),
		WithTerminologyAuthority(auth),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), resourceWithBoundExtension("Totally Wrong"))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if issueFor(t, result, issue.DiagBindingDisplayMismatch) == nil {
		t.Errorf("a wrong display inside an extension must be reported, as it is "+
			"anywhere else; issues: %v", result.Issues)
	}
}

// TestExtensionBindingHonorsUnresolvedPolicy covers the other half: the extension
// path treated an unresolvable binding as nothing at all, so WithUnresolvedPolicy
// had no effect there.
func TestExtensionBindingHonorsUnresolvedPolicy(t *testing.T) {
	v, err := New(
		WithVersion("4.0.1"),
		WithConformanceResources([][]byte{[]byte(boundExtensionSD)}),
		WithTerminologyAuthority(unresolvableAuthority{}),
		WithUnresolvedPolicy(UnresolvedError),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), resourceWithBoundExtension("Good"))
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
}

// TestExtensionBindingHonorsDisplayLanguage confirms the i18n handling reaches
// extensions too: a valid translation must not be reported as a mismatch.
func TestExtensionBindingHonorsDisplayLanguage(t *testing.T) {
	auth := &localizingAuthority{displays: map[string]string{
		"":   "Good",
		"es": "Bueno",
	}}
	v, err := New(
		WithVersion("4.0.1"),
		WithConformanceResources([][]byte{[]byte(boundExtensionSD)}),
		WithTerminologyAuthority(auth),
		WithDisplayLanguage("es"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), resourceWithBoundExtension("Bueno"))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if got := issueFor(t, result, issue.DiagBindingDisplayMismatch); got != nil {
		t.Errorf("a valid Spanish display inside an extension must not be a mismatch: %s",
			got.Diagnostics)
	}
}

// TestExtensionBindingReportsRequiredViolation keeps the behavior that already
// worked: a non-member code under a required binding is still an error.
func TestExtensionBindingReportsRequiredViolation(t *testing.T) {
	v, err := New(
		WithVersion("4.0.1"),
		WithConformanceResources([][]byte{[]byte(boundExtensionSD)}),
		WithTerminologyAuthority(&membershipAuthority{
			resolution: terminology.Invalid,
			membership: terminology.MembershipIncluded,
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), resourceWithBoundExtension("Good"))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagBindingRequired)
	if got == nil {
		t.Fatalf("a required binding violation on an extension value must error; issues: %v",
			result.Issues)
	}
	if got.Severity != issue.SeverityError {
		t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityError)
	}
}
