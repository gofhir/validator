package extension

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/internal/testfhir"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// phaseRegistry carga cada extensión de prueba de pkg/validator/testdata/extension-value, con su
// snapshot.
func phaseRegistry(t *testing.T, version string) *registry.Registry {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "validator", "testdata", "extension-value", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no definitions: %v", err)
	}
	raw := make([][]byte, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, data)
	}
	reg := testfhir.Registry(t, version)
	pkg, err := loader.NewLoader("").LoadFromResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"bool-ext", "str-ext", "opt-str-ext", "id-ext", "flag-ext"} {
		if err := reg.EnsureSnapshot(context.Background(), reg.GetByURL("http://example.org/StructureDefinition/"+url)); err != nil {
			t.Fatal(err)
		}
	}
	return reg
}

// La fase recibe también lo que la fase estructural deja fuera en el pipeline (ValidateData es
// pública): un valor que no está escrito como su elemento dice no es un valor. No cuenta para «tiene
// valor», no se le revisa el tipo y no se valida lo que tiene adentro; HL7 informa solo que falta
// (evidence/2026-10-27-extension-value-content/tests-hl7/unwritten).
func TestUnwrittenValueIsNoValue(t *testing.T) {
	const base = "http://example.org/StructureDefinition/"
	required := []string{string(issue.DiagExtensionValueRequired) + " Patient.extension[0]"}
	tests := []struct {
		name, ext string
		want      []string
	}{
		{"null", `{"url":"` + base + `str-ext","valueString":null}`, required},
		{"array where one goes", `{"url":"` + base + `id-ext","valueIdentifier":[{"system":"urn:x"}]}`, required},
		{"object for a primitive", `{"url":"` + base + `str-ext","valueString":{"extension":[{"url":"` + base + `flag-ext","valueString":"bad"}]}}`, required},
		{"null of a type not allowed", `{"url":"` + base + `bool-ext","valueString":null}`, required},
		// Con un valor opcional no falta nada, y la extensión de adentro no se valida.
		{"object for an optional primitive", `{"url":"` + base + `opt-str-ext","valueString":{"extension":[{"url":"` + base + `flag-ext","valueString":"bad"}]}}`, nil},
	}
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		reg := phaseRegistry(t, version)
		v := New(reg, nil, nil)
		for _, tt := range tests {
			t.Run(version+"/"+tt.name, func(t *testing.T) {
				var resource map[string]any
				if err := json.Unmarshal([]byte(`{"resourceType":"Patient","extension":[`+tt.ext+`]}`), &resource); err != nil {
					t.Fatal(err)
				}
				result := issue.NewResult()
				v.ValidateData(context.Background(), resource, reg.GetByType("Patient"), result)
				var got []string
				for _, is := range result.Issues {
					if is.MessageID != string(issue.DiagExtensionInvalidContext) {
						got = append(got, is.MessageID+" "+strings.Join(is.Expression, ","))
					}
				}
				if !slices.Equal(got, tt.want) {
					t.Errorf("got %v, want %v", got, tt.want)
				}
			})
		}
	}
}
