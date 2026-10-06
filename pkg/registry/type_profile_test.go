package registry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

const simpleQuantity = "http://hl7.org/fhir/StructureDefinition/SimpleQuantity"

// typeProfileRegistry is R4 plus an Observation profile whose value[x] declares, per element of
// the test, a different set of type profiles.
func typeProfileRegistry(t *testing.T) *Registry {
	t.Helper()
	r := loadVersion(t, "4.0.1")
	obs := func(id, types string) json.RawMessage {
		return json.RawMessage(`{"resourceType":"StructureDefinition","url":"https://example.org/` + id + `","name":"P","type":"Observation",
"kind":"resource","fhirVersion":"4.0.1","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation",
"snapshot":{"element":[{"id":"Observation","path":"Observation"},{"id":"Observation.value[x]","path":"Observation.value[x]","type":` + types + `}]}}`)
	}
	defs := map[string]json.RawMessage{
		"choice":     obs("choice", `[{"code":"Quantity","profile":["`+simpleQuantity+`"]},{"code":"string"}]`),
		"several":    obs("several", `[{"code":"Quantity","profile":["`+simpleQuantity+`","http://hl7.org/fhir/StructureDefinition/MoneyQuantity"]}]`),
		"unresolved": obs("unresolved", `[{"code":"Quantity","profile":["https://example.org/nothing"]}]`),
		"version":    obs("version", `[{"code":"Quantity","profile":["`+simpleQuantity+`|9.9.9"]}]`),
		"nosnapshot": obs("nosnapshot", `[{"code":"Quantity","profile":["https://example.org/qty-no-base"]}]`),
		"qty-no-base": json.RawMessage(`{"resourceType":"StructureDefinition","url":"https://example.org/qty-no-base","name":"Q","type":"Quantity",
"kind":"complex-type","fhirVersion":"4.0.1","derivation":"constraint","baseDefinition":"https://example.org/missing-base",
"differential":{"element":[{"id":"Quantity","path":"Quantity"}]}}`),
	}
	if err := r.LoadFromPackages([]*loader.Package{{Name: "test", Version: "0.0.1", Resources: defs}}); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTypeProfile(t *testing.T) {
	r := typeProfileRegistry(t)
	node := func(url, id string) *ElementNode {
		t.Helper()
		sd := r.GetByURL(url)
		if sd == nil {
			t.Fatalf("%s not loaded", url)
		}
		return sd.Tree().ByID(id)
	}
	for _, tt := range []struct {
		name, url, id, typeCode string
		canonical, sd, reason   string // sd is the resolved definition's URL
	}{
		{"one type, one profile", "http://hl7.org/fhir/StructureDefinition/MedicationRequest", "MedicationRequest.dispenseRequest.quantity", "",
			simpleQuantity, simpleQuantity, ""},
		{"a choice picks the value's type", "https://example.org/choice", "Observation.value[x]", "Quantity", simpleQuantity, simpleQuantity, ""},
		{"a choice's type with no profile", "https://example.org/choice", "Observation.value[x]", "string", "", "", ""},
		{"a choice needs the value's type", "https://example.org/choice", "Observation.value[x]", "", "", "", ""},
		{"several profiles", "https://example.org/several", "Observation.value[x]", "Quantity", "", "", ""},
		{"not loaded", "https://example.org/unresolved", "Observation.value[x]", "Quantity", "https://example.org/nothing", "", "not-found"},
		{"pinned version not loaded", "https://example.org/version", "Observation.value[x]", "Quantity", simpleQuantity + "|9.9.9", "", "version-missing"},
		{"no snapshot can be generated", "https://example.org/nosnapshot", "Observation.value[x]", "Quantity", "https://example.org/qty-no-base", "", "cannot generate snapshot"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tp := r.TypeProfile(context.Background(), node(tt.url, tt.id), tt.typeCode)
			canonical, reason, got := tp.Canonical, tp.Reason, ""
			if tp.SD != nil {
				got = tp.SD.URL
			}
			if tp.Canonical != "" && tp.TypeCode != "Quantity" {
				t.Errorf("type %q, want Quantity", tp.TypeCode)
			}
			if canonical != tt.canonical || got != tt.sd || !strings.Contains(reason, tt.reason) || (tt.reason == "" && reason != "") {
				t.Errorf("got (%q, %q, %q), want (%q, %q, %q)", canonical, got, reason, tt.canonical, tt.sd, tt.reason)
			}
		})
	}
}
