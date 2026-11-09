package validator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// El Scope de un recurso sale de su posición en el árbol (fhirpath.html#variables y
// bundle.html#references), no de quien lo referencia. Estas pruebas comparan los errores de gofhir
// con los de HL7 validator_cli 7.0.4 sobre las sondas de
// docs/plans/evidence/2026-11-06-canonical-scope/probe/ (c1: sc8 a sc10; c2: sc11 a sc14; d1: d1a1 a
// d1c2, el Scope del destino de "#id" desde un Bundle contenido; d3: d3a1 a d3c1, los grupos de
// holders que una corrida especulativa deja), en R4, R4B y R5.
//
// Los valores esperados salen de las salidas de HL7, no de gofhir: testdata/canonical_scope/expected.json
// lo genera probe/scripts/gen_expected.py desde probe/<grupo>/[v<versión>/]hl7-<sondas>-<versión>-7.0.4.json.
// Los perfiles y las instancias de testdata/canonical_scope/<grupo>/ son los de la sonda; los de R4B y R5
// son los de R4 con solo fhirVersion cambiado (probe/scripts/mkver.sh), y ninguna instancia cambia
// entre R4, R4B y R5.
//
// Diferencias conocidas que NO son de esta pieza, y que no se codifican como la salida de gofhir:
//   - HL7 informa un destino que no cumple una vez, en el primer referente; gofhir lo informa en cada
//     referente (sc6 a sc9). En sc8 y sc9 se exige el error que HL7 informa, y de más solo el
//     error de perfil de destino de extraReferrers, una vez.
//   - La política de ciclos de Conforms (probe/d5/, sc d5s1 a d5s7): HL7 supone que un ciclo de
//     discriminadores se cumple y gofhir responde false. Las sondas de d5 no se incluyen.
//   - Los mensajes de mínimo no se comparan por ubicación exacta: HL7 los ubica en el recurso y gofhir
//     en el elemento que falta (el elemento está dentro del recurso que HL7 nombra).

// scopeProbe son los errores de HL7 de una instancia, por versión de FHIR.
type scopeProbe struct {
	FirstReferrer bool                    `json:"firstReferrer"`
	Expected      map[string][]scopeIssue `json:"expected"`
}

// scopeIssue es un error: su clase y su ubicación sin la anotación /*Tipo/id*/ de HL7.
type scopeIssue struct {
	Kind     string `json:"kind"`
	Location string `json:"location"`
}

func (s scopeIssue) String() string { return s.Kind + " @ " + s.Location }

var scopeConstraintKey = regexp.MustCompile(`^Constraint failed: (\S+?):`)

// classifyOwn da la clase de un error de gofhir, por su ID de mensaje.
func classifyOwn(is issue.Issue) string {
	switch id := issue.DiagnosticID(is.MessageID); {
	case id == issue.DiagReferenceTargetProfile:
		return "target-profile"
	case id == issue.DiagCardinalityMin || id == issue.DiagSlicingCardinalityMin:
		return "min"
	case id == issue.DiagConstraintFailed:
		if m := scopeConstraintKey.FindStringSubmatch(is.Diagnostics); m != nil {
			return "constraint:" + m[1]
		}
	case strings.HasPrefix(is.MessageID, "BINDING") || strings.HasPrefix(is.MessageID, "CODESYSTEM"):
		return "binding"
	}
	return "other:" + is.MessageID
}

// scopeDefinitions lee los perfiles de un grupo, con fhirVersion de la versión.
func scopeDefinitions(t *testing.T, group, version string) [][]byte {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "canonical_scope", group, "ig", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("perfiles de %s: %v", group, err)
	}
	sort.Strings(files)
	out := make([][]byte, 0, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var d map[string]any
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		if _, ok := d["fhirVersion"]; ok {
			d["fhirVersion"] = version
		}
		raw, err = json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, raw)
	}
	return out
}

// sameLocation dice si el error de gofhir está donde el de HL7: la misma ubicación, y para un mínimo
// (HL7 nombra el recurso) también un elemento dentro de ella.
func sameLocation(kind, expected, got string) bool {
	return expected == got || (kind == "min" && strings.HasPrefix(got, expected+"."))
}

// extraReferrers son, por sonda con firstReferrer, los errores de perfil de destino de más que
// gofhir da y HL7 no: el mismo destino y perfil, en otro de los que lo referencian. Se aceptan
// exactamente esos, cada uno las veces escritas; cualquier otro error de más falla.
var extraReferrers = map[string][]scopeIssue{
	"sc8": {{Kind: "target-profile", Location: "Patient.contained[1].signature.who"}},
	"sc9": {{Kind: "target-profile", Location: "Patient.contained[1].subject"}},
}

// compareScope devuelve los errores de HL7 que gofhir no da y los de gofhir que HL7 no da. Los
// errores de allowed (extraReferrers) no cuentan como de más, cada uno una vez.
func compareScope(expected, got, allowed []scopeIssue) (missing, extra []scopeIssue) {
	left := slices.Clone(got)
	for _, e := range expected {
		i := slices.IndexFunc(left, func(g scopeIssue) bool { return g.Kind == e.Kind && sameLocation(e.Kind, e.Location, g.Location) })
		if i < 0 {
			missing = append(missing, e)
			continue
		}
		left = slices.Delete(left, i, i+1)
	}
	allowed = slices.Clone(allowed)
	for _, g := range left {
		if i := slices.Index(allowed, g); i >= 0 {
			allowed = slices.Delete(allowed, i, i+1)
			continue
		}
		extra = append(extra, g)
	}
	return missing, extra
}

// El error de perfil de destino de más de otro referente se acepta solo en el sitio conocido, una
// vez; uno sin relación, o repetido, falla.
func TestCompareScopeAllowsOnlyTheKnownExtraReferrer(t *testing.T) {
	want := []scopeIssue{{Kind: "target-profile", Location: "Patient.contained[0].subject"}}
	known := scopeIssue{Kind: "target-profile", Location: "Patient.contained[1].subject"}
	hit := want[0]
	for name, tc := range map[string]struct {
		got       []scopeIssue
		wantExtra int
	}{
		"known":     {[]scopeIssue{hit, known}, 0},
		"unrelated": {[]scopeIssue{hit, known, {Kind: "target-profile", Location: "Patient.unrelated[99]"}}, 1},
		"repeated":  {[]scopeIssue{hit, known, known}, 1},
		"absent":    {[]scopeIssue{hit}, 0},
	} {
		missing, extra := compareScope(want, tc.got, []scopeIssue{known})
		if len(missing) != 0 || len(extra) != tc.wantExtra {
			t.Errorf("%s: missing=%v extra=%v", name, missing, extra)
		}
	}
}

func TestCanonicalScopeProbes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "canonical_scope", "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var probes map[string]map[string]scopeProbe
	if err := json.Unmarshal(raw, &probes); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		for group, instances := range probes {
			v, err := New(WithVersion(version), WithConformanceResources(scopeDefinitions(t, group, version)))
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, 0, len(instances))
			for name := range instances {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				probe := instances[name]
				t.Run(version+"/"+group+"/"+name, func(t *testing.T) {
					body, err := os.ReadFile(filepath.Join("testdata", "canonical_scope", group, "inst", name+".json"))
					if err != nil {
						t.Fatal(err)
					}
					res, err := v.Validate(context.Background(), body)
					if err != nil {
						t.Fatal(err)
					}
					var got []scopeIssue
					for _, is := range res.Issues {
						if is.Severity != issue.SeverityError && is.Severity != issue.SeverityFatal {
							continue
						}
						loc := ""
						if len(is.Expression) > 0 {
							loc = is.Expression[0]
						}
						got = append(got, scopeIssue{Kind: classifyOwn(is), Location: loc})
					}
					missing, extra := compareScope(probe.Expected[version], got, extraReferrers[name])
					if len(missing) > 0 || len(extra) > 0 {
						t.Errorf("HL7 7.0.4 da %v; faltan %v; de más %v", probe.Expected[version], missing, extra)
					}
				})
			}
		}
	}
}
