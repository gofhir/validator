package fixedpattern

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/internal/testfhir"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
)

// fixedPatient fija dos elementos de Patient cuyo orden por nombre (birthDate, gender) es el inverso
// del de la definición (gender, birthDate).
const fixedPatient = `{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/StructureDefinition/fixed-patient",
  "name": "FixedPatient",
  "status": "active",
  "kind": "resource",
  "abstract": false,
  "type": "Patient",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
  "derivation": "constraint",
  "differential": {
    "element": [
      {"id": "Patient", "path": "Patient"},
      {"id": "Patient.gender", "path": "Patient.gender", "fixedCode": "male"},
      {"id": "Patient.birthDate", "path": "Patient.birthDate", "fixedDate": "2000-01-01"}
    ]
  }
}`

// Validate da los valores fijos violados en orden de nombre del elemento, siempre el mismo (#174;
// elementdefinition.html#ElementDefinition.fixed_x_).
func TestValidateIssueOrder(t *testing.T) {
	want := []string{"Patient.birthDate", "Patient.gender"}
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		t.Run(version, func(t *testing.T) {
			reg := testfhir.Registry(t, version)
			pkg, err := loader.NewLoader("").LoadFromResources([][]byte{[]byte(fixedPatient)})
			if err != nil {
				t.Fatal(err)
			}
			if err := reg.LoadFromPackages([]*loader.Package{pkg}); err != nil {
				t.Fatal(err)
			}
			sd := reg.GetByURL("http://example.org/StructureDefinition/fixed-patient")
			if err := reg.EnsureSnapshot(context.Background(), sd); err != nil {
				t.Fatal(err)
			}
			v := New(reg)
			for range 30 {
				result := issue.NewResult()
				v.Validate([]byte(`{"resourceType":"Patient","gender":"female","birthDate":"1990-05-05"}`), sd, result)
				got := make([]string, 0, len(result.Issues))
				for _, is := range result.Issues {
					got = append(got, strings.Join(is.Expression, ","))
				}
				if !slices.Equal(got, want) {
					t.Fatalf("issues at %v, want %v", got, want)
				}
			}
		})
	}
}
