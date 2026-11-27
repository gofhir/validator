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
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/validator/v2/internal/walkprobe"
	"github.com/gofhir/validator/v2/pkg/issue"
)

// Descenso por todas las definiciones que gobiernan un valor (resto de #193; plan
// docs/plans/2026-11-18-definition-layers-descent.md, §2 v4).
//
// Bajo un valor, la fase de constraints aplica en sus descendientes todas las definiciones que lo gobiernan: la que da
// los hijos, los elementos de su cadena SliceOf, el perfil de su tipo (o los elegidos), su definición propia (la
// extensión que nombra url) y, en un primitivo, los hijos que el perfil despliega para su `_x`. R4 profiling.html, de un
// perfil que restringe un elemento dividido sin sliceName: «the profile is adding constraints to all slices of X»; R4
// elementdefinition-definitions.html#ElementDefinition.type.profile: «If any profiles are specified, then the content must
// conform to at least one of them».
//
// Los valores esperados salen de HL7 validator_cli 7.0.4 (-tx n/a), no de gofhir: testdata/definition_layers/expected.json
// lo genera docs/plans/evidence/2026-11-18-definition-layers-descent/tests/gen_expected.py desde las salidas de
// run_hl7.sh (seis fuentes de sondas: la matriz de gen.py y las de las rondas del plan), en R4, R4B y R5. Los perfiles
// y las instancias son los de R4; el test ajusta fhirVersion por versión (en cada fuente solo cambia ese campo). Cada
// fuente tiene su validador: algunas comparten la URL base.
//
// Se comparan veredicto, severidad, ubicación y clase de informe de todos los errores y todas las advertencias (criterio
// de salida 1). Las excepciones declaradas, cada una con su comentario en gen_expected.py:
//   - un mismo hecho que dos definiciones dicen del mismo valor se informa una vez (HL7 lo repite);
//   - matrix/m-d2-*: HL7 pierde en el snapshot del perfil derivado los hijos del elemento dividido que trae la base; el
//     valor esperado es el del control m-sid-* (D2);
//   - x2/bad-r-fix: las reglas del slice a valen también en su reslice a/b (inferencia, declarada);
//   - p/m6-keys, p/m7-keys: dos claves con la misma expresión son dos constraints (HL7 calla la segunda);
//   - los avisos de buenas prácticas de Observation de HL7 no salen de ninguna StructureDefinition (regla 5).
// Fuera de la matriz: binding y cardinalidad (pieza siguiente) y bad-g4 (misma clave con otra expresión, que HL7
// rechaza al generar el snapshot en R5; plan §2, «Fuera del alcance»).

const descentDir = "testdata/definition_layers"

// descentIssue es un informe esperado: el de la pieza 2 más el perfil que nombra «(defined in …)» (plan §2, punto 8).
type descentIssue struct {
	layerIssue
	Source string `json:"source"`
}

func (d descentIssue) String() string {
	if d.Source == "" {
		return d.layerIssue.String()
	}
	return d.layerIssue.String() + " (defined in " + d.Source + ")"
}

type descentProbe struct {
	Description string                    `json:"description"`
	Note        string                    `json:"note"`
	Expected    map[string][]descentIssue `json:"expected"`
}

var (
	descentProbesOnce sync.Once
	descentProbesVal  map[string]descentProbe
	descentValsMu     sync.Mutex
	descentVals       = map[string]*Validator{}
)

func descentProbes(t testing.TB) map[string]descentProbe {
	t.Helper()
	descentProbesOnce.Do(func() {
		raw, err := os.ReadFile(filepath.Join(descentDir, "expected.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &descentProbesVal); err != nil {
			t.Fatal(err)
		}
	})
	return descentProbesVal
}

var fhirVersionField = []byte(`"fhirVersion": "4.0.1"`)

// descentDefinitions lee los perfiles de una fuente, con la versión de FHIR de version, y les aplica edit (si no es nil).
func descentDefinitions(t testing.TB, source, version string, edit func(name string, raw []byte) []byte) [][]byte {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(descentDir, source, "ig", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("perfiles de %s: %v", source, err)
	}
	sort.Strings(files)
	out := make([][]byte, 0, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(raw, fhirVersionField) {
			t.Fatalf("%s no declara %s", f, fhirVersionField)
		}
		raw = bytes.Replace(raw, fhirVersionField, []byte(`"fhirVersion": "`+version+`"`), 1)
		if edit != nil {
			raw = edit(strings.TrimSuffix(filepath.Base(f), ".json"), raw)
		}
		out = append(out, raw)
	}
	return out
}

// descentValidator es el validador compartido de una fuente y una versión.
func descentValidator(t testing.TB, source, version string) *Validator {
	t.Helper()
	descentValsMu.Lock()
	defer descentValsMu.Unlock()
	k := source + "@" + version
	if v, ok := descentVals[k]; ok {
		return v
	}
	v, err := New(WithVersion(version), WithConformanceResources(descentDefinitions(t, source, version, nil)))
	if err != nil {
		t.Fatal(err)
	}
	descentVals[k] = v
	return v
}

// descentInstance lee la instancia <fuente>/<nombre>.
func descentInstance(t testing.TB, name string) []byte {
	t.Helper()
	source, file, _ := strings.Cut(name, "/")
	body, err := os.ReadFile(filepath.Join(descentDir, source, "inst", file+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

var (
	descentFixed = regexp.MustCompile(`but the profile requires (.*)$`)
	descentTypes = regexp.MustCompile(`allows the type\(s\) (.*) but found type`)
)

// typeSet normaliza una lista de tipos («[string, boolean]», «'string', 'boolean'») a «boolean,string».
func typeSet(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune("[], '\"", r) })
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// classifyDescent da la clase de un informe de gofhir como gen_expected.py da la de HL7: todas las constraints (de
// cualquier severidad), fixed y pattern con el valor exigido, tipo no permitido con los tipos permitidos, y las demás
// clases de la pieza 2. Ninguna advertencia se deja afuera (criterio de salida 1).
func classifyDescent(is issue.Issue) (gotIssue, bool) {
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
	switch id := is.MessageID; id {
	case string(issue.DiagConstraintFailed):
		m := layerConstraint.FindStringSubmatch(is.Diagnostics)
		if m == nil {
			g.Kind = "other:" + id
			return g, true
		}
		g.Kind, g.Detail, g.Text = "constraint", m[1], m[2]
	case string(issue.DiagFixedValueMismatch):
		g.Kind = "fixed"
		if m := descentFixed.FindStringSubmatch(is.Diagnostics); m != nil {
			g.Detail = strings.Trim(m[1], `'"`)
		}
	case string(issue.DiagStructureTypeNotAllowed):
		g.Kind = "type-not-allowed"
		if m := descentTypes.FindStringSubmatch(is.Diagnostics); m != nil {
			g.Detail = typeSet(m[1])
		}
	case string(issue.DiagSlicingMultipleMatch):
		g.Kind = "slice-match-multiple"
	default:
		l, ok := classifyLayers(is)
		if ok {
			return l, true
		}
		g.Kind = "other:" + id
	}
	return g, true
}

// matchDescent dice si el informe de gofhir es el esperado.
func matchDescent(exp descentIssue, got gotIssue) bool {
	switch exp.Kind {
	case "fixed":
		return exp.Kind == got.Kind && exp.Severity == got.Severity && exp.Location == got.Location && exp.Detail == got.Detail
	case "type-not-allowed":
		// HL7 lo ubica en el elemento (la extensión) y gofhir en su valor de elección: «Patient.extension[0]» y
		// «Patient.extension[0].valueBoolean» son el mismo informe (ubicación normalizada, criterio de salida 1).
		at := got.Location == exp.Location ||
			(strings.HasPrefix(got.Location, exp.Location+".value") && !strings.Contains(got.Location[len(exp.Location)+1:], "."))
		return exp.Kind == got.Kind && exp.Severity == got.Severity && at && exp.Detail == got.Detail
	case "constraint":
		if exp.Source != "" && !strings.Contains(got.Diagnostics, "(defined in "+exp.Source+")") {
			return false
		}
	}
	return matchLayer(exp.layerIssue, got)
}

func compareDescent(expected []descentIssue, got []gotIssue) (missing []descentIssue, extra []gotIssue) {
	left := slices.Clone(got)
	for _, e := range expected {
		i := slices.IndexFunc(left, func(g gotIssue) bool { return matchDescent(e, g) })
		if i < 0 {
			missing = append(missing, e)
			continue
		}
		left = slices.Delete(left, i, i+1)
	}
	return missing, left
}

// descentRunBody valida body con v y da los informes clasificados.
func descentRunBody(ctx context.Context, t testing.TB, v *Validator, name string, body []byte) []gotIssue {
	t.Helper()
	res, err := v.Validate(ctx, body)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out []gotIssue
	for _, is := range res.Issues {
		if g, ok := classifyDescent(is); ok {
			out = append(out, g)
		}
	}
	return out
}

func checkDescent(t *testing.T, version, name string, got []gotIssue) {
	t.Helper()
	probe, ok := descentProbes(t)[name]
	if !ok {
		t.Fatalf("la sonda %s no está en expected.json", name)
	}
	missing, extra := compareDescent(probe.Expected[version], got)
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("%s %s\nHL7 7.0.4 da %v\nfaltan %v\nde más %v", probe.Description, probe.Note, probe.Expected[version], missing, extraStrings(extra))
	}
}

// runDescent compara a gofhir con HL7 en cada instancia de names, en R4, R4B y R5.
func runDescent(t *testing.T, names []string) {
	t.Helper()
	for _, version := range layerVersions {
		for _, name := range names {
			source, _, _ := strings.Cut(name, "/")
			v := descentValidator(t, source, version)
			t.Run(version+"/"+name, func(t *testing.T) {
				checkDescent(t, version, name, descentRunBody(context.Background(), t, v, name, descentInstance(t, name)))
			})
		}
	}
}

// descentExpand da los nombres de las sondas que coinciden por entero con re, y exige al menos min.
func descentExpand(t testing.TB, re string, least int) []string {
	t.Helper()
	rx := regexp.MustCompile("^(?:" + re + ")$")
	var out []string
	for n := range descentProbes(t) {
		if rx.MatchString(n) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	if len(out) < least {
		t.Fatalf("%q: %d sondas, se esperaban al menos %d", re, len(out), least)
	}
	return out
}

// --- La matriz: portador × regla × violación (gen.py). Cada regla (invariante, fixed, pattern, perfil de tipo de un
// hijo, slice de un hijo, tipos permitidos, la misma invariante con dos severidades) está en las dos definiciones, y
// se viola en ninguna, en una, en la otra o en las dos.

// Forma S: el elemento dividido y su slice, en Patient.identifier y en Patient.extension.
func TestDescentSlicedElementAndSlice(t *testing.T) {
	runDescent(t, descentExpand(t, `matrix/m-(sid|sext)-.+`, 48))
}

// Forma D: un perfil derivado restringe el elemento dividido de un slice que viene de la base (d1), o redeclara el
// slice de una base que restringe el elemento dividido (d2, divergencia D2).
func TestDescentDerivedProfile(t *testing.T) {
	runDescent(t, descentExpand(t, `matrix/m-d[12]-.+`, 44))
}

// Forma T2: el perfil de tipo y los hijos que el perfil de la instancia despliega, en un Identifier y en una extensión.
func TestDescentTypeProfileAndUnrolledChildren(t *testing.T) {
	runDescent(t, descentExpand(t, `matrix/m-(t2|t2ext)-.+`, 40))
}

// Forma P: el slice de la extensión de un primitivo (Patient.birthDate), y el `id` y la invariante de la extensión de un
// primitivo.
func TestDescentPrimitiveExtensionSlice(t *testing.T) {
	runDescent(t, descentExpand(t, `matrix/m-pext-.+|probe/(ok|bad)-p-(fixed|inv)|x2/bad-p-(id|einv)`, 26))
}

// Forma O: la definición propia de una extensión (la que nombra url) junto con el elemento del perfil que la contiene.
func TestDescentOwnDefinition(t *testing.T) {
	runDescent(t, descentExpand(t, `matrix/m-own-.+|x2/(ok|bad)-o(-.+)?|x2/bad-p-own-fix`, 26))
}

// Forma H: el perfil de un recurso sostenido declarado en el elemento base y en su slice (Bundle.entry.resource,
// Patient.contained), cada par (recurso, perfil) una vez.
func TestDescentHeldResource(t *testing.T) {
	runDescent(t, descentExpand(t, `matrix/m-held-.+|g/(ok|bad)-g3c?|p/h[1-4]-.+`, 20))
}

// Arreglos (solo algunas repeticiones son del slice) y profundidad 2 a 5 (una extensión dentro de otra).
func TestDescentArraysAndDepth(t *testing.T) {
	runDescent(t, descentExpand(t, `matrix/m-(arr|depth\d)-.+`, 20))
}

// --- Sondas de las rondas del plan.

// Severidades (puntos 8 y 9): la misma invariante como error y como advertencia en la misma ubicación y con el mismo
// texto deja el error; con «(defined in …)» distinto son dos informes. Incluye las del prototipo (m1) y las del clon
// que cede ante la constraint del diferencial del slice (punto 7, s5a a s5e).
func TestDescentSeverities(t *testing.T) {
	runDescent(t, descentExpand(t, `x2/bad-v-.+|p/s5[a-e]-.+|m1/.+|matrix/m-[a-z0-9]+-sev-.+`, 30))
}

// Identidad de invariante (divergencia declarada): dos claves con la misma expresión se informan las dos. Las sondas de
// sombra (m8, m9: la misma clave con otra expresión) salen del alcance: HL7 no genera su snapshot (plan §9).
func TestDescentInvariantIdentity(t *testing.T) {
	runDescent(t, descentExpand(t, `p/m[67]-.+`, 2))
}

// El foco de una invariante (`$this is FHIR.string`) sobre un valor, con y sin perfil de tipo: Identifier.value es un
// FHIR.string (divergencia declarada, plan §9: HL7 evalúa el operador is distinto de la función is()).
func TestDescentTypeProfileInvariantFocus(t *testing.T) {
	runDescent(t, descentExpand(t, `p/m[1-5]-[np]|g/bad-g1`, 11))
}

// Las demás sondas con nombre: formas D y F en hijos profundos, slices anidados, reslicing (divergencia declarada),
// slicing sin discriminador, elección entre perfiles de tipo y las que el plan deja por clasificar (bad-d-type,
// bad-d-type2, bad-f-type, bad-l-none, bad-m-nod, bad-o-alt, bad-q-cr, h4-one; plan §9), con las sondas de control de los
// perfiles cuyo snapshot HL7 no genera (ctl-*, divergencia D2b).
func TestDescentNamedProbes(t *testing.T) {
	runDescent(t, descentExpand(t, `x2/(ok|bad)-[dfhlmnqrs]-.+|probe/(ok|bad)-[st]-(fixed|inv)|x2/ctl-.+|g/ctl-.+`, 28))
}

// Todas las sondas de expected.json están en algún test de arriba.
func TestDescentEveryProbeIsRun(t *testing.T) {
	groups := []string{
		`matrix/m-(sid|sext)-.+`, `matrix/m-d[12]-.+`, `matrix/m-(t2|t2ext)-.+`, `matrix/m-pext-.+|probe/(ok|bad)-p-(fixed|inv)|x2/bad-p-(id|einv)`,
		`matrix/m-own-.+|x2/(ok|bad)-o(-.+)?|x2/bad-p-own-fix`, `matrix/m-held-.+|g/(ok|bad)-g3c?|p/h[1-4]-.+`,
		`matrix/m-(arr|depth\d)-.+`, `x2/bad-v-.+|p/s5[a-e]-.+|m1/.+|matrix/m-[a-z0-9]+-sev-.+`, `p/m[67]-.+`,
		`p/m[1-5]-[np]|g/bad-g1`, `x2/(ok|bad)-[dfhlmnqrs]-.+|probe/(ok|bad)-[st]-(fixed|inv)|x2/ctl-.+|g/ctl-.+`,
	}
	rx := regexp.MustCompile("^(?:" + strings.Join(groups, "|") + ")$")
	for n := range descentProbes(t) {
		if !rx.MatchString(n) {
			t.Errorf("la sonda %s no corre en ningún test", n)
		}
	}
}

// --- Metamórficos.

// Invertir el orden de las claves del JSON en todos los niveles da el mismo resultado, y es el de HL7.
func TestDescentKeyOrderDoesNotMatter(t *testing.T) {
	names := descentExpand(t, `.+`, 300)
	for _, version := range layerVersions {
		t.Run(version, func(t *testing.T) {
			for _, name := range names {
				source, _, _ := strings.Cut(name, "/")
				v := descentValidator(t, source, version)
				body := descentInstance(t, name)
				base := primKey(descentRunBody(context.Background(), t, v, name, body))
				got := descentRunBody(context.Background(), t, v, name, reordered(t, body, "rev"))
				if !slices.Equal(base, primKey(got)) {
					t.Errorf("%s con las claves invertidas da %v y en el orden del archivo %v", name, primKey(got), base)
				}
			}
		})
	}
}

// Una extensión sin reglas en meta (matrix/eany, contexto Element), que ningún perfil de la matriz restringe, no cambia
// el resultado.
func TestDescentUnconstrainedExtensionDoesNotMatter(t *testing.T) {
	names := descentExpand(t, `matrix/.+`, 200)
	for _, version := range layerVersions {
		v := descentValidator(t, "matrix", version)
		t.Run(version, func(t *testing.T) {
			for _, name := range names {
				body := descentInstance(t, name)
				var o map[string]any
				if err := json.Unmarshal(body, &o); err != nil {
					t.Fatal(err)
				}
				meta, _ := o["meta"].(map[string]any)
				meta["extension"] = []any{map[string]any{"url": "http://example.org/dl/eany", "valueString": "m"}}
				extended, _ := json.Marshal(o)
				if got := primKey(descentRunBody(context.Background(), t, v, name, extended)); !slices.Equal(got, primKey(descentRunBody(context.Background(), t, v, name, body))) {
					t.Errorf("%s con una extensión en meta da %v", name, got)
				}
			}
		})
	}
}

// Un slice más, sin otra restricción que su discriminador (Patient.identifier:extra, system urn:extra), que ninguna
// repetición de la instancia cumple, no cambia el resultado de la forma S.
func TestDescentUnmatchedExtraSliceDoesNotMatter(t *testing.T) {
	names := descentExpand(t, `matrix/m-sid-.+`, 20)
	extra := []any{
		map[string]any{"id": "Patient.identifier:extra", "path": "Patient.identifier", "sliceName": "extra"},
		map[string]any{"id": "Patient.identifier:extra.system", "path": "Patient.identifier.system", "fixedUri": "urn:extra"},
	}
	edit := func(name string, raw []byte) []byte {
		if !strings.HasPrefix(name, "sid-") {
			return raw
		}
		var o map[string]any
		if err := json.Unmarshal(raw, &o); err != nil {
			t.Fatal(err)
		}
		d := o["differential"].(map[string]any)
		d["element"] = append(d["element"].([]any), extra...)
		out, _ := json.Marshal(o)
		return out
	}
	for _, version := range layerVersions {
		v, err := New(WithVersion(version), WithConformanceResources(descentDefinitions(t, "matrix", version, edit)))
		if err != nil {
			t.Fatal(err)
		}
		t.Run(version, func(t *testing.T) {
			for _, name := range names {
				checkDescent(t, version, name, descentRunBody(context.Background(), t, v, name, descentInstance(t, name)))
			}
		})
	}
}

// Invertir las repeticiones de los arreglos de m-arr-* da los mismos informes con los índices invertidos.
func TestDescentRepetitionOrderDoesNotMatter(t *testing.T) {
	cases := map[string][]string{ // instancia: el camino del arreglo que se invierte
		"matrix/m-arr-sid":  {"identifier"},
		"matrix/m-arr-sext": {"extension"},
		"matrix/m-arr-pext": {"_birthDate", "extension"},
		"matrix/m-arr-held": {"entry"},
	}
	for _, version := range layerVersions {
		v := descentValidator(t, "matrix", version)
		for name, path := range cases {
			t.Run(version+"/"+name, func(t *testing.T) {
				body := descentInstance(t, name)
				var o map[string]any
				if err := json.Unmarshal(body, &o); err != nil {
					t.Fatal(err)
				}
				holder := o
				for _, k := range path[:len(path)-1] {
					holder = holder[k].(map[string]any)
				}
				arr := holder[path[len(path)-1]].([]any)
				slices.Reverse(arr)
				reversed, _ := json.Marshal(o)
				// en la ubicación, el arreglo es el último paso del camino sin «_» (FHIR lo ubica en el valor)
				field := strings.TrimPrefix(strings.Join(path, "."), "_")
				rx := regexp.MustCompile(regexp.QuoteMeta(field) + `\[(\d+)\]`)
				var got []gotIssue
				for _, g := range descentRunBody(context.Background(), t, v, name, reversed) {
					g.Location = rx.ReplaceAllStringFunc(g.Location, func(s string) string {
						i, _ := strconv.Atoi(rx.FindStringSubmatch(s)[1])
						return fmt.Sprintf("%s[%d]", field, len(arr)-1-i)
					})
					got = append(got, g)
				}
				checkDescent(t, version, name, got)
			})
		}
	}
}

// --- Costo (criterio de salida 4).

// walkCounter escucha el recorrido: cuenta los ámbitos y los descensos, y anota los pares repetidos dentro de un mismo
// recorrido (los especulativos anidados tienen su propio registro).
type walkCounter struct {
	mu       sync.Mutex
	stack    []map[string]bool
	scopes   int
	descents int
	repeated []string
}

func (w *walkCounter) listen(event, detail string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch event {
	case walkprobe.Begin:
		w.stack = append(w.stack, map[string]bool{})
	case walkprobe.End:
		w.stack = w.stack[:len(w.stack)-1]
	case walkprobe.Scope:
		w.scopes++
	case walkprobe.Descent:
		w.descents++
		if len(w.stack) == 0 {
			w.repeated = append(w.repeated, "descent outside a walk: "+detail)
			return
		}
		top := w.stack[len(w.stack)-1]
		if top[detail] {
			w.repeated = append(w.repeated, detail)
		}
		top[detail] = true
	}
}

// Cada par (valor, dueño de los hijos) pasa a lo sumo una vez por recorrido, en todas las sondas, en R4, R4B y R5; y
// el recorrido abre ámbitos (si no, la prueba no comprueba nada).
func TestDescentEachPairIsWalkedOnce(t *testing.T) {
	names := descentExpand(t, `.+`, 300)
	var w walkCounter
	for _, version := range layerVersions {
		for _, name := range names {
			source, _, _ := strings.Cut(name, "/")
			v := descentValidator(t, source, version)
			w.repeated = nil
			descentRunBody(walkprobe.With(context.Background(), w.listen), t, v, name, descentInstance(t, name))
			if len(w.repeated) > 0 {
				t.Errorf("%s %s: pares recorridos más de una vez: %v", version, name, w.repeated)
			}
			if len(w.stack) != 0 {
				t.Errorf("%s %s: %d recorridos sin cerrar", version, name, len(w.stack))
				w.stack = nil
			}
		}
	}
	if w.scopes == 0 || w.descents == 0 {
		t.Fatalf("ámbitos %d, descensos %d: el recorrido no emite eventos y la prueba no comprueba nada", w.scopes, w.descents)
	}
}

// Un valor con una sola definición no abre ámbito: las instancias de Patient.identifier sin meta.profile (solo la
// definición base de cada elemento) recorren sin abrir ninguno.
func TestDescentSingleDefinitionOpensNoScope(t *testing.T) {
	names := descentExpand(t, `matrix/m-(sid|d1|d2|t2)-.+`, 80)
	for _, version := range layerVersions {
		v := descentValidator(t, "matrix", version)
		var w walkCounter
		for _, name := range names {
			var o map[string]any
			if err := json.Unmarshal(descentInstance(t, name), &o); err != nil {
				t.Fatal(err)
			}
			delete(o, "meta")
			body, _ := json.Marshal(o)
			descentRunBody(walkprobe.With(context.Background(), w.listen), t, v, name, body)
		}
		if w.scopes != 0 {
			t.Errorf("%s: %d ámbitos abiertos sin perfiles", version, w.scopes)
		}
		if w.descents == 0 {
			t.Errorf("%s: ningún descenso: la prueba no comprueba nada", version)
		}
	}
}
