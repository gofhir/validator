package structural

import (
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// Plan: docs/plans/2026-11-02-empty-array.md (D5, D8) y ADR-003, issue #190.

var (
	r4Once    sync.Once
	r4Reg     *registry.Registry
	errR4Load error
)

// r4Registry es un registro de R4 con su versión puesta (SetFHIRVersion antes de cargar): la regla de
// los primitivos depende de ella.
func r4Registry(t *testing.T) *registry.Registry {
	t.Helper()
	r4Once.Do(func() {
		packages, err := loader.NewLoader("").LoadVersion("4.0.1")
		if err != nil {
			errR4Load = err
			return
		}
		reg := registry.New()
		reg.SetFHIRVersion("4.0.1")
		errR4Load = reg.LoadFromPackages(packages)
		r4Reg = reg
	})
	if errR4Load != nil {
		t.Skipf("Cannot load FHIR packages: %v", errR4Load)
	}
	return r4Reg
}

func arrayEmptyPaths(res *issue.Result) []string {
	var out []string
	for _, is := range res.Issues {
		if is.MessageID == string(issue.DiagStructureArrayEmpty) {
			out = append(out, strings.Join(is.Expression, ","))
		}
	}
	slices.Sort(out)
	return out
}

func parse(t *testing.T, src string) map[string]any {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(src), &data); err != nil {
		t.Fatal(err)
	}
	return data
}

// D5: el array vacío se reporta solo en Normalize, sobre la fuente; la fase estructural, sin edits, no
// lo repite.
func TestEmptyArrayReportedOnlyByNormalize(t *testing.T) {
	reg := r4Registry(t)
	v := New(reg)
	sd := reg.GetByType("Patient")
	for _, tt := range []struct {
		name, src string
		want      []string
	}{
		{"complex", `{"resourceType":"Patient","name":[]}`, []string{"Patient.name"}},
		{"primitive in R4", `{"resourceType":"Patient","name":[{"family":"X","given":[]}]}`, []string{"Patient.name[0].given"}},
		{"shadow in R4", `{"resourceType":"Patient","name":[{"family":"X","_given":[]}]}`, []string{"Patient.name[0].given"}},
		{"several", `{"resourceType":"Patient","name":[],"telecom":[],"contact":[{"name":{"family":"X","extension":[]}}]}`,
			[]string{"Patient.contact[0].name.extension", "Patient.name", "Patient.telecom"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := arrayEmptyPaths(v.ValidateData(parse(t, tt.src), sd)); len(got) != 0 {
				t.Errorf("ValidateData reports %q, want none", got)
			}
			n := v.Normalize(parse(t, tt.src), []byte(tt.src), sd)
			if got := arrayEmptyPaths(n.Issues); !slices.Equal(got, tt.want) {
				t.Errorf("Normalize reports %q, want %q", got, tt.want)
			}
		})
	}
}

// D8: el chequeo solo depende de que el elemento se repita. Un elemento que se repite con dos tipos y
// cuyo path no termina en [x] no tiene tipo conocido (primitiveKind), y su [] se reporta igual.
func TestEmptyArrayOfAnElementOfSeveralTypes(t *testing.T) {
	reg := setupTestRegistry(t) // sin versión: la regla de los primitivos no interviene
	base := reg.GetByType("Patient")
	elements := append([]registry.ElementDefinition{}, base.Snapshot.Element...)
	elements = append(elements, registry.ElementDefinition{
		ID: "Patient.multi", Path: "Patient.multi", Max: "*",
		Base: &registry.ElementBase{Path: "Patient.multi", Max: "*"},
		Type: []registry.Type{{Code: "HumanName"}, {Code: "Address"}},
	})
	sd := &registry.StructureDefinition{URL: "http://example.org/StructureDefinition/patient-multi", Type: "Patient",
		Kind: base.Kind, Snapshot: &registry.Snapshot{Element: elements}}
	src := `{"resourceType":"Patient","multi":[]}`
	n := New(reg).Normalize(parse(t, src), []byte(src), sd)
	if got, want := arrayEmptyPaths(n.Issues), []string{"Patient.multi"}; !slices.Equal(got, want) {
		t.Errorf("Normalize reports %q, want %q", got, want)
	}
}

// La regla de ADR-003 por versión: json.html de DSTU2, STU3, R4 y R4B dice «arrays are never empty»; el de
// R5 y después no. Una versión vacía sigue la regla de R5.
func TestEmptyPrimitiveArraysForbidden(t *testing.T) {
	for version, want := range map[string]bool{
		"": false, "1.0.2": true, "3.0.2": true, "4.0.1": true, "4.3.0": true, "4.3.0-snapshot1": true,
		"5.0.0": false, "6.0.0-ballot3": false,
	} {
		if got := emptyPrimitiveArraysForbidden(version); got != want {
			t.Errorf("emptyPrimitiveArraysForbidden(%q) = %t, want %t", version, got, want)
		}
	}
}
