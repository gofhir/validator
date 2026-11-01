package validator

import (
	"context"
	"strings"
	"testing"
)

// json.html#primitive: las Extension de un primitivo viven en la propiedad "_x" del JSON, pero la
// expresión FHIRPath de un issue sobre ellas es la del elemento (Patient.birthDate.extension[0]...).
// El issue conserva sus coordenadas en el JSON original; la columna esperada se calcula del texto
// de entrada: justo después del nombre de la propiedad que el issue señala.
func TestPrimitiveSiblingIssueLocation(t *testing.T) {
	bad := `[{"url":"` + extBase + `flag-ext","valueString":"bad"}]`
	cases := []struct {
		name, data string
		expr       string
	}{
		{
			"object primitive",
			`{"resourceType":"Patient","birthDate":"2000-01-01","_birthDate":{"extension":` + bad + `}}`,
			"Patient.birthDate.extension[0].valueString",
		},
		{
			"primitive with no value",
			`{"resourceType":"Patient","_birthDate":{"extension":` + bad + `}}`,
			"Patient.birthDate.extension[0].valueString",
		},
		{
			"array primitive",
			`{"resourceType":"Patient","name":[{"given":["A","B"],"_given":[null,{"extension":` + bad + `}]}]}`,
			"Patient.name[0].given[1].extension[0].valueString",
		},
	}
	for _, version := range orderVersions {
		v := orderValidator(t, version, extValueDefs(t)...)
		for _, c := range cases {
			t.Run(version+"/"+c.name, func(t *testing.T) {
				r, err := v.Validate(context.Background(), []byte(c.data))
				if err != nil {
					t.Fatal(err)
				}
				found := 0
				for _, is := range r.Issues {
					if len(is.Expression) == 0 || !strings.HasSuffix(is.Expression[0], ".extension[0].valueString") {
						continue
					}
					found++
					if is.Location == nil {
						t.Errorf("lost source coordinates: %+v", is)
						continue
					}
					// Una sola línea: la columna es el offset 0-indexado más uno.
					want := strings.LastIndex(c.data, `"valueString"`) + len(`"valueString"`) + 1
					if is.Location.Line != 1 || is.Location.Column != want {
						t.Errorf("location %+v, want line 1 column %d (%s)", is.Location, want, is.Expression[0])
					}
				}
				if found != 1 {
					t.Fatalf("got %d issues at %s, want 1: %+v", found, c.expr, r.Issues)
				}
			})
		}
	}
}
