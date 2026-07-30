package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/terminology"
)

// A Coding whose display is the valid Spanish translation of the concept.
const spanishDisplayResource = `{
  "resourceType": "Encounter",
  "status": "finished",
  "class": {
    "system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
    "code": "AMB",
    "display": "ambulatorio"
  }
}`

func encounterWithDisplay(display string) []byte {
	return []byte(`{
	  "resourceType": "Encounter",
	  "status": "finished",
	  "class": {
	    "system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
	    "code": "AMB",
	    "display": "` + display + `"
	  }
	}`)
}

// TestDisplayValidationIsLanguageAware covers the case that motivated the work: a
// display in another language used to be compared against the English one and
// rejected, which is a validator bug rather than a data problem.
//
// Subtests share one validator built with WithDisplayLanguage("es"), because each
// New() reloads the full package set.
func TestDisplayValidationIsLanguageAware(t *testing.T) {
	v, auth := spanishValidator(t)
	ctx := context.Background()

	t.Run("valid translation is accepted", func(t *testing.T) {
		auth.set(terminology.Valid, terminology.MembershipIncluded)
		auth.setDisplay("ambulatorio") // honored: the backend has the Spanish designation

		result, err := v.Validate(ctx, []byte(spanishDisplayResource))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if got := issueFor(t, result, issue.DiagBindingDisplayMismatch); got != nil {
			t.Errorf("a valid Spanish display must not be a mismatch: %s", got.Diagnostics)
		}
	})

	t.Run("skipped when the language cannot be honored", func(t *testing.T) {
		// This is the decoupling designed into the contract: a host can ship display
		// validation before its store carries designations without a single false
		// rejection, because an unhonored language skips the comparison instead of
		// checking a translation against English.
		auth.set(terminology.Valid, terminology.MembershipIncluded)
		auth.setUnhonoredDisplay("ambulatory")

		result, err := v.Validate(ctx, []byte(spanishDisplayResource))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if got := issueFor(t, result, issue.DiagBindingDisplayMismatch); got != nil {
			t.Errorf("with the language unhonored there is nothing to compare against: %s",
				got.Diagnostics)
		}
	})

	t.Run("a wrong display is still an error", func(t *testing.T) {
		// The i18n handling must not become a blanket excuse to skip the check.
		// Error rather than warning, per HL7's guidance: a wrong display often means
		// the wrong code was chosen.
		auth.set(terminology.Valid, terminology.MembershipIncluded)
		auth.setDisplay("ambulatorio")

		result, err := v.Validate(ctx, encounterWithDisplay("hospitalizado"))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		got := issueFor(t, result, issue.DiagBindingDisplayMismatch)
		if got == nil {
			t.Fatalf("a genuinely wrong display must still be reported; issues: %v",
				result.Issues)
		}
		if got.Severity != issue.SeverityError {
			t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityError)
		}
	})
}

// TestDisplayComparisonIsCaseInsensitive uses the fixture with no language
// preference, where the concept's default display is authoritative.
func TestDisplayComparisonIsCaseInsensitive(t *testing.T) {
	v, auth := warnValidator(t)
	auth.set(terminology.Valid, terminology.MembershipIncluded)
	auth.setDisplay("ambulatory")

	result, err := v.Validate(context.Background(), encounterWithDisplay("Ambulatory"))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := issueFor(t, result, issue.DiagBindingDisplayMismatch); got != nil {
		t.Errorf("display comparison is case-insensitive, per HL7: %s", got.Diagnostics)
	}
}
