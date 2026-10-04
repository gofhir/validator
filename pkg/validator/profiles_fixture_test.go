package validator

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/specs"
	"github.com/gofhir/validator/pkg/terminology"
)

// The profiles of the profile-loading tests. Each has its own URL and its own conformance package,
// so one validator holds them all without one test seeing another's profile.
const (
	igPatientProfileURL     = "https://example.org/fhir/StructureDefinition/test-patient-profile"
	strictPatientProfileURL = "https://example.org/fhir/StructureDefinition/strict-patient"
	noPhotoProfileURL       = "http://example.org/StructureDefinition/no-photo-patient"
	diffOnlyProfileURL      = "http://example.org/StructureDefinition/diff-only-patient"
)

// requireIdentifier is a differential-only Patient profile that requires an identifier.
func requireIdentifier(url, name string) []byte {
	return []byte(`{
		"resourceType": "StructureDefinition",
		"url": "` + url + `",
		"name": "` + name + `",
		"status": "active",
		"type": "Patient",
		"kind": "resource",
		"abstract": false,
		"derivation": "constraint",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
		"differential": {
			"element": [
				{"id": "Patient", "path": "Patient"},
				{"id": "Patient.identifier", "path": "Patient.identifier", "min": 1}
			]
		}
	}`)
}

// noPhotoProfile copies the core Patient snapshot with Patient.photo prohibited (max = 0).
func noPhotoProfile() ([]byte, error) {
	core, err := coreDefinition("http://hl7.org/fhir/StructureDefinition/Patient")
	if err != nil {
		return nil, err
	}
	base := core["snapshot"].(map[string]any)["element"].([]any)
	elements := make([]any, 0, len(base))
	for _, e := range base {
		m := e.(map[string]any)
		upper := m["max"]
		if m["path"] == "Patient.photo" {
			upper = "0"
		}
		elements = append(elements, map[string]any{"id": m["id"], "path": m["path"], "min": m["min"], "max": upper})
	}
	return json.Marshal(map[string]any{
		"resourceType": "StructureDefinition", "url": noPhotoProfileURL, "name": "NoPhotoPatient", "status": "active",
		"kind": "resource", "abstract": false, "type": "Patient", "derivation": "constraint",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient", "snapshot": map[string]any{"element": elements},
	})
}

// coreDefinition is a core R4 definition, decoded from the embedded package.
func coreDefinition(url string) (map[string]any, error) {
	pkgs, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages("4.0.1"))
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		for _, raw := range p.Resources {
			var peek struct{ URL string }
			if json.Unmarshal(raw, &peek) == nil && peek.URL == url {
				var def map[string]any
				if err := json.Unmarshal(raw, &def); err != nil {
					return nil, err
				}
				return def, nil
			}
		}
	}
	return nil, fmt.Errorf("%s not found", url)
}

// profileFixture is one validator for the profile-loading tests: building one costs tens of
// seconds under -race. Terminology is not under test here; an authority skips parsing the base
// ValueSets/CodeSystems, the dominant cost of building a validator under -race and coverage.
var profileFixture = sync.OnceValues(func() (*Validator, error) {
	noPhoto, err := noPhotoProfile()
	if err != nil {
		return nil, err
	}
	return New(
		WithVersion("4.0.1"),
		WithConformancePackage("test.ig", "1.0.0", [][]byte{requireIdentifier(igPatientProfileURL, "TestPatientProfile")}),
		WithConformancePackage("test.strict.ig", "2.0.0", [][]byte{requireIdentifier(strictPatientProfileURL, "StrictPatient")}),
		WithConformancePackage("test.profiles", "0.0.1", [][]byte{noPhoto, requireIdentifier(diffOnlyProfileURL, "DiffOnlyPatient"), bornProfile, evalProfile}),
		WithConformancePackage("test.typed", "0.0.1", typedProfiles),
		WithConformancePackage("test.tree", "0.0.1", treeProfiles),
		WithConformancePackage("test.extdefs", "0.0.1", extensionDefinitions),
		WithTerminologyAuthority(&membershipAuthority{resolution: terminology.Valid}),
	)
})

func profileValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := profileFixture()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}
