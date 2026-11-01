package extension

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// La fase usada sola (ValidateData es pública) compara el tipo del valor de una extensión con el que
// declara su definición: la url de la extensión nombra la definición a la que se ajusta, y esa
// definición gobierna su valor (extensibility.html). Se informa una vez, en la clave del valor.
func TestStandaloneExtensionValueType(t *testing.T) {
	const want = "Patient.extension[0].valueString"
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		t.Run(version, func(t *testing.T) {
			reg := phaseRegistry(t, version)
			var data map[string]any
			err := json.Unmarshal([]byte(`{"resourceType":"Patient","extension":[{"url":"http://example.org/StructureDefinition/bool-ext","valueString":"bad"}]}`), &data)
			if err != nil {
				t.Fatal(err)
			}
			res := issue.NewResult()
			New(reg, nil, nil).ValidateData(context.Background(), data, reg.GetByType("Patient"), res)
			n := 0
			for _, is := range res.Issues {
				if is.MessageID == string(issue.DiagStructureTypeNotAllowed) && len(is.Expression) == 1 && is.Expression[0] == want {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("got %d %s at %s; want 1\n%+v", n, issue.DiagStructureTypeNotAllowed, want, res.Issues)
			}
		})
	}
}
