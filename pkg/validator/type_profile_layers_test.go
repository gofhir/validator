package validator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gofhir/validator/v2/internal/scopeindex"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicing"
)

// Capas por valor de `type.profile` con varios perfiles (#193, pieza 2).
//
// R4 elementdefinition-definitions.html#ElementDefinition.type.profile: «If any profiles are specified,
// then the content must conform to at least one of them». Un valor cuyo elemento declara dos o más
// perfiles para su tipo se elige por valor (los perfiles que cumple, o todos si no cumple ninguno), y
// cada fase que baja por los hijos controla los de cada capa elegida. Sin perfil que cumpla se informa
// PROFILE_NO_MATCH, con varios PROFILE_MULTIPLE_MATCHES, y un perfil que no se resuelve es
// TYPE_PROFILE_NOT_FOUND (plan: docs/plans/2026-11-06-type-profile-layers.md, §2 v4 a v6 y §5).
//
// Los valores esperados salen de las salidas de HL7 validator_cli 7.0.4 (-tx n/a), no de gofhir:
// testdata/type_profile_layers/expected.json lo genera
// docs/plans/evidence/2026-11-14-type-profile-layers/tests/gen_expected.py desde las sondas de
// .../probe/ (p1 a p21 y las de la pieza) y .../probe-callers/ (una por llamador), en R4, R4B y R5. Los perfiles
// y las instancias de testdata/type_profile_layers/ son los de las sondas, con fhirVersion de R4 (el test
// lo ajusta por versión). Correspondencia de IDs: Validation_VAL_Profile_NoMatch es PROFILE_NO_MATCH,
// Validation_VAL_Profile_MultipleMatches es PROFILE_MULTIPLE_MATCHES, Validation_VAL_Unknown_Profile es
// TYPE_PROFILE_NOT_FOUND, y los demás como en canonical_scope_test.go.
//
// Excepciones declaradas, cada una con su comentario en gen_expected.py (el valor esperado no es el de HL7):
//   - p12: un tipo de elección usa todos sus perfiles (la especificación dice «at least one»); HL7 usa el primero.
//   - p20: un perfil de otro tipo queda sin caminar; HL7 camina el Quantity contra un perfil de CodeableConcept.
//   - p21: la cardinalidad común a dos capas se informa una vez; HL7 la repite porque su texto nombra el origen.
//   - e2s* y e2h* (E2, sonda 11): el discriminador por perfil evalúa un tipo de datos con el Scope canónico de su
//     recurso dueño (fhirpath.html#variables: «%resource: the resource that contains the original node»); HL7
//     toma el propio tipo de datos. En `h` el dueño es la Observation que resolve() halla.
//   - bcyc3: HL7 no informa el perfil de destino de una referencia dentro de un tipo de datos; el valor es el de la
//     especificación.
//   - kucum: HL7 con -tx n/a no valida un código UCUM; el valor es el de la especificación de UCUM.
//
// Fuera de esta pieza y no comparado: dom-3 de un contenido sin referenciar (HL7 lo ubica en el contenido y
// gofhir en el contenedor), los avisos de terminología y las mejores prácticas.

const layersDir = "testdata/type_profile_layers"

// layerIssue es un informe esperado: su clase, severidad y ubicación, más el detalle que la distingue (la
// clave de una invariante, el elemento de una cardinalidad, el canónico de un perfil que no se resuelve) y el
// texto de la invariante cuando se comprueba.
type layerIssue struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Location string `json:"location"`
	Detail   string `json:"detail"`
	Text     string `json:"text"`
}

func (l layerIssue) String() string {
	s := l.Severity + " " + l.Kind + " @ " + l.Location
	if l.Detail != "" {
		s += " [" + l.Detail + "]"
	}
	return s
}

// layerProbe son los informes de HL7 de una instancia, por versión de FHIR.
type layerProbe struct {
	Description string                  `json:"description"`
	Note        string                  `json:"note"`
	Expected    map[string][]layerIssue `json:"expected"`
}

var layerVersions = []string{"4.0.1", "4.3.0", "5.0.0"}

var (
	layerProbesOnce sync.Once
	layerProbesVal  map[string]layerProbe
	layerValsMu     sync.Mutex
	layerVals       = map[string]*Validator{}
)

func layerProbes(t testing.TB) map[string]layerProbe {
	t.Helper()
	layerProbesOnce.Do(func() {
		raw, err := os.ReadFile(filepath.Join(layersDir, "expected.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &layerProbesVal); err != nil {
			t.Fatal(err)
		}
	})
	return layerProbesVal
}

// layerDefinitions lee los perfiles de las sondas, con la versión de FHIR de version (los archivos son
// de R4: el canónico versionado de p17 y fhirVersion llevan «4.0.1»).
func layerDefinitions(t testing.TB, version string) [][]byte {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(layersDir, "ig", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("perfiles: %v", err)
	}
	sort.Strings(files)
	out := make([][]byte, 0, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, bytes.ReplaceAll(raw, []byte("4.0.1"), []byte(version)))
	}
	return out
}

func layerValidator(t testing.TB, version string) *Validator {
	t.Helper()
	layerValsMu.Lock()
	defer layerValsMu.Unlock()
	if v, ok := layerVals[version]; ok {
		return v
	}
	v, err := New(WithVersion(version), WithConformanceResources(layerDefinitions(t, version)))
	if err != nil {
		t.Fatal(err)
	}
	layerVals[version] = v
	return v
}

func layerInstance(t testing.TB, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(layersDir, "inst", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// gotIssue es un informe de gofhir, clasificado como los de HL7.
type gotIssue struct {
	layerIssue
	Diagnostics string
}

var layerConstraint = regexp.MustCompile(`^Constraint failed: (\S+?): '(.*?)'(?: \(|$)`)

// classifyLayers da la clase de un informe de gofhir por su ID de mensaje. Los avisos solo cuentan si son de
// varios perfiles o la invariante dup-1 (la de las sondas de E3). Los demás (narrativa, terminología, mejores
// prácticas) y dom-3 no son de esta pieza (ver arriba).
func classifyLayers(is issue.Issue) (gotIssue, bool) {
	var sev string
	switch is.Severity {
	case issue.SeverityError, issue.SeverityFatal:
		sev = "error"
	case issue.SeverityWarning:
		sev = "warning"
	default:
		return gotIssue{}, false
	}
	loc := ""
	if len(is.Expression) > 0 {
		loc = is.Expression[0]
	}
	g := gotIssue{layerIssue: layerIssue{Severity: sev, Location: loc}, Diagnostics: is.Diagnostics}
	switch id := is.MessageID; {
	case id == "PROFILE_NO_MATCH":
		g.Kind = "profile-no-match"
	case id == "PROFILE_MULTIPLE_MATCHES":
		g.Kind = "profile-multiple-matches"
	case id == string(issue.DiagTypeProfileNotFound):
		g.Kind = "type-profile-not-found"
	case id == string(issue.DiagCardinalityMin) || id == string(issue.DiagSlicingCardinalityMin):
		g.Kind = "min"
	case id == string(issue.DiagCardinalityMax) || id == string(issue.DiagSlicingCardinalityMax):
		g.Kind = "max"
	case id == string(issue.DiagConstraintFailed):
		m := layerConstraint.FindStringSubmatch(is.Diagnostics)
		if m == nil {
			return gotIssue{}, false
		}
		if m[1] == "dom-3" || (sev == "warning" && m[1] != "dup-1") {
			return gotIssue{}, false
		}
		g.Kind, g.Detail, g.Text = "constraint", m[1], m[2]
	case id == string(issue.DiagReferenceTargetProfile):
		g.Kind = "ref-target-profile"
	case id == string(issue.DiagSlicingNoMatch):
		g.Kind = "slice-no-match"
	case id == string(issue.DiagExtensionInvalidContext):
		g.Kind = "extension-context"
	case strings.HasPrefix(id, "BINDING") || strings.HasPrefix(id, "CODESYSTEM"):
		g.Kind = "binding"
	case id == string(issue.DiagUCUMInvalidCode):
		g.Kind = "ucum"
	case id == string(issue.DiagStructureTypeNotAllowed):
		g.Kind = "type-not-allowed"
	case id == string(issue.DiagStructureUnknownElement):
		g.Kind = "unknown-element"
	case strings.HasPrefix(id, "TYPE_INVALID") || id == "TYPE_WRONG_JSON_TYPE":
		g.Kind = "type-invalid"
	default:
		g.Kind = "other:" + id
	}
	if sev == "warning" && g.Kind != "profile-multiple-matches" && g.Kind != "constraint" {
		return gotIssue{}, false
	}
	return g, true
}

var layerIndex = regexp.MustCompile(`\[\d+\]|:[^.]*`)

// lastSegment es el último elemento de una ubicación, sin índices ni nombres de slice.
func lastSegment(loc string) string {
	loc = layerIndex.ReplaceAllString(loc, "")
	return loc[strings.LastIndex(loc, ".")+1:]
}

// matchLayer dice si el informe de gofhir es el de HL7. Una cardinalidad, que HL7 ubica en el valor y gofhir
// en el hijo que falta, coincide con el hijo del mismo nombre bajo esa ubicación.
func matchLayer(exp layerIssue, got gotIssue) bool {
	if exp.Kind != got.Kind || exp.Severity != got.Severity {
		return false
	}
	switch exp.Kind {
	case "type-profile-not-found":
		return exp.Location == got.Location && strings.Contains(got.Diagnostics, exp.Detail)
	case "min", "max":
		if got.Location == exp.Location {
			return strings.Contains(got.Diagnostics, exp.Detail)
		}
		return strings.HasPrefix(got.Location, exp.Location+".") && lastSegment(got.Location) == exp.Detail
	case "constraint":
		return exp.Location == got.Location && exp.Detail == got.Detail && (exp.Text == "" || exp.Text == got.Text)
	}
	return exp.Location == got.Location
}

// compareLayers devuelve los informes de HL7 que gofhir no da y los de gofhir que HL7 no da, cada informe
// una vez por aparición.
func compareLayers(expected []layerIssue, got []gotIssue) (missing []layerIssue, extra []gotIssue) {
	left := slices.Clone(got)
	for _, e := range expected {
		i := slices.IndexFunc(left, func(g gotIssue) bool { return matchLayer(e, g) })
		if i < 0 {
			missing = append(missing, e)
			continue
		}
		left = slices.Delete(left, i, i+1)
	}
	return missing, left
}

// layerRun valida una instancia de las sondas y da los informes de gofhir que esta pieza compara.
func layerRun(t testing.TB, v *Validator, name string) []gotIssue {
	t.Helper()
	res, err := v.Validate(context.Background(), layerInstance(t, name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out []gotIssue
	for _, is := range res.Issues {
		if g, ok := classifyLayers(is); ok {
			out = append(out, g)
		}
	}
	return out
}

func extraStrings(extra []gotIssue) []string {
	out := make([]string, len(extra))
	for i, g := range extra {
		out[i] = g.String() + " | " + g.Diagnostics
	}
	return out
}

// runLayerProbes compara a gofhir con HL7 en cada instancia de names, en R4, R4B y R5.
func runLayerProbes(t *testing.T, names []string) {
	t.Helper()
	probes := layerProbes(t)
	for _, version := range layerVersions {
		v := layerValidator(t, version)
		for _, name := range names {
			probe, ok := probes[name]
			if !ok {
				t.Fatalf("la sonda %s no está en expected.json", name)
			}
			t.Run(version+"/"+name, func(t *testing.T) {
				missing, extra := compareLayers(probe.Expected[version], layerRun(t, v, name))
				if len(missing) > 0 || len(extra) > 0 {
					t.Errorf("%s\nHL7 7.0.4 da %v\nfaltan %v\nde más %v", probe.Note, probe.Expected[version], missing, extraStrings(extra))
				}
			})
		}
	}
}

// expand da los nombres de las sondas que empiezan con prefix y siguen con lo que dice re.
func expand(t *testing.T, re string) []string {
	t.Helper()
	rx := regexp.MustCompile("^(?:" + re + ")$")
	var out []string
	for n := range layerProbes(t) {
		if rx.MatchString(n) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("ninguna sonda coincide con %q", re)
	}
	return out
}

// piece3Probes son las sondas de valores primitivos (valueString con varios perfiles de tipo), que
// quedan fuera de esta pieza (#193, pieza 3). Su evidencia y su valor esperado siguen en testdata/ y expected.json;
// la pieza 3 las corre con runLayerProbes(t, piece3Probes).
var piece3Probes = []string{"p14", "p15"}

// p1 a p21 (2026-11-05): elección por valor, NoMatch, MultipleMatches, perfil que no se resuelve, repetido,
// extensión. P4 y p11 son de un solo perfil y están en el grupo de regresión; p12, p20 y p21 son las divergencias
// declaradas. Las de un valor primitivo (p14, p15) no corren aquí: son de la pieza 3 (piece3Probes).
func TestLayersProfileChoice(t *testing.T) {
	runLayerProbes(t, expand(t, `p([1-9]|1[0-3]|1[6-9]|2[01])`))
}

// Un solo perfil de tipo, o ninguno que se repita: la salida no cambia. Deben pasar antes y después de la pieza.
func TestLayersSingleProfileRegression(t *testing.T) {
	runLayerProbes(t, []string{"p4", "p11", "ref0n", "ref0x", "single1", "single2", "single3"})
}

// Un test por llamador (a), con la regla que falla solo en el segundo perfil de los dos que se eligen (ninguno
// cumple, así que se eligen los dos): cada uno mata «solo la primera capa» en su llamador.
//   - cardinalidad (pkg/cardinality): ele2, la propiedad `code` mínima de qb;
//   - holder (pkg/validator/holder.go) y reference (pkg/reference): ref1n, ref2n y refb1n, refb2n, el destino
//     de cada capa por separado (D2);
//   - extension (pkg/extension/extension.go): kext, la extensión requerida solo por qext;
//   - extension/context.go: kctx, la extensión con otro contexto;
//   - binding (pkg/binding/walk.go): kbind, el binding de `comparator` solo de qbind;
//   - UCUM (pkg/ucumvalidator): kucum, un código UCUM inválido bajo un tipo de datos con capas;
//   - slicing (pkg/slicing/tree.go): kslice, el slice requerido solo por cc-b.
//
// pkg/walker/discover.go no tiene una regla que falle solo en una capa: lo que descubre son los elementos que
// contienen recursos, y ningún tipo de datos de R4 los tiene (ver el informe de la pieza).
func TestLayersSecondLayerOnlyRuleInEachCaller(t *testing.T) {
	runLayerProbes(t, []string{"ele2", "two", "ref1n", "ref2n", "refb1n", "refb2n", "kext", "kctx", "kbind", "kucum", "kslice"})
}

// Cada llamador (a) informa la regla de la segunda capa, con su nombre en el mensaje del fallo.
func TestLayersEachCallerReportsTheSecondLayer(t *testing.T) {
	cases := []struct {
		caller, name, kind, detail string
		count                      int
	}{
		{"cardinality", "ele2", "min", "code", 1},
		{"holder", "ref1n", "ref-target-profile", "", 2},
		{"reference", "refb1n", "ref-target-profile", "", 2},
		{"extension", "kext", "min", "extension", 1},
		{"extension context", "kctx", "extension-context", "", 1},
		{"binding walk", "kbind", "binding", "", 1},
		{"UCUM", "kucum", "ucum", "", 1},
		{"slicing tree", "kslice", "min", "coding", 1},
	}
	for _, version := range layerVersions {
		v := layerValidator(t, version)
		for _, c := range cases {
			t.Run(version+"/"+c.caller, func(t *testing.T) {
				n := 0
				for _, g := range layerRun(t, v, c.name) {
					if g.Kind == c.kind && (c.detail == "" || g.Detail == c.detail || lastSegment(g.Location) == c.detail) {
						n++
					}
				}
				if n != c.count {
					t.Errorf("%s: %s: %d informes de %s %s, quiero %d (la segunda capa elegida)", c.caller, c.name, n, c.kind, c.detail, c.count)
				}
			})
		}
	}
}

// (c): un slice cuyo hijo coincide con la segunda capa y no con la primera. El mínimo del hijo del slice se
// informa aunque el nodo elija otra capa (c1b), y no cuando alguna capa lo cumple (c1a); c1d es el mismo slice
// con el tipo declarado también en él.
//
// Ronda 3 (d3, mutación 15b): el tipo y el slice comparten dos capas de CodeableConcept que no se cumplen, y solo la segunda
// asigna un perfil de Coding que exige `display`. El mínimo se informa una vez (CARDINALITY_MIN), y el slicing no suma un
// SLICING_CARDINALITY_MIN: su `definedIn` debe mirar todas las capas del tipo, no solo la primera (HL7: un mínimo, sin el del slice).
func TestLayersSliceChildAgainstTheLayers(t *testing.T) {
	runLayerProbes(t, append(expand(t, `c1[a-cn]|c1d[a-cn]`), "d3"))
}

// A1: una invariante que solo tiene el segundo perfil, en los dos órdenes de los perfiles.
func TestLayersInvariantOnlyInTheSecondProfile(t *testing.T) {
	runLayerProbes(t, expand(t, `a1r?[npq]`))
}

// A3: un slice hoja y un reslice con varios perfiles, y el discriminador por perfil.
func TestLayersSlicesWithSeveralProfiles(t *testing.T) {
	runLayerProbes(t, expand(t, `leaf[abc]|resl[abc]|pd[12](a|b|n|ab)`))
}

// A4: perfil recursivo, con referencia y con discriminador por perfil, en los dos órdenes.
func TestLayersBothOrders(t *testing.T) {
	runLayerProbes(t, expand(t, `rec[12][a-d]|pd[12](a|b|n|ab)|ref[12][nxy]|refb[12][nxy]|sev[12]|txt[12]`))
}

// El resultado de gofhir es el mismo en los dos órdenes de los perfiles (A4), sin mirar HL7: es lo que no puede
// ser distinto aunque HL7 cambiara.
func TestLayersResultDoesNotDependOnTheOrder(t *testing.T) {
	pairs := [][2]string{
		{"rec1a", "rec2a"}, {"rec1b", "rec2b"}, {"rec1c", "rec2c"}, {"rec1d", "rec2d"},
		{"pd1a", "pd2a"}, {"pd1b", "pd2b"}, {"pd1n", "pd2n"}, {"pd1ab", "pd2ab"},
		{"ref1x", "ref2x"}, {"ref1y", "ref2y"}, {"ref1n", "ref2n"},
		{"refb1x", "refb2x"}, {"refb1y", "refb2y"}, {"refb1n", "refb2n"},
		{"a1n", "a1rn"}, {"a1p", "a1rp"}, {"a1q", "a1rq"}, {"sev1", "sev2"},
	}
	key := func(gs []gotIssue) []string {
		out := make([]string, len(gs))
		for i, g := range gs {
			out[i] = g.String()
		}
		sort.Strings(out)
		return out
	}
	for _, version := range layerVersions {
		v := layerValidator(t, version)
		for _, p := range pairs {
			t.Run(version+"/"+p[0], func(t *testing.T) {
				a, b := key(layerRun(t, v, p[0])), key(layerRun(t, v, p[1]))
				if !slices.Equal(a, b) {
					t.Errorf("%s da %v y %s da %v", p[0], a, p[1], b)
				}
			})
		}
	}
}

// A5: el mismo canónico (resuelto o no) en dos nodos se informa por nodo.
func TestLayersSameCanonicalOnTwoNodes(t *testing.T) {
	runLayerProbes(t, []string{"two", "twom", "twom1"})
}

// B2: una respuesta provisional que se invalida entre fases, en un perfil recursivo (rec*c) y en un ciclo de
// referencias entre dos Organization (bcyc*). Se comprueban las capas elegidas y los diagnósticos finales.
// En bcyc3 HL7 no informa el destino de la referencia del Identifier; el valor esperado es el de la especificación.
func TestLayersInvalidationBetweenPhases(t *testing.T) {
	runLayerProbes(t, []string{"rec1c", "rec2c", "bcyc1", "bcyc2", "bcyc3"})
}

// D1: el recurso dueño del valor da su `%resource`, en cada llamador (a) con alternativas que dependen de él:
// cardinalidad (ocard), binding (obind), extensión (oext) y slicing (oslice), y en la constraint (res1 a res5, un
// recurso contenido y una entrada de Bundle). En X1 el contenido cumple una sola alternativa y el raíz ninguna; en
// X2 al revés.
func TestLayersOwnerPerCaller(t *testing.T) {
	runLayerProbes(t, expand(t, `o(card|bind|ext|slice)[12]|res[1-5]`))
}

// D2: cada grupo de holders se exige por separado, en los dos órdenes de las capas; un destino que cumple solo
// uno no da error y uno que no cumple ninguno da un informe por capa.
func TestLayersHolderGroupsAreRequiredSeparately(t *testing.T) {
	runLayerProbes(t, expand(t, `ref[012][nxy]|refb[12][nxy]`))
}

// D4: sin perfiles usables hay una sola capa, la del tipo, y su contenido base se informa una vez. Sonda un17 (ronda 2, 17a): con
// dos perfiles de Annotation que no existen en `Observation.note` y una nota sin `text`, el contenido base del tipo sigue
// (CARDINALITY_MIN en `note[0].text`) y cada perfil se informa una vez (HL7: el mínimo y un Unknown_Profile por perfil).
func TestLayersNoUsableProfile(t *testing.T) {
	runLayerProbes(t, []string{"d4a", "d4b", "d4c", "un17"})
}

// D6/E4: un canónico de ImplementationGuide, o de un StructureDefinition contenido, en `type.profile` no se
// resuelve, y cuenta como no cumplido. Es una pérdida conocida: HL7 hace lo mismo (Unknown_Profile), y esta
// prueba la documenta; si un día se admite, el valor de HL7 de estas sondas cambia.
func TestLayersImplementationGuideAndContainedProfileAreKnownLosses(t *testing.T) {
	runLayerProbes(t, expand(t, `ig[123][cnu]|ct[12][cnu]`))
}

// E2 (decisión del 14-11-2026): el discriminador por perfil evalúa un tipo de datos con el Scope canónico de su
// recurso dueño; con type.profile simple (e2t*) HL7 ya lo hace. Las filas e2s* y e2h* son la divergencia declarada.
func TestLayersDatatypeScopeInDiscriminators(t *testing.T) {
	runLayerProbes(t, expand(t, `e2(t|s|hn|h)r.+`))
}

// E3/F1: invariantes de la misma clave en las dos capas: igual texto y severidad, un solo informe; texto
// distinto, el de la primera capa declarada; severidad distinta, los dos; ele-1 heredado por las dos capas, uno.
// Las dos órdenes de candidatos, y con un solo perfil la salida no cambia (single*).
func TestLayersReportIdentityAcrossLayers(t *testing.T) {
	runLayerProbes(t, []string{"dup", "sev1", "sev2", "txt1", "txt2", "ele1", "ele2", "single1", "single2", "single3"})
}

// Las divergencias declaradas en hl7diff, con su valor de la especificación.
func TestLayersDeclaredDivergences(t *testing.T) {
	runLayerProbes(t, []string{"p12", "p20", "p21"})
}

// Los casos de control de los llamadores: el mismo caso con la regla de la segunda capa cumplida (solo esa capa cumple y
// no hay informe) o con un valor válido.
func TestLayersCallerControls(t *testing.T) {
	runLayerProbes(t, []string{"kbindok", "kextok", "kslicep", "kucumb"})
}

// Huecos del oráculo de las mutaciones (14-11-2026, probe-gaps): lo que solo una capa del valor tiene. Un valor Timing con
// dos perfiles, cada uno con un tipo distinto para `bounds[x]`, que no cumple ninguno: se camina por las dos capas, y la
// extensión de su `boundsRange` (solo de la segunda) se comprueba en el contexto (gtctx) y en el valor (gtval); con una
// extensión válida (gtok) no se informa nada de ella.
func TestLayersExtensionsOnAValueOnlyTheSecondLayerHas(t *testing.T) {
	runLayerProbes(t, []string{"gtctx", "gtval", "gtok"})
}

// El %resource del dueño decide entre los perfiles de tipo en cada llamador del slicing (14-11-2026, probe-gaps): el
// slice que declara los perfiles (s13a), un hijo de un miembro de slice que los declara (s13b) y el elemento que el slice
// divide, cuando el slice no declara ninguno (s10); en un recurso contenido (c) y en la entrada de un Bundle (e), con el
// estado «amended» en uno y «final» en el otro (1 y 2).
func TestLayersOwnerInSliceMembers(t *testing.T) {
	runLayerProbes(t, expand(t, `s(13a|13b|10)[ce][12]`))
}

// Dos alternativas de Identifier que declaran, para `assigner`, perfiles de Reference distintos y que no existen
// (revisión, ronda 1, probe-gaps/unk): el valor se camina una vez bajo la unión de las capas, y aun así se informa
// cada perfil que no se resuelve, como HL7 (un Unknown_Profile por perfil). ElementDefinition.type.profile
// (elementdefinition.html): cada perfil declarado es uno al que el valor puede ajustarse.
func TestLayersUnresolvedProfilesOfEveryLayerAreReported(t *testing.T) {
	runLayerProbes(t, []string{"unk"})
}

// El slicing corre en frío (revisión, ronda 1): sin que otra fase haya llenado antes el memo de conformidad, que es lo que
// tapa un dueño perdido en el slicing (las mutaciones 13a a 13c). El %resource del dueño decide entre los perfiles de tipo
// (fhirpath.html#variables: «%resource: the resource that contains the original node»), así que en el contenido, que es
// «final», el perfil «amended» (cc-od, que exige `text`) no es el que le toca: `text` se exige solo del recurso de arriba,
// que es «amended» (HL7 lo informa solo ahí, probe-gaps). El control es s10c1, donde el slice no declara perfiles.
//
// Ronda 2: las mismas formas más hondas (dpin, dppr, dpwk; el recurso de arriba «amended», el contenido «final»), donde
// el dueño se pierde en la propagación de `slicing.go` (líneas ~202 y ~221) o en `walkChildren` de `tree.go`: las
// alternativas de Coding en línea (dpin), en el perfil de CodeableConcept que las declara (dppr), y dos perfiles de
// CodeableConcept con un slice `coding:c` obligatorio de código distinto cada uno (dpwk). Valor esperado: el de la
// especificación y la decisión E2 del plan (§5): el %resource de un tipo de datos es el recurso que lo contiene (en el
// contenido, el contenido), así que el slicing no informa nada bajo `contained[0]`. HL7 no informa nada en estas tres, en R4,
// R4B y R5 (probe-gaps, README de probe, ronda 2), así que aquí no hay divergencia.
func TestLayersSlicingColdHasTheOwner(t *testing.T) {
	for _, version := range layerVersions {
		v := layerValidator(t, version)
		for _, name := range []string{"s13ac1", "s13bc1", "s10c1", "dpinc1", "dpprc1", "dpwkc1"} {
			t.Run(version+"/"+name, func(t *testing.T) {
				var data map[string]any
				if err := json.Unmarshal(layerInstance(t, name), &data); err != nil {
					t.Fatal(err)
				}
				ctx := scopeindex.With(context.Background(), v.buildPositions(data))
				ctx = withConformState(withRootScope(ctx, data, nil))
				ctx = registry.WithProfileChooser(ctx, typeChooser{v: v})
				// el perfil lo declara la instancia de arriba (s*) o solo el contenido (dp*, que corre sobre el tipo base)
				sd := v.registry.GetByType("Observation")
				meta, _ := data["meta"].(map[string]any)
				if profs, ok := meta["profile"].([]any); ok {
					sd, _ = v.registry.ResolveCanonical(profs[0].(string))
				}
				if err := v.registry.EnsureSnapshot(ctx, sd); err != nil {
					t.Fatal(err)
				}
				r := issue.NewResult()
				v.slicingValidator.ValidateDataContext(ctx, data, sd, slicing.Options{}, r)
				deep := strings.HasPrefix(name, "dp")
				for _, is := range r.Issues {
					if len(is.Expression) == 0 || !strings.Contains(is.Expression[0], "contained[0]") {
						continue
					}
					if deep || strings.HasSuffix(is.Expression[0], ".text") {
						t.Errorf("el slicing exige %s del contenido por un perfil que no es el suyo: %s %s", is.Expression[0], is.MessageID, is.Diagnostics)
					}
				}
			})
		}
	}
}

// El invariante «un map, un Scope» (TestScopeInvariantOnTheProbes) vale también sobre las sondas de esta pieza,
// en R4, R4B y R5: todo Scope con que llega una consulta de conformidad es el del índice de posiciones, también
// para el recurso dueño de un tipo de datos con capas.
func TestScopeInvariantOnTheTypeProfileLayerProbes(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(layersDir, "inst", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("instancias: %v", err)
	}
	checks := int64(0)
	for _, version := range layerVersions {
		v := layerValidator(t, version)
		for _, f := range files {
			body, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			ctx, n := countedCtx(t)
			if _, err := v.Validate(ctx, body); err != nil {
				t.Fatalf("%s %s: %v", version, f, err)
			}
			checks += n.Load()
		}
	}
	if checks == 0 {
		t.Fatal("ninguna consulta de conformidad pasó por el invariante: la prueba no comprueba nada")
	}
}

// probedCtx devuelve un contexto que cuenta, en visits, las elecciones entre perfiles de tipo que se
// piden ("visit") y, en checks, las comprobaciones de conformidad que corren de verdad ("check").
func probedCtx(visits, checks *atomic.Int64) context.Context {
	return context.WithValue(context.Background(), probeKey{}, conformProbe(func(event string) {
		switch event {
		case "visit":
			visits.Add(1)
		case "check":
			checks.Add(1)
		}
	}))
}

// scaleProfiles son una Observation cuyo component.value[x] declara k perfiles de Quantity, cada uno con una
// invariante propia que el valor del bench no cumple (ninguno cumple, así que se caminan todos), y los k perfiles.
func scaleProfiles(k int) (url string, defs [][]byte) {
	base := "http://example.org/fhir/StructureDefinition/"
	profiles := make([]string, k)
	for i := range k {
		id := fmt.Sprintf("qscale%d", i)
		profiles[i] = base + id
		raw, _ := json.Marshal(map[string]any{
			"resourceType": "StructureDefinition", "id": id, "url": base + id, "name": id, "status": "draft",
			"fhirVersion": "4.0.1", "kind": "complex-type", "abstract": false, "type": "Quantity",
			"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Quantity", "derivation": "constraint",
			"differential": map[string]any{"element": []any{map[string]any{
				"id": "Quantity", "path": "Quantity",
				"constraint": []any{map[string]any{"key": fmt.Sprintf("qs%d-1", i), "severity": "error", "human": "too big", "expression": fmt.Sprintf("value > %d", 100000+i)}},
			}}},
		})
		defs = append(defs, raw)
	}
	obs, _ := json.Marshal(map[string]any{
		"resourceType": "StructureDefinition", "id": "obs-scale", "url": base + "obs-scale", "name": "obs_scale", "status": "draft",
		"fhirVersion": "4.0.1", "kind": "resource", "abstract": false, "type": "Observation",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Observation", "derivation": "constraint",
		"differential": map[string]any{"element": []any{map[string]any{
			"id": "Observation.component.value[x]", "path": "Observation.component.value[x]",
			"type": []any{map[string]any{"code": "Quantity", "profile": profiles}},
		}}},
	})
	return base + "obs-scale", append(defs, obs)
}

// scaleObservation es una Observation con n elementos component de valor Quantity, bajo el perfil profile.
func scaleObservation(b testing.TB, profile string, n int) []byte {
	b.Helper()
	comps := make([]any, n)
	for i := range comps {
		comps[i] = map[string]any{"code": map[string]any{"text": "c"}, "valueQuantity": map[string]any{"value": 5, "system": "http://unitsofmeasure.org"}}
	}
	doc, err := json.Marshal(map[string]any{
		"resourceType": "Observation", "id": "scale", "status": "final", "code": map[string]any{"text": "x"},
		"meta":      map[string]any{"profile": []string{profile}},
		"component": comps,
	})
	if err != nil {
		b.Fatal(err)
	}
	return doc
}

// scaleChain es una Observation cuyo identifier[0] anida depth Identifier por assigner.identifier, bajo obs-rec1
// (idr-a se declara a sí mismo y a idr-b en cada nivel).
func scaleChain(b testing.TB, depth int) []byte {
	b.Helper()
	return scaleChainUnder(b, "http://example.org/fhir/StructureDefinition/obs-rec1", depth)
}

// scaleChainUnder es scaleChain bajo el perfil de Observation profile.
func scaleChainUnder(b testing.TB, profile string, depth int) []byte {
	b.Helper()
	var ident map[string]any
	for range depth {
		next := map[string]any{"value": "x"}
		if ident != nil {
			next["assigner"] = map[string]any{"display": "d", "identifier": ident}
		}
		ident = next
	}
	doc, err := json.Marshal(map[string]any{
		"resourceType": "Observation", "id": "chain", "status": "final", "code": map[string]any{"text": "x"},
		"meta":       map[string]any{"profile": []string{profile}},
		"identifier": []any{ident},
	})
	if err != nil {
		b.Fatal(err)
	}
	return doc
}

// scaleBothRecurse son dos perfiles de Identifier que declaran, los dos, a los dos como perfiles de
// assigner.identifier (cada nivel tiene dos capas y las dos bajan), y una Observation que los declara en identifier.
// Es el peor caso para el recorrido por capas: si repitiera el subárbol bajo cada capa sería de orden 2^D en la
// profundidad.
func scaleBothRecurse() (url string, defs [][]byte) {
	base := "http://example.org/fhir/StructureDefinition/"
	both := []string{base + "idc", base + "idd"}
	for _, id := range []string{"idc", "idd"} {
		raw, _ := json.Marshal(map[string]any{
			"resourceType": "StructureDefinition", "id": id, "url": base + id, "name": id, "status": "draft",
			"fhirVersion": "4.0.1", "kind": "complex-type", "abstract": false, "type": "Identifier",
			"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Identifier", "derivation": "constraint",
			"differential": map[string]any{"element": []any{map[string]any{
				"id": "Identifier.assigner.identifier", "path": "Identifier.assigner.identifier",
				"type": []any{map[string]any{"code": "Identifier", "profile": both}},
			}}},
		})
		defs = append(defs, raw)
	}
	obs, _ := json.Marshal(map[string]any{
		"resourceType": "StructureDefinition", "id": "obs-both", "url": base + "obs-both", "name": "obs_both", "status": "draft",
		"fhirVersion": "4.0.1", "kind": "resource", "abstract": false, "type": "Observation",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Observation", "derivation": "constraint",
		"differential": map[string]any{"element": []any{map[string]any{
			"id": "Observation.identifier", "path": "Observation.identifier",
			"type": []any{map[string]any{"code": "Identifier", "profile": both}},
		}}},
	})
	return base + "obs-both", append(defs, obs)
}

// runScale valida doc b.N veces con v y da, por iteración, las elecciones pedidas (visits/op) y las
// comprobaciones de conformidad que corrieron (checks/op).
func runScale(b *testing.B, v *Validator, doc []byte) {
	b.Helper()
	var visits, checks atomic.Int64
	ctx := probedCtx(&visits, &checks)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.Validate(ctx, doc); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(visits.Load())/float64(b.N), "visits/op")
	b.ReportMetric(float64(checks.Load())/float64(b.N), "checks/op")
}

// BenchmarkTypeProfileLayersScale mide el costo del recorrido por capas (D5, plan §3d): debe ser lineal en las
// capas elegidas por valor, no en las capas superpuestas ni en la profundidad de los descendientes que se repiten.
// Informa, además del tiempo y las asignaciones, visits/op (elecciones entre perfiles pedidas) y checks/op
// (comprobaciones de conformidad que corren de verdad, sin las respuestas de memo), en tres ejes:
//   - components=N: N valores con dos alternativas (repeticiones); checks/op debe ser 2N;
//   - alternatives=K: diez valores con K alternativas que ninguna cumple (se caminan todas); checks/op debe ser 10K;
//   - depth=D: D Identifier anidados, cada uno con dos perfiles (idr-a se declara a sí mismo); checks/op debe ser 2D.
func BenchmarkTypeProfileLayersScale(b *testing.B) {
	v := layerValidator(b, "4.0.1")
	for _, n := range []int{1, 10, 100} {
		doc := scaleObservation(b, "http://example.org/fhir/StructureDefinition/obs-two", n)
		b.Run(fmt.Sprintf("components=%d", n), func(b *testing.B) { runScale(b, v, doc) })
	}
	for _, k := range []int{2, 4, 8} {
		url, defs := scaleProfiles(k)
		sv, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
		if err != nil {
			b.Fatal(err)
		}
		doc := scaleObservation(b, url, 10)
		b.Run(fmt.Sprintf("alternatives=%d", k), func(b *testing.B) { runScale(b, sv, doc) })
	}
	for _, d := range []int{1, 2, 4, 8} {
		doc := scaleChain(b, d)
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) { runScale(b, v, doc) })
	}
	// Las dos capas de cada nivel cumplen y bajan las dos: todas las capas elegidas en cada nivel.
	url, defs := scaleBothRecurse()
	bv, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
	if err != nil {
		b.Fatal(err)
	}
	for _, d := range []int{1, 2, 4, 8, 12} {
		doc := scaleChainUnder(b, url, d)
		b.Run(fmt.Sprintf("depth-all-layers=%d", d), func(b *testing.B) { runScale(b, bv, doc) })
	}
}

// Un valor que dos capas de su padre declaran se camina una vez, bajo la unión de las capas (plan §2 v8): con
// todas las capas elegidas en cada nivel el costo no crece como 2^D. Las visitas al elector crecen como el
// cuadrado de la profundidad (cada comprobación de un nivel camina el subárbol de debajo; las comprobaciones
// siguen siendo 2D por la memoización), no como 2^D (130800 visitas en D = 12 antes de v8, 2696 después).
func TestLayersAllLayersChosenAtEveryLevelIsPolynomial(t *testing.T) {
	url, defs := scaleBothRecurse()
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	visitsAt := map[int]int64{}
	for _, d := range []int{2, 4, 8, 12} {
		var visits, checks atomic.Int64
		if _, err := v.Validate(probedCtx(&visits, &checks), scaleChainUnder(t, url, d)); err != nil {
			t.Fatal(err)
		}
		visitsAt[d] = visits.Load()
		if checks.Load() != int64(2*d) {
			t.Errorf("D=%d: %d comprobaciones de conformidad, se esperaban %d (dos por nivel)", d, checks.Load(), 2*d)
		}
		if bound := int64(24 * d * d); visits.Load() > bound {
			t.Errorf("D=%d: %d visitas, más que 24·D² = %d", d, visits.Load(), bound)
		}
	}
	// Con 2^D, pasar de D = 8 a D = 12 multiplicaría las visitas por 16; con D² por 2,25.
	if ratio := float64(visitsAt[12]) / float64(visitsAt[8]); ratio > 4 {
		t.Errorf("las visitas de D = 12 son %.1f veces las de D = 8 (%d y %d); con costo polinomial de grado 2 serían 2,25", ratio, visitsAt[12], visitsAt[8])
	}
}

// scaleSameChild son dos perfiles de Identifier que declaran el mismo hijo, assigner, con reglas distintas
// (ida-1 pide un display que empiece por «x», idb-1 por «y») y los dos a ambos como perfiles de
// assigner.identifier, y una Observation que los declara en identifier.
func scaleSameChild() (url string, defs [][]byte) {
	base := "http://example.org/fhir/StructureDefinition/"
	both := []string{base + "ida", base + "idb"}
	rule := map[string]string{"ida": "x", "idb": "y"}
	for _, id := range []string{"ida", "idb"} {
		raw, _ := json.Marshal(map[string]any{
			"resourceType": "StructureDefinition", "id": id, "url": base + id, "name": id, "status": "draft",
			"fhirVersion": "4.0.1", "kind": "complex-type", "abstract": false, "type": "Identifier",
			"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Identifier", "derivation": "constraint",
			"differential": map[string]any{"element": []any{
				map[string]any{
					"id": "Identifier.assigner", "path": "Identifier.assigner",
					"constraint": []any{map[string]any{
						"key": id + "-1", "severity": "error", "human": "display starts with " + rule[id],
						"expression": "display.startsWith('" + rule[id] + "')",
					}},
				},
				map[string]any{
					"id": "Identifier.assigner.identifier", "path": "Identifier.assigner.identifier",
					"type": []any{map[string]any{"code": "Identifier", "profile": both}},
				},
			}},
		})
		defs = append(defs, raw)
	}
	obs, _ := json.Marshal(map[string]any{
		"resourceType": "StructureDefinition", "id": "obs-same", "url": base + "obs-same", "name": "obs_same", "status": "draft",
		"fhirVersion": "4.0.1", "kind": "resource", "abstract": false, "type": "Observation",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Observation", "derivation": "constraint",
		"differential": map[string]any{"element": []any{map[string]any{
			"id": "Observation.identifier", "path": "Observation.identifier",
			"type": []any{map[string]any{"code": "Identifier", "profile": both}},
		}}},
	})
	return base + "obs-same", append(defs, obs)
}

// Dos capas que declaran el mismo hijo con reglas distintas que el valor no cumple informan las dos reglas, y el
// valor hijo se camina una vez: el nieto, que las dos capas declaran con las mismas alternativas, se elige
// una vez por fase y no una vez por capa.
func TestLayersSameChildWithDifferentRulesIsReportedByBothAndWalkedOnce(t *testing.T) {
	url, defs := scaleSameChild()
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	// Seis niveles de Identifier, cada assigner con un display que ninguna de las dos reglas acepta («z»).
	level := func(inner map[string]any) map[string]any {
		assigner := map[string]any{"display": "z"}
		if inner != nil {
			assigner["identifier"] = inner
		}
		return map[string]any{"value": "v", "assigner": assigner}
	}
	doc, err := json.Marshal(map[string]any{
		"resourceType": "Observation", "id": "same", "status": "final", "code": map[string]any{"text": "x"},
		"meta":       map[string]any{"profile": []string{url}},
		"identifier": []any{level(level(level(level(level(level(nil))))))},
	})
	if err != nil {
		t.Fatal(err)
	}
	var visits, checks atomic.Int64
	res, err := v.Validate(probedCtx(&visits, &checks), doc)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range res.Issues {
		if g, ok := classifyLayers(is); ok && g.Kind == "constraint" {
			got = append(got, g.Detail+"@"+g.Location)
		}
	}
	// Las dos reglas, en cada assigner que los perfiles gobiernan; ninguna se repite.
	sort.Strings(got)
	for _, rule := range []string{"ida-1", "idb-1"} {
		if !slices.ContainsFunc(got, func(g string) bool { return strings.HasPrefix(g, rule+"@Observation.identifier[0].assigner") }) {
			t.Errorf("falta la regla %s en el primer assigner: %v", rule, got)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i] == got[i-1] {
			t.Errorf("la misma regla se informa dos veces: %s", got[i])
		}
	}
	// Una elección por valor y fase, no por capa: con seis niveles el recorrido por capa da 1896 visitas y el de
	// v8 632 (el tope deja margen y falla con el recorrido por capa).
	if bound := int64(900); visits.Load() > bound {
		t.Errorf("%d visitas al elector, más que %d: un valor se camina más de una vez", visits.Load(), bound)
	}
	t.Logf("visitas: %d, comprobaciones: %d, reglas: %v", visits.Load(), checks.Load(), got)
}

// Las sondas de la pieza 3 siguen en el corpus (su evidencia y su valor esperado), aunque esta pieza no las corra.
func TestLayersPiece3ProbesStayInTheCorpus(t *testing.T) {
	probes := layerProbes(t)
	for _, name := range piece3Probes {
		if _, ok := probes[name]; !ok {
			t.Errorf("la sonda %s de la pieza 3 no está en expected.json", name)
		}
	}
}
