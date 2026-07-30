package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/terminology"
)

// localizingAuthority answers with a display in the requested language when it has
// one, mirroring a host whose store carries designations.
type localizingAuthority struct {
	displays map[string]string // BCP-47 tag -> display; "" is the default
}

func (a *localizingAuthority) ResolveCodeInValueSet(_ context.Context, _, _, _ string, opts terminology.LookupOptions) (terminology.CodeResult, error) {
	display, honored := a.pick(opts.DisplayLanguage)
	return terminology.CodeResult{
		Resolution:             terminology.Valid,
		Display:                display,
		DisplayLanguageHonored: honored,
		SystemInValueSet:       terminology.MembershipIncluded,
	}, nil
}

func (a *localizingAuthority) ResolveCodeInCodeSystem(_ context.Context, _, _ string, opts terminology.LookupOptions) (terminology.CodeResult, error) {
	display, honored := a.pick(opts.DisplayLanguage)
	return terminology.CodeResult{
		Resolution:             terminology.Valid,
		Display:                display,
		DisplayLanguageHonored: honored,
	}, nil
}

func (a *localizingAuthority) pick(lang string) (display string, honored bool) {
	if lang == "" {
		return a.displays[""], true
	}
	if d, ok := a.displays[lang]; ok {
		return d, true
	}
	// No designation in that language: return the default and say so.
	return a.displays[""], false
}

func (a *localizingAuthority) Supports(context.Context, string) bool { return true }

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

// TestSpanishDisplayIsNotRejectedWhenLanguageIsHonored is the case that motivated
// this work: a valid non-English display must not be reported as a mismatch.
func TestSpanishDisplayIsNotRejectedWhenLanguageIsHonored(t *testing.T) {
	auth := &localizingAuthority{displays: map[string]string{
		"":   "ambulatory",
		"es": "ambulatorio",
	}}
	v, err := New(
		WithVersion("4.0.1"),
		WithTerminologyAuthority(auth),
		WithDisplayLanguage("es"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), []byte(spanishDisplayResource))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if got := issueFor(t, result, issue.DiagBindingDisplayMismatch); got != nil {
		t.Errorf("a valid Spanish display must not be a mismatch: %s", got.Diagnostics)
	}
}

// TestDisplayCheckIsSkippedWhenLanguageCannotBeHonored covers the decoupling that
// lets this ship before a host carries designations: rather than compare a
// submitted translation against English, the check is skipped.
func TestDisplayCheckIsSkippedWhenLanguageCannotBeHonored(t *testing.T) {
	// The authority has only the default display — no Spanish designation yet.
	auth := &localizingAuthority{displays: map[string]string{"": "ambulatory"}}
	v, err := New(
		WithVersion("4.0.1"),
		WithTerminologyAuthority(auth),
		WithDisplayLanguage("es"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), []byte(spanishDisplayResource))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if got := issueFor(t, result, issue.DiagBindingDisplayMismatch); got != nil {
		t.Errorf("with the language unhonored there is nothing to compare against, "+
			"so no mismatch should be reported: %s", got.Diagnostics)
	}
}

// TestWrongDisplayIsStillAnError guards the other direction: the i18n handling
// must not become a blanket excuse to skip display validation.
func TestWrongDisplayIsStillAnError(t *testing.T) {
	auth := &localizingAuthority{displays: map[string]string{
		"":   "ambulatory",
		"es": "ambulatorio",
	}}
	v, err := New(
		WithVersion("4.0.1"),
		WithTerminologyAuthority(auth),
		WithDisplayLanguage("es"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	wrong := []byte(`{
	  "resourceType": "Encounter",
	  "status": "finished",
	  "class": {
	    "system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
	    "code": "AMB",
	    "display": "hospitalizado"
	  }
	}`)

	result, err := v.Validate(context.Background(), wrong)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagBindingDisplayMismatch)
	if got == nil {
		t.Fatalf("a genuinely wrong display must still be reported; issues: %v", result.Issues)
	}
	// Error, not warning: per HL7's guidance a wrong display often means the wrong
	// code was chosen.
	if got.Severity != issue.SeverityError {
		t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityError)
	}
}

// TestDisplayValidationWithoutLanguagePreference checks the default path, where no
// language is requested and the concept's default display is authoritative.
func TestDisplayValidationWithoutLanguagePreference(t *testing.T) {
	auth := &localizingAuthority{displays: map[string]string{"": "ambulatory"}}
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(auth))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	matching := []byte(`{
	  "resourceType": "Encounter",
	  "status": "finished",
	  "class": {
	    "system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
	    "code": "AMB",
	    "display": "Ambulatory"
	  }
	}`)

	result, err := v.Validate(context.Background(), matching)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Case-insensitive, per HL7.
	if got := issueFor(t, result, issue.DiagBindingDisplayMismatch); got != nil {
		t.Errorf("display comparison is case-insensitive: %s", got.Diagnostics)
	}
}
