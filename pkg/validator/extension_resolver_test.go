package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/terminology"
)

// mockExtensionResolver serves a single extension StructureDefinition on demand.
type mockExtensionResolver struct {
	extensionURL string
	sdJSON       []byte
	calls        int
}

func (r *mockExtensionResolver) ResolveProfile(_ context.Context, url, _ string) ([]byte, error) {
	r.calls++
	if url == r.extensionURL {
		return r.sdJSON, nil
	}
	return nil, nil
}

// TestExtensionValidation_UsesProfileResolver verifies that the extension validator
// falls back to the configured ProfileResolver when an extension SD is not in the
// in-memory registry, so extensions stored in external sources (DB, IG packages)
// can be resolved on demand. Reproduces issue #55.
func TestExtensionValidation_UsesProfileResolver(t *testing.T) {
	const extURL = "http://example.org/StructureDefinition/custom-extension"

	extSDJSON := []byte(`{
		"resourceType": "StructureDefinition",
		"url": "` + extURL + `",
		"name": "CustomExtension",
		"status": "active",
		"fhirVersion": "4.0.1",
		"kind": "complex-type",
		"abstract": false,
		"type": "Extension",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Extension",
		"derivation": "constraint",
		"context": [{"type": "element", "expression": "Patient"}],
		"snapshot": {
			"element": [
				{"id": "Extension", "path": "Extension", "min": 0, "max": "*"},
				{"id": "Extension.url", "path": "Extension.url", "min": 1, "max": "1", "fixedUri": "` + extURL + `"},
				{"id": "Extension.value[x]", "path": "Extension.value[x]", "min": 1, "max": "1", "type": [{"code": "string"}]}
			]
		}
	}`)

	resolver := &mockExtensionResolver{
		extensionURL: extURL,
		sdJSON:       extSDJSON,
	}

	v, err := New(
		WithProfileResolver(resolver),
		// Terminology is not under test here; an authority skips parsing the base
		// ValueSets/CodeSystems, the dominant cost of building a validator under
		// -race and coverage.
		WithTerminologyAuthority(&membershipAuthority{resolution: terminology.Valid}),
	)
	if err != nil {
		t.Skipf("cannot create validator: %v", err)
	}

	// Patient with an extension whose SD is only available via the resolver.
	resource := []byte(`{
		"resourceType": "Patient",
		"extension": [
			{
				"url": "` + extURL + `",
				"valueString": "custom value"
			}
		],
		"name": [{"family": "Resolver"}]
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	// The extension SD is served by the resolver, so EXTENSION_UNKNOWN must NOT appear.
	for _, iss := range result.Issues {
		if iss.MessageID == "EXTENSION_UNKNOWN" {
			t.Errorf("unexpected EXTENSION_UNKNOWN: extension SD was not resolved via ProfileResolver. Diagnostic: %s", iss.Diagnostics)
		}
	}

	if resolver.calls == 0 {
		t.Error("ProfileResolver was never called — extension validator is still using GetByURL instead of ResolveByCanonical")
	}
}
