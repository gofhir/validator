package validator

import (
	"context"
	"encoding/json"
	"strings"
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
// the SHALL above and is an error. A `urn:` url used to be waved through the
// format check and land on the warning path, which contradicted the one sentence
// in the specification that names URNs explicitly.
func TestExtensionURLMustBeAURLNotAURN(t *testing.T) {
	v := getSharedValidator(t)
	ctx := context.Background()

	rejected := []string{
		"urn:uuid:3f2504e0-4f89-11d3-9a0c-0305e82c3301", // the spec's own example of what to exclude
		"urn:oid:1.2.3.4.5",
		"relative-not-absolute",
		"ex:createdAt", // a valid RFC 3986 URI, but not a URL
	}
	for _, url := range rejected {
		t.Run("rejected/"+url, func(t *testing.T) {
			result, err := v.Validate(ctx, json.RawMessage(patientWithExtensionURL(url)))
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			got := issueFor(t, result, issue.DiagExtensionInvalidURL)
			if got == nil {
				t.Fatalf("%q was accepted as an Extension.url; §2.5.0.1 requires an absolute URL, not a URN", url)
			}
			if got.Severity != issue.SeverityError {
				t.Errorf("%q: severity = %v, want error (it transgresses a SHALL)", url, got.Severity)
			}
		})
	}

	// An absolute URL is well-formed even when nothing defines it. That case is
	// the documented divergence from the HL7 validator — unresolvable is a
	// warning here, not an error — so it must not produce the url diagnostic and
	// must not be raised to an error by this change.
	t.Run("accepted/absolute URL with no definition", func(t *testing.T) {
		result, err := v.Validate(ctx, json.RawMessage(
			patientWithExtensionURL("http://miclinica.cl/fhir/StructureDefinition/nota")))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if got := issueFor(t, result, issue.DiagExtensionInvalidURL); got != nil {
			t.Errorf("an absolute URL was reported as malformed: %s", got.Diagnostics)
		}
		for i := range result.Issues {
			if result.Issues[i].Severity == issue.SeverityError &&
				strings.Contains(result.Issues[i].Diagnostics, "xtension") {
				t.Errorf("an unresolvable extension must stay a warning, got error: %s", result.Issues[i].Diagnostics)
			}
		}
	})
}
