package validator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Perfiles de tipo en un valor primitivo (#193, pieza 3).
//
// R4 elementdefinition-definitions.html#ElementDefinition.type.profile: «If any profiles are specified, then the
// content must conform to at least one of them». La regla no exceptúa a los primitivos: un `valueString`, un
// `given[0]` o un `birthDate` con uno o más perfiles de tipo se controlan como un tipo de datos (piezas 1 y 2): con un
// perfil se aplica, con varios se elige el que cumple (PROFILE_NO_MATCH, PROFILE_MULTIPLE_MATCHES), y un perfil que no
// se resuelve es TYPE_PROFILE_NOT_FOUND (plan: docs/plans/2026-11-15-primitive-type-profiles.md, §2 v3).
//
// De cada capa se aplican las constraints de la raíz del perfil (el foco es el primitivo con su `_x`, así que
// `extension`, `id`, `hasValue()` y `%context` se ven) y los hijos `id` y `extension` sobre `_x` (cardinalidad,
// slices obligatorios y prohibidos). `<tipo>.value` (maxLength, fixed, pattern, minValue, binding, constraints) no se
// aplica: queda para la pieza 3b, y HL7 tampoco lo aplica (límite temporal A4, casos c10-*).
//
// Los valores esperados salen de las salidas de HL7 validator_cli 7.0.4 (-tx n/a), no de gofhir:
// testdata/primitive_type_profiles/expected.json lo genera
// docs/plans/evidence/2026-11-15-primitive-type-profiles/tests/gen_expected.py desde las sondas de .../probe/ (346) y de
// p14, p15, p16 (pieza 2) y s22 (children-precedence), en R4, R4B y R5. Los perfiles y las instancias de
// testdata/primitive_type_profiles/ son los de las sondas, con fhirVersion de R4 (el test lo ajusta por versión). La
// correspondencia de IDs y la comparación son las de type_profile_layers_test.go.
//
// Excepciones declaradas, cada una con su comentario en gen_expected.py (el valor esperado no es el de HL7):
//   - c5-xc2-*, c5-oc2-*, c5-oc2r-* (plan A6, la misma divergencia que p12 de la pieza 2): un tipo de elección con
//     varios tipos usa todos los perfiles del tipo del valor (la especificación dice «at least one»), en cualquier
//     orden; HL7 aplica solo el primero.
//
//   - c13-pgu-extmin-none, c13-pnu-fam-giv (plan §8, R2, divergencia A7): con un perfil de tipo que no se resuelve HL7 no
//     aplica los hijos que el perfil de la instancia despliega para el valor; el valor esperado suma el `min` que HL7 sí
//     informa en el control sin perfil de tipo (c13-pgc-extmin-none, c13-pnc-fam-giv).
//
// Las demás líneas de HL7 se comparan como están, incluido el `ele-1` doble de R4B para un valor sin valor ni hijos.
// Una constraint de la raíz del tipo base de un primitivo (`ele-1`) y la de `checkValueless` se juntan por clave y
// expresión (plan C1): en R4 y R5 es una línea y en R4B son dos, como en HL7.

const primDir = "testdata/primitive_type_profiles"

var (
	primProbesOnce sync.Once
	primProbesVal  map[string]layerProbe
	primValsMu     sync.Mutex
	primVals       = map[string]*Validator{}
)

func primProbes(t testing.TB) map[string]layerProbe {
	t.Helper()
	primProbesOnce.Do(func() {
		raw, err := os.ReadFile(filepath.Join(primDir, "expected.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &primProbesVal); err != nil {
			t.Fatal(err)
		}
	})
	return primProbesVal
}

// primDefinitions lee los perfiles de las sondas, con la versión de FHIR de version (los archivos son de R4).
func primDefinitions(t testing.TB, version string) [][]byte {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(primDir, "ig", "*.json"))
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

func newPrimValidator(t testing.TB, version string) *Validator {
	t.Helper()
	v, err := New(WithVersion(version), WithConformanceResources(primDefinitions(t, version)))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// primValidator es el validador compartido de una versión; los tests que miran la memoización en frío usan uno propio.
func primValidator(t testing.TB, version string) *Validator {
	t.Helper()
	primValsMu.Lock()
	defer primValsMu.Unlock()
	if v, ok := primVals[version]; ok {
		return v
	}
	v := newPrimValidator(t, version)
	primVals[version] = v
	return v
}

func primInstance(t testing.TB, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(primDir, "inst", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// primRunBody valida el cuerpo de una instancia y da los informes de gofhir que esta pieza compara.
func primRunBody(t testing.TB, v *Validator, name string, body []byte) []gotIssue {
	t.Helper()
	res, err := v.Validate(context.Background(), body)
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

func primRun(t testing.TB, v *Validator, name string) []gotIssue {
	t.Helper()
	return primRunBody(t, v, name, primInstance(t, name))
}

// checkPrim compara lo que gofhir informó con lo que HL7 da (con la divergencia declarada, si la hay).
func checkPrim(t *testing.T, version, name string, got []gotIssue) {
	t.Helper()
	probe, ok := primProbes(t)[name]
	if !ok {
		t.Fatalf("la sonda %s no está en expected.json", name)
	}
	missing, extra := compareLayers(probe.Expected[version], got)
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("%s\nHL7 7.0.4 da %v\nfaltan %v\nde más %v", probe.Note, probe.Expected[version], missing, extraStrings(extra))
	}
}

// runPrimProbes compara a gofhir con HL7 en cada instancia de names, en R4, R4B y R5.
func runPrimProbes(t *testing.T, names []string) {
	t.Helper()
	runPrimProbesIn(t, layerVersions, names)
}

// runPrimProbesIn es runPrimProbes en las de versions.
func runPrimProbesIn(t *testing.T, versions, names []string) {
	t.Helper()
	for _, version := range versions {
		v := primValidator(t, version)
		for _, name := range names {
			t.Run(version+"/"+name, func(t *testing.T) {
				checkPrim(t, version, name, primRun(t, v, name))
			})
		}
	}
}

// primExpand da los nombres de las sondas que coinciden por entero con re.
func primExpand(t *testing.T, re string) []string {
	t.Helper()
	rx := regexp.MustCompile("^(?:" + re + ")$")
	var out []string
	for n := range primProbes(t) {
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

// Un perfil en un primitivo: la raíz sobre el valor (len-1, up-1, un entero, una fecha, un code), `id` y `extension`
// sobre `_x` (mínimo, slice obligatorio, constraint de `id`) y lo que `<tipo>.value` declara, que HL7 no aplica
// (c1-max, c1-fix, c1-pat, c1-valc, c1-imin, c1-ifx, c1-dmin y c1-bind: el valor esperado es no informar nada).
func TestPrimitiveOneProfile(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c1-.+`))
}

// Un perfil que no se resuelve: TYPE_PROFILE_NOT_FOUND en el valor, solo o con otro perfil (si el otro cumple, es lo
// único que se informa; si no, NoMatch y la constraint del que no cumple). En s22, el slice de primitivos con un perfil
// que no se resuelve informa SLICING_CANNOT_BE_EVALUATED.
func TestPrimitiveUnresolvedProfile(t *testing.T) {
	runPrimProbes(t, append(primExpand(t, `c2-.+`), "s22"))
}

// Dos o más perfiles: cumplen los dos (MULTIPLE_MATCHES), uno (nada) o ninguno (NO_MATCH y las constraints de cada
// uno), por elemento de un arreglo, con un valor ausente o un `_x` más largo, y con la misma clave en los dos perfiles
// (c4-*). Las sondas p14, p15 y p16 son las de la pieza 2 (dos perfiles de string en `Observation.value[x]`).
func TestPrimitiveTwoProfiles(t *testing.T) {
	runPrimProbes(t, append(primExpand(t, `c3-.+|c4-.+`), "p14", "p15", "p16"))
}

// Un tipo de elección con un solo perfil, y un slice por tipo (que sí elige entre los perfiles). El valor de otro
// tipo (c5-xc2-int) no tiene perfil.
func TestPrimitiveChoiceWithOneProfileAndSliceByType(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c5-xc1-.+|c5-oc1-.+|c5-os[12]-.+|c5-xc2-int`))
}

// Divergencia declarada A6 (la misma de p12 en la pieza 2): un tipo de elección con varios tipos usa todos los perfiles
// del tipo del valor, en los dos órdenes de los perfiles. El valor esperado es el de la especificación
// (ElementDefinition.type.profile: «the content must conform to at least one of them»), no el de HL7, que aplica solo
// el primero (InstanceValidator.java:8072 y :8085).
func TestPrimitiveDeclaredDivergences(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c5-xc2-(both|none|len)|c5-oc2r?-.+`))
}

// El resultado de la divergencia A6 no depende del orden de los perfiles ni de que el elemento sea un tipo de elección
// con un solo tipo: es el de la sonda de dos perfiles del mismo valor (sin mirar HL7).
func TestPrimitiveChoiceDoesNotDependOnTheProfileOrder(t *testing.T) {
	key := func(gs []gotIssue) []string {
		out := make([]string, len(gs))
		for i, g := range gs {
			out[i] = g.String()
		}
		sort.Strings(out)
		return out
	}
	for _, version := range layerVersions {
		v := primValidator(t, version)
		for _, tag := range []string{"both", "none", "len"} {
			t.Run(version+"/"+tag, func(t *testing.T) {
				a := key(primRun(t, v, "c5-oc2-"+tag))
				b := key(primRun(t, v, "c5-oc2r-"+tag))
				c := key(primRun(t, v, "c3-"+tag+"-obs"))
				if !slices.Equal(a, b) || !slices.Equal(a, c) {
					t.Errorf("c5-oc2-%s da %v, c5-oc2r-%s da %v, c3-%s-obs da %v", tag, a, tag, b, tag, c)
				}
			})
		}
	}
}

// %resource y %rootResource dentro de un contenido: el primitivo ve el contenido y el recurso raíz (c6-*).
func TestPrimitiveResourceVariablesInAContainedResource(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c6-.+`))
}

// A1: el foco de la raíz es el primitivo con su `_x`. Una constraint con `extension`, `id`, `hasValue()` o `%context`,
// con el escalar y sin él, sola y de a dos perfiles (c7-*).
func TestPrimitiveRootConstraintFocus(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c7-.+`))
}

// A2 y B2: la identidad de un primitivo es su posición (el map padre inmediato, la propiedad y el índice), no su valor
// ni su recurso. Escalares iguales en `name[0].given[0]` y `name[1].given[0]`, en `given[0]` y `given[1]` y en dos
// Patient de un Bundle, con `_x` distinto, se informan cada uno por separado (c8-*).
func TestPrimitiveIdentityIsThePosition(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c8-.+`))
}

// A3 y B3: slices de extensión obligatorios y prohibidos bajo el perfil de un primitivo, incluido un escalar sin `_x` cuyo
// perfil exige el slice y un miembro sin valor de un arreglo (c9-*).
func TestPrimitiveExtensionSlicesUnderTheProfile(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c9-.+`))
}

// A4, límite temporal de la pieza 3: un perfil que solo restringe `.value` cumple siempre (HL7 tampoco lo aplica, ni
// para elegir). Perfiles pareados que difieren solo en `.value`, y perfiles que mezclan `.value` con `id` o `extension`
// (c10-*). La pieza 3b cambia estos esperados junto con la elección, el memo, los diagnósticos y la capa que gobierna.
func TestPrimitiveValueOnlyProfilesAreTheTemporaryLimit(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c10-.+`))
}

// Sin perfiles el camino no cambia (plan §2.6 y B1): un primitivo sin `type.profile` informa lo de hoy, en
// R4, R4B y R5. Estos casos pasan antes y después de la pieza. Lo único que cambia es el `ele-1` doble de R4B para un
// valor sin valor ni hijos, que HL7 también informa: sus casos (c11-vl-*, c11-lon-giv) corren en R4 y R5 aquí, y en R4B
// en TestPrimitiveZeroProfileR4BDoubleEle1.
func TestPrimitiveZeroProfileGuard(t *testing.T) {
	names := primExpand(t, `c11-.+`)
	runPrimProbesIn(t, []string{"4.0.1", "5.0.0"}, names)
	var r4b []string
	for _, n := range names {
		if !strings.HasPrefix(n, "c11-vl-") && n != "c11-lon-giv" {
			r4b = append(r4b, n)
		}
	}
	runPrimProbesIn(t, []string{"4.3.0"}, r4b)
}

// El `ele-1` doble de R4B (plan B1 y C1): la raíz del tipo base de un primitivo y `checkValueless` informan `ele-1` con
// expresiones distintas en R4B, así que salen dos líneas, como en HL7; en R4 y R5 la expresión es la misma y sale una.
// Hoy gofhir informa una en R4B: este test falla hasta la pieza.
func TestPrimitiveZeroProfileR4BDoubleEle1(t *testing.T) {
	runPrimProbesIn(t, []string{"4.3.0"}, []string{"c11-vl-giv", "c11-vl-obs", "c11-vl-bd", "c11-lon-giv"})
}

// Cuántos `ele-1` salen en un valor sin valor ni hijos y sin perfiles: dos en R4B y una en R4 y R5, como HL7 (plan B1 y C1).
func TestPrimitiveZeroProfileValuelessEle1Count(t *testing.T) {
	want := map[string]int{"4.0.1": 1, "4.3.0": 2, "5.0.0": 1}
	for _, version := range layerVersions {
		v := primValidator(t, version)
		t.Run(version, func(t *testing.T) {
			n := 0
			for _, g := range primRun(t, v, "c11-vl-giv") {
				if g.Kind == "constraint" && g.Detail == "ele-1" && g.Location == "Patient.name[0].given[0]" {
					n++
				}
			}
			if n != want[version] {
				t.Errorf("ele-1 en given[0]: %d, quiero %d", n, want[version])
			}
		})
	}
}

// Tests metamórficos y de memoización.

// onode es un valor JSON que conserva el orden de las claves.
type onode struct {
	kind   byte // 'o' objeto, 'a' arreglo, 's' escalar
	keys   []string
	vals   []*onode
	scalar []byte
}

func parseOrdered(dec *json.Decoder) (*onode, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, isDelim := tok.(json.Delim)
	if !isDelim {
		raw, err := json.Marshal(tok)
		return &onode{kind: 's', scalar: raw}, err
	}
	n := &onode{kind: 'o'}
	if d == '[' {
		n.kind = 'a'
	}
	for dec.More() {
		if n.kind == 'o' {
			k, err := dec.Token()
			if err != nil {
				return nil, err
			}
			n.keys = append(n.keys, k.(string))
		}
		c, err := parseOrdered(dec)
		if err != nil {
			return nil, err
		}
		n.vals = append(n.vals, c)
	}
	_, err = dec.Token()
	return n, err
}

// reorder da el orden de las claves de un objeto: «orig» el del archivo, «rev» el inverso, «x-first» con las `_x` antes
// del valor y «x-last» con ellas después. El orden de un arreglo no cambia: es parte de la semántica.
func (n *onode) write(w io.Writer, mode string) {
	switch n.kind {
	case 's':
		_, _ = w.Write(n.scalar)
	case 'a':
		_, _ = io.WriteString(w, "[")
		for i, c := range n.vals {
			if i > 0 {
				_, _ = io.WriteString(w, ",")
			}
			c.write(w, mode)
		}
		_, _ = io.WriteString(w, "]")
	default:
		idx := make([]int, len(n.keys))
		for i := range idx {
			idx[i] = i
		}
		under := func(i int) bool { return strings.HasPrefix(n.keys[i], "_") }
		switch mode {
		case "rev":
			slices.Reverse(idx)
		case "x-first":
			sort.SliceStable(idx, func(a, b int) bool { return under(idx[a]) && !under(idx[b]) })
		case "x-last":
			sort.SliceStable(idx, func(a, b int) bool { return !under(idx[a]) && under(idx[b]) })
		}
		_, _ = io.WriteString(w, "{")
		for j, i := range idx {
			if j > 0 {
				_, _ = io.WriteString(w, ",")
			}
			k, _ := json.Marshal(n.keys[i])
			_, _ = w.Write(k)
			_, _ = io.WriteString(w, ":")
			n.vals[i].write(w, mode)
		}
		_, _ = io.WriteString(w, "}")
	}
}

func reordered(t testing.TB, body []byte, mode string) []byte {
	t.Helper()
	n, err := parseOrdered(json.NewDecoder(bytes.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	n.write(&out, mode)
	return out.Bytes()
}

// instancesWithUnderscore son las sondas con un `_x` (las que dependen del orden de `_x` y el valor), de todos los casos.
func instancesWithUnderscore(t testing.TB) []string {
	t.Helper()
	var out []string
	for name := range primProbes(t) {
		if bytes.Contains(primInstance(t, name), []byte(`"_`)) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func primKey(gs []gotIssue) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.String() + " | " + g.Diagnostics
	}
	sort.Strings(out)
	return out
}

// Metamórfico: reordenar las claves del JSON (en todos los niveles), o poner `_x` antes o después del valor, da el mismo
// resultado, y es el de HL7. Se comprueban todas las sondas con `_x`, en R4, R4B y R5.
func TestPrimitiveKeyOrderAndUnderscoreOrderDoNotMatter(t *testing.T) {
	names := instancesWithUnderscore(t)
	if len(names) < 100 {
		t.Fatalf("solo %d sondas con _x", len(names))
	}
	for _, version := range layerVersions {
		v := primValidator(t, version)
		t.Run(version, func(t *testing.T) {
			for _, name := range names {
				body := primInstance(t, name)
				base := primKey(primRunBody(t, v, name, body))
				for _, mode := range []string{"rev", "x-first", "x-last"} {
					got := primRunBody(t, v, name, reordered(t, body, mode))
					if !slices.Equal(base, primKey(got)) {
						t.Errorf("%s con %s da %v y en el orden del archivo %v", name, mode, primKey(got), base)
					}
					if mode == "x-first" {
						checkPrim(t, version, name, got)
					}
				}
			}
		})
	}
}

// Memoización en frío contra en caliente (B2): escalares iguales con `_x` distinto, bajo padres distintos y en recursos
// distintos, dan lo mismo con la memoria vacía que con la memoria llena de los vecinos, en cualquier orden.
func TestPrimitiveColdAndWarmMemoGiveTheSameResult(t *testing.T) {
	groups := [][]string{
		{"c8-par-two-a", "c8-par-two-b", "c8-par-two-c", "c8-par-two-d", "c8-idx-two", "c8-idx-two-r"},
		{"c8-par-twoup-a", "c8-par-twoup-b", "c8-par-twoup-c", "c8-par-twoup-d", "c8-idx-twoup", "c8-idx-twoup-r"},
		{"c8-par-sub-a", "c8-par-sub-b", "c8-res-sub-ab", "c8-res-sub-ba", "c8-res-sub-aa", "c8-res-sub-bb"},
		{"c8-res-two-ab", "c8-res-two-ba", "c8-res-two-aa", "c8-res-two-bb", "c8-par-one-a", "c8-par-one-b"},
		{"c9-sublen-none-giv", "c9-sublen-sub-giv", "c9-sublen-oth-giv", "c9-sublen-both-giv", "c9-sublen-vl"},
	}
	for _, version := range layerVersions {
		for gi, group := range groups {
			t.Run(fmt.Sprintf("%s/g%d", version, gi), func(t *testing.T) {
				// En frío: la primera y la última instancia, cada una con un validador sin historia.
				for _, name := range []string{group[0], group[len(group)-1]} {
					cold := newPrimValidator(t, version)
					checkPrim(t, version, name, primRun(t, cold, name))
				}
				// En caliente: el mismo validador valida todas, dos veces y en los dos órdenes.
				warm := newPrimValidator(t, version)
				for pass := 0; pass < 2; pass++ {
					order := slices.Clone(group)
					if pass == 1 {
						slices.Reverse(order)
					}
					for _, name := range order {
						checkPrim(t, version, name, primRun(t, warm, name))
					}
				}
			})
		}
	}
}

// El primitivo se reconoce por su posición aunque el escalar sea el mismo: con la misma instancia validada dos veces en
// el mismo validador el resultado no cambia (c8-par-*-a en caliente contra la primera vez).
func TestPrimitiveSameInstanceTwiceGivesTheSameResult(t *testing.T) {
	for _, version := range layerVersions {
		v := newPrimValidator(t, version)
		for _, name := range []string{"c8-par-two-a", "c8-par-sub-a", "c7-rxri-none-obs", "c9-sublen-oth-obs"} {
			t.Run(version+"/"+name, func(t *testing.T) {
				first := primKey(primRun(t, v, name))
				second := primKey(primRun(t, v, name))
				if !slices.Equal(first, second) {
					t.Errorf("primera %v, segunda %v", first, second)
				}
			})
		}
	}
}

// Huecos de la corrida de mutaciones (plan §7, «Huecos cerrados»): entradas que mata cada mutante que sobrevivió (c12-*).
//   - c12-pgp-*, c12-pgs*: un slice de `given` con discriminador type=profile o value cuyo `type.profile` es de un primitivo
//     (el descriptor del primitivo llega a profileConforms; identidad por posición: dos `name` iguales con `_given` distinto;
//     un perfil de otro tipo; el slicing baja a un primitivo y a un miembro de un slice);
//   - c12-pft-*: `family` y `text` bajo el mismo padre con los mismos dos perfiles y `_x` distinto (la clave es parte de la posición);
//   - c12-pnh*: un perfil de HumanName cuyo `given` tiene un perfil de primitivo con un slice obligatorio;
//   - c12-{idc-up,extc-up,len-extw,extw-len}-*: los hijos `id` y `extension` del perfil entran en la elección (Conforms) y las
//     constraints de aviso de todas las capas se informan;
//   - c12-ext1x2-*: dos perfiles que exigen lo mismo se informan una vez;
//   - c12-xs-*: el perfil de un slice de extensión de un perfil no elegido no se informa (contexto, valor, binding, reference).
func TestPrimitiveMutationGaps(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c12-.+`))
}

// Correcciones de la ronda 1 de la revisión de código (plan §8, c13-*):
//   - c13-dec-*: el texto de un decimal decide la elección entre perfiles (`precision()` y `toString()` leen 1.50, no 1.5);
//   - c13-pgu-*: un perfil de tipo que no se resuelve se informa aunque el perfil de la instancia despliegue los hijos
//     del primitivo (`given.extension`, `given.id`), solo o con otro perfil;
//   - c13-pnu-*: lo mismo en un tipo de datos (`name` con un perfil de HumanName que no se resuelve y un hijo propio).
func TestPrimitiveReviewRound1(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c13-.+`))
}

// Precedencia por clave entre el dueño y el perfil de su tipo (plan §8, ronda 2, S1; caso 14 de las sondas). Con la misma
// clave de constraint declarada por el elemento y por el perfil de su tipo (primitivo o tipo de datos), HL7 informa solo la
// del perfil de tipo, con su severidad y su expresión (c14-a*, c14-b*). Un slice o un perfil derivado que vuelve a
// declarar la clave sigue como antes: gana la base (c14-c*, c14-d*).
func TestTypeProfileConstraintPrecedence(t *testing.T) {
	runPrimProbes(t, primExpand(t, `c1[4-7]-.+`))
}
