package validator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
)

// patientWithExtensionURL wraps one extension url in an otherwise valid Patient.
func patientWithExtensionURL(url string) []byte {
	return []byte(`{
      "resourceType": "Patient",
      "extension": [{"url": "` + url + `", "valueString": "x"}],
      "gender": "female"
    }`)
}

// TestExtensionURLMustBeAURLNotAURN pins FHIR R4 §2.5.0.1: "The url SHALL be a
// URL, not a URN (e.g. not an OID or a UUID), and it SHALL be the canonical URL
// of a StructureDefinition that defines the extension."
//
// The distinction carries weight here because the two ways an Extension.url can
// be wrong have different severities on purpose (docs/VALIDATION-GAPS.md): an
// extension whose definition cannot be resolved only transgresses a SHOULD and
// is reported as a warning, while a url that is not a URL at all transgresses
// the SHALL above and is an error. A url that slips through the format check
// lands on the warning path, which is the severity the specification does not
// grant it.
func TestExtensionURLMustBeAURLNotAURN(t *testing.T) {
	v := getSharedValidator(t)
	ctx := context.Background()

	rejected := map[string]string{
		"urn:uuid:3f2504e0-4f89-11d3-9a0c-0305e82c3301": "the spec's own example of what to exclude",
		"urn:oid:1.2.3.4.5":                             "an OID, named in the same sentence",
		"URN:UUID:3f2504e0-4f89-11d3-9a0c-0305e82c3301": "the scheme is case-insensitive (RFC 3986 §3.1)",
		"urn:uuid://3f2504e0":                           "still a URN even though it contains '://'",
		"relative-not-absolute":                         "a relative reference has no scheme",
		"ex:createdAt":                                  "a valid RFC 3986 URI, but opaque, so not a URL",
		"://createdAt":                                  "'//' without a scheme is not absolute",
		"1http://example.org/x":                         "a scheme cannot start with a digit",
	}
	for url, why := range rejected {
		t.Run("rejected/"+url, func(t *testing.T) {
			result, err := v.Validate(ctx, json.RawMessage(patientWithExtensionURL(url)))
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			got := issueFor(t, result, issue.DiagExtensionInvalidURL)
			if got == nil {
				t.Fatalf("%q was accepted as an Extension.url (%s)", url, why)
			}
			if got.Severity != issue.SeverityError {
				t.Errorf("%q: severity = %v, want error (it transgresses a SHALL)", url, got.Severity)
			}
		})
	}

	// Accepted as *well-formed*. Whether anything defines them is a separate
	// question these fixtures deliberately do not answer: an absolute URL with
	// no definition stays a warning, which is the documented divergence from the
	// HL7 validator.
	accepted := map[string]string{
		"http://miclinica.cl/fhir/StructureDefinition/nota":  "the ordinary case",
		"https://miclinica.cl/fhir/StructureDefinition/nota": "https too",
		"ex://createdAt": "the scheme is not restricted to http(s): a URL of an unregistered scheme is still a URL",
	}
	for url, why := range accepted {
		t.Run("accepted/"+url, func(t *testing.T) {
			result, err := v.Validate(ctx, json.RawMessage(patientWithExtensionURL(url)))
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if got := issueFor(t, result, issue.DiagExtensionInvalidURL); got != nil {
				t.Errorf("%q was reported as malformed (%s): %s", url, why, got.Diagnostics)
			}

			// None of these resolve to a definition, so each must produce the
			// unknown-extension diagnostic AS A WARNING. Asserting it is present
			// — not merely that no error appeared — is what keeps this from
			// passing when nothing validates the extension at all, and it pins
			// the severity that the documented divergence rests on.
			got := issueFor(t, result, issue.DiagExtensionUnknown)
			if got == nil {
				t.Fatalf("%q produced no unknown-extension diagnostic, so nothing exercised the extension path", url)
			}
			if got.Severity != issue.SeverityWarning {
				t.Errorf("%q: unresolvable extension reported as %v, want warning (docs/VALIDATION-GAPS.md)",
					url, got.Severity)
			}
		})
	}
}

// TestComplexExtensionChildrenKeepBareNames guards the exception the URL rule
// leans on: "Except for child extensions defined within complex extensions, the
// URL SHALL be an absolute URL" (§2.5.0.1). Children are resolved by name
// against the parent's definition and must never be measured against the
// absolute-URL rule.
//
// Without this test, a refactor routing children through the same path as
// top-level extensions would reject every complex extension in existence and no
// test would fail — the exception lives only in a comment and in two doc pages.
func TestComplexExtensionChildrenKeepBareNames(t *testing.T) {
	v := getSharedValidator(t)

	// patient-nationality is a complex extension from the base packages: its
	// children are named `code` and `period`, with no scheme and no host.
	patient := []byte(`{
      "resourceType": "Patient",
      "extension": [{
        "url": "http://hl7.org/fhir/StructureDefinition/patient-nationality",
        "extension": [
          {"url": "code", "valueCodeableConcept": {"coding": [
            {"system": "urn:iso:std:iso:3166", "code": "CL"}]}},
          {"url": "period", "valuePeriod": {"start": "2020-01-01"}}
        ]
      }],
      "gender": "female"
    }`)

	// The control comes first and is the only assertion here that can fail when
	// children are not walked at all. Everything else in this test is an
	// absence, and absences are exactly what a skipped code path produces: with
	// the call to validateNestedExtensions removed, a test built only on them
	// stays green while the behavior it guards is gone.
	//
	// A child named after nothing in the parent's definition has to be reported.
	// If that diagnostic is missing, the children were never inspected.
	unknownChild := []byte(`{
      "resourceType": "Patient",
      "extension": [{
        "url": "http://hl7.org/fhir/StructureDefinition/patient-nationality",
        "extension": [{"url": "no-such-child", "valueString": "x"}]
      }],
      "gender": "female"
    }`)
	control, err := v.Validate(context.Background(), json.RawMessage(unknownChild))
	if err != nil {
		t.Fatalf("Validate (control): %v", err)
	}
	if got := issueFor(t, control, issue.DiagExtensionNestedUnknown); got == nil {
		t.Fatal("an unknown child produced no nested-extension diagnostic, so children are not being walked; " +
			"the rest of this test would pass vacuously")
	}

	result, err := v.Validate(context.Background(), json.RawMessage(patient))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if got := issueFor(t, result, issue.DiagExtensionUnknown); got != nil {
		t.Fatalf("the parent extension did not resolve, so the children were never walked: %s", got.Diagnostics)
	}
	if got := issueFor(t, result, issue.DiagExtensionNestedUnknown); got != nil {
		t.Fatalf("a conformant child was not recognized against the parent's definition: %s", got.Diagnostics)
	}
	if got := issueFor(t, result, issue.DiagExtensionInvalidURL); got != nil {
		t.Fatalf("a complex extension's child was measured against the absolute-URL rule: %s", got.Diagnostics)
	}
	for i := range result.Issues {
		if result.Issues[i].Severity == issue.SeverityError {
			t.Errorf("unexpected error on a conformant complex extension: %s", result.Issues[i].Diagnostics)
		}
	}
}
