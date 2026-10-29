package fixedpattern

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/internal/testfhir"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
)

// fixedComponent fija value[x] de Observation.component, no el de Observation.
const fixedComponent = `{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/StructureDefinition/fixed-component",
  "name": "FixedComponent",
  "status": "active",
  "kind": "resource",
  "abstract": false,
  "type": "Observation",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Observation",
  "derivation": "constraint",
  "differential": {
    "element": [
      {"id": "Observation", "path": "Observation"},
      {"id": "Observation.component.value[x]", "path": "Observation.component.value[x]", "fixedString": "a"}
    ]
  }
}`

// fixedRestricted deja value[x] de Observation solo en string, con un valor fijo.
const fixedRestricted = `{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/StructureDefinition/fixed-restricted",
  "name": "FixedRestricted",
  "status": "active",
  "kind": "resource",
  "abstract": false,
  "type": "Observation",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Observation",
  "derivation": "constraint",
  "differential": {
    "element": [
      {"id": "Observation", "path": "Observation"},
      {"id": "Observation.value[x]", "path": "Observation.value[x]", "type": [{"code": "string"}], "fixedString": "ok"}
    ]
  }
}`

// Una clave nombra la variante de un elemento de elección de su mismo padre con el código del tipo
// en inicial mayúscula (formats.html#choice); los nombres distinguen mayúsculas (json.html). La
// clave mal escrita no es el elemento y no se compara con su valor fijo, y value[x] de Observation
// no es el de component (#180).
func TestValidateChoiceKey(t *testing.T) {
	const obs = `{"resourceType":"Observation","status":"final","code":{"text":"x"},`
	for _, tc := range []struct {
		name, profile, members string
		want                   []string
	}{
		{"variant", fixedComponent, `"component":[{"code":{"text":"c"},"valueString":"b"}]`, []string{"Observation.component[0].valueString"}},
		{"lowercase type", fixedComponent, `"component":[{"code":{"text":"c"},"valuestring":"b"}]`, nil},
		{"uppercase type", fixedComponent, `"component":[{"code":{"text":"c"},"valueSTRING":"b"}]`, nil},
		{"another level", fixedComponent, `"valueString":"b"`, nil},
		// Un tipo que el perfil no permite y la base sí nombra un valor, del tipo equivocado, que
		// tampoco es el fijo (HL7 6.10.2 da los dos errores: evidence/2026-10-27-choice-key-case,
		// probe-fp/f06).
		{"type the profile does not allow", fixedRestricted, `"valueBoolean":true`, []string{"Observation.valueBoolean"}},
	} {
		for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				reg := testfhir.Registry(t, version)
				pkg, err := loader.NewLoader("").LoadFromResources([][]byte{[]byte(tc.profile)})
				if err != nil {
					t.Fatal(err)
				}
				if err := reg.LoadFromPackages([]*loader.Package{pkg}); err != nil {
					t.Fatal(err)
				}
				var head struct {
					URL string `json:"url"`
				}
				if err := json.Unmarshal([]byte(tc.profile), &head); err != nil {
					t.Fatal(err)
				}
				sd := reg.GetByURL(head.URL)
				if err := reg.EnsureSnapshot(context.Background(), sd); err != nil {
					t.Fatal(err)
				}
				result := issue.NewResult()
				New(reg).Validate([]byte(obs+tc.members+`}`), sd, result)
				got := make([]string, 0, len(result.Issues))
				for _, is := range result.Issues {
					got = append(got, strings.Join(is.Expression, ","))
				}
				if !slices.Equal(got, tc.want) {
					t.Errorf("issues at %v, want %v", got, tc.want)
				}
			})
		}
	}
}
