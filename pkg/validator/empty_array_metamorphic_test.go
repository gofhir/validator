package validator

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// Plan: docs/plans/2026-11-02-empty-array.md (§Tests 2, A2, A3, A10), issue #190.
//
// El oráculo metamórfico corre sobre los ejemplos oficiales (hl7.fhir.r4/r4b/r5.examples), con json.html §2.6.2.1
// (R4, R4B): «Omit a property if it is empty»; la propiedad escrita como [] equivale a la omitida más el
// error del array vacío (ADR-003 para los primitivos y su "_x" en R5). Dos mutaciones por recurso:
//
//   - agregar: en un objeto, un hijo que se repite y no está, escrito []. Lo esperado es la línea base más un
//     STRUCTURE_ARRAY_EMPTY en ese path (si la regla de la versión lo pide), y ningún otro cambio.
//   - vaciar: un array existente pasa a []. Lo esperado es lo que da la instancia con la propiedad omitida
//     más el STRUCTURE_ARRAY_EMPTY; salvo que la omisión deje el objeto {} (A2): entonces la omitida suma un
//     STRUCTURE_EMPTY_OBJECT en el objeto, y la escrita con [] en su lugar un ele-1 en el mismo path (un
//     array vacío no es contenido, HL7 7.0.4).
//
// El esquema (qué hijos hay, cuáles se repiten, de qué tipo es cada objeto) sale del snapshot por el
// registro, sin nombres de elementos. Se compara por (ID, severidad, expresión), sin ubicación (A3).

// Cotas de la muestra: eaSampleTarget son los recursos por versión (a paso fijo sobre los archivos
// ordenados), eaMaxBytes el tamaño máximo de uno, eaShadowTarget los "_x" existentes que se vacían por
// versión (se buscan en todos los ejemplos) y eaShadowMin lo mínimo que se exige de ellos.
const (
	eaShadowTarget = 80
	eaShadowMin    = 10 // los ejemplos (hasta 60 KB) tienen 12 en R4 y R4B y 17 en R5
	eaSampleTarget = 260
	eaMaxBytes     = 60_000
)

// eaForbidsPrimitive es la regla de ADR-003 escrita aparte del código de producción: los primitivos y su
// "_x" se rechazan en R4 y R4B, y no en R5.
func eaForbidsPrimitive(version string) bool { return version != "5.0.0" }

// eaCtx es el contexto de tipos de un objeto de la instancia: los hijos del elemento node de sd.
type eaCtx struct {
	sd   *registry.StructureDefinition
	node *registry.ElementNode
}

type eaWalker struct {
	reg *registry.Registry
}

func (w eaWalker) root(rt string) (eaCtx, bool) {
	sd := w.reg.GetByType(rt)
	if sd == nil || sd.Snapshot == nil || sd.Tree().Root() == nil {
		return eaCtx{}, false
	}
	return eaCtx{sd, sd.Tree().Root()}, true
}

// child busca el hijo de cx que nombra key (un elemento de elección, por el tipo que sigue a su nombre).
func (w eaWalker) child(cx eaCtx, key string) *registry.ElementNode {
	for _, c := range cx.node.Children {
		name := c.Name()
		if base, ok := strings.CutSuffix(name, "[x]"); ok {
			if strings.HasPrefix(key, base) && w.reg.ChoiceType(c, key) != "" {
				return c
			}
		} else if name == key {
			return c
		}
	}
	return nil
}

// typeOf devuelve el código del tipo de c para la propiedad key, y si es primitivo; "" si no se sabe.
func (w eaWalker) typeOf(c *registry.ElementNode, key string) (code string, primitive bool) {
	if c.Def.ContentReference != nil {
		return "", false
	}
	if strings.HasSuffix(c.Name(), "[x]") {
		code = w.reg.ChoiceType(c, key)
	} else if len(c.Def.Type) == 1 {
		code = c.Def.Type[0].Code
	}
	return code, code != "" && w.reg.IsPrimitiveTypeCode(code)
}

// objectCtx devuelve el contexto del objeto m, el valor de la propiedad key del hijo c de cx.
func (w eaWalker) objectCtx(cx eaCtx, c *registry.ElementNode, key string, m map[string]any) (eaCtx, bool) {
	if c.Def.ContentReference != nil {
		sd, node, res := w.reg.ContentReferenceTarget(cx.sd, c)
		return eaCtx{sd, node}, res == registry.ResolutionExact
	}
	if w.reg.HoldsResource(c.Def) {
		rt, _ := m["resourceType"].(string)
		return w.root(rt)
	}
	code, primitive := w.typeOf(c, key)
	switch {
	case code == "" || primitive:
		return eaCtx{}, false
	case code == "BackboneElement" || code == "Element":
		return eaCtx{cx.sd, c}, true
	}
	return w.root(code)
}

// eaKind es la mutación de un candidato.
type eaKind int

const (
	eaAdd       eaKind = iota // agregar "key": [] donde no está
	eaEmpty                   // "key": [...] pasa a []
	eaShadow                  // "_key": [...] pasa a []
	eaAddShadow               // "_key": [] donde no está, junto a "key": [...]
)

// eaCand es una mutación posible: en el objeto at (pasos desde la raíz), de path path, la propiedad key.
type eaCand struct {
	kind      eaKind
	at        []any
	path      string // path del objeto, como lo escriben los issues
	key       string // propiedad de la que se habla, sin "_"
	primitive bool
	holder0   bool // la omisión deja el objeto {}
	shadowCtx bool // el objeto es el contenido de un "_x"
}

// elementPath es el path del issue del array vacío: el del elemento, sin índice.
func (c eaCand) elementPath() string { return c.path + "." + c.key }

// candidates visita la instancia y devuelve las mutaciones posibles.
func (w eaWalker) candidates(root map[string]any) []eaCand {
	var out []eaCand
	var walk func(obj map[string]any, cx eaCtx, at []any, path string, shadow bool)
	walk = func(obj map[string]any, cx eaCtx, at []any, path string, shadow bool) {
		names := make([]string, 0, len(obj))
		for name := range obj {
			names = append(names, name)
		}
		sort.Strings(names)
		// agregar: hijos que se repiten y faltan, en un objeto con contenido
		if len(obj) > 0 {
			for _, c := range cx.node.Children {
				name := c.Name()
				if strings.HasSuffix(name, "[x]") || !c.Def.Repeats() {
					continue
				}
				if _, ok := obj[name]; ok {
					continue
				}
				if _, ok := obj["_"+name]; ok {
					continue
				}
				_, prim := w.typeOf(c, name)
				if !prim && c.Def.ContentReference == nil && !w.reg.HoldsResource(c.Def) && len(c.Def.Type) != 1 {
					continue
				}
				out = append(out, eaCand{kind: eaAdd, at: slices.Clone(at), path: path, key: name, primitive: prim, shadowCtx: shadow})
			}
		}
		for _, name := range names {
			val := obj[name]
			if name == "resourceType" {
				continue
			}
			key := strings.TrimPrefix(name, "_")
			c := w.child(cx, key)
			if c == nil {
				continue
			}
			_, prim := w.typeOf(c, key)
			isShadow := strings.HasPrefix(name, "_")
			if isShadow && !prim {
				continue
			}
			if arr, ok := val.([]any); ok && c.Def.Repeats() {
				if len(arr) > 0 {
					k := eaEmpty
					if isShadow {
						k = eaShadow
					}
					_, hasSibling := obj["_"+key]
					if !isShadow && prim && !hasSibling {
						out = append(out, eaCand{kind: eaAddShadow, at: slices.Clone(at), path: path, key: key, primitive: true, shadowCtx: shadow})
					}
					out = append(out, eaCand{kind: k, at: slices.Clone(at), path: path, key: key, primitive: prim,
						holder0: len(obj) == 1, shadowCtx: shadow})
				}
				for i, item := range arr {
					m, ok := item.(map[string]any)
					if !ok {
						continue
					}
					ipath := path + "." + name + "[" + strconv.Itoa(i) + "]"
					iat := append(slices.Clone(at), name, i)
					if isShadow {
						if el, ok := w.root("Element"); ok {
							walk(m, el, iat, ipath, true)
						}
						continue
					}
					if sub, ok := w.objectCtx(cx, c, key, m); ok {
						walk(m, sub, iat, ipath, shadow)
					}
				}
				continue
			}
			m, ok := val.(map[string]any)
			if !ok || c.Def.Repeats() {
				continue
			}
			npath := path + "." + name
			nat := append(slices.Clone(at), name)
			if isShadow {
				if el, ok := w.root("Element"); ok {
					walk(m, el, nat, npath, true)
				}
				continue
			}
			if sub, ok := w.objectCtx(cx, c, key, m); ok {
				walk(m, sub, nat, npath, shadow)
			}
		}
	}
	rt, _ := root["resourceType"].(string)
	if cx, ok := w.root(rt); ok {
		walk(root, cx, nil, rt, false)
	}
	return out
}

// eaClone copia la instancia y devuelve la copia y el objeto at de ella.
func eaClone(root map[string]any, at []any) (copied, obj map[string]any) {
	data, err := json.Marshal(root)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &copied); err != nil {
		panic(err)
	}
	var node any = copied
	for _, step := range at {
		switch s := step.(type) {
		case string:
			node = node.(map[string]any)[s]
		case int:
			node = node.([]any)[s]
		}
	}
	return copied, node.(map[string]any)
}

// eaLines son los issues como «ID | severidad | expresión», ordenados: sin ubicación ni mensaje (A3).
func eaLines(res *issue.Result) []string {
	lines := make([]string, 0, len(res.Issues))
	for _, is := range res.Issues {
		lines = append(lines, is.MessageID+" | "+string(is.Severity)+" | "+strings.Join(is.Expression, ","))
	}
	slices.Sort(lines)
	return lines
}

func eaRemove(lines []string, line string) ([]string, bool) {
	i := slices.Index(lines, line)
	if i < 0 {
		return lines, false
	}
	return slices.Delete(slices.Clone(lines), i, i+1), true
}

func eaMarshal(m map[string]any) []byte {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

func TestEmptyArrayMetamorphic(t *testing.T) {
	if testing.Short() {
		t.Skip("metamorphic oracle over the official examples")
	}
	packages := loader.DefaultPackagePath()
	sources := []struct{ version, dir string }{
		{"4.0.1", filepath.Join(packages, "hl7.fhir.r4.examples#4.0.1", "package")},
		{"4.3.0", filepath.Join(packages, "hl7.fhir.r4b.examples#4.3.0", "package")},
		{"5.0.0", filepath.Join(packages, "hl7.fhir.r5.examples#5.0.0", "package")},
	}
	for _, src := range sources {
		t.Run(src.version, func(t *testing.T) {
			start := time.Now()
			files := resourceFiles(t, src.dir)
			if len(files) == 0 {
				t.Fatalf("no resources in %s", src.dir)
			}
			v := orderValidator(t, src.version)
			w := eaWalker{v.Registry()}
			stride := max(1, len(files)/eaSampleTarget)
			forbids := eaForbidsPrimitive(src.version)
			counts := map[string]int{}
			resources, validations := 0, 0
			for fi := range files {
				// fuera de la muestra a paso fijo solo interesan los "_x" que se vacían, hasta eaShadowTarget
				sampled := fi%stride == 0
				if !sampled && counts["shadow"] >= eaShadowTarget {
					continue
				}
				if st, err := os.Stat(files[fi]); err != nil || st.Size() > eaMaxBytes {
					continue
				}
				data, err := os.ReadFile(files[fi])
				if err != nil {
					t.Fatal(err)
				}
				var root map[string]any
				if json.Unmarshal(data, &root) != nil {
					continue
				}
				if sampled {
					resources++
				}
				rng := rand.New(rand.NewPCG(uint64(fi), 190))
				groups := map[string][]eaCand{}
				for _, c := range w.candidates(root) {
					g := fmt.Sprintf("%d/%t/%t", c.kind, c.primitive, c.shadowCtx && c.kind == eaAdd)
					groups[g] = append(groups[g], c)
				}
				keys := make([]string, 0, len(groups))
				for g := range groups {
					keys = append(keys, g)
				}
				sort.Strings(keys)
				var base *issue.Result
				for _, g := range keys {
					cs := groups[g]
					if !sampled && !strings.HasPrefix(g, fmt.Sprintf("%d/", eaShadow)) {
						continue
					}
					if !strings.HasPrefix(g, fmt.Sprintf("%d/", eaShadow)) {
						// uno por grupo; los "_x" que se vacían son pocos en los ejemplos: se prueban todos
						cs = []eaCand{cs[rng.IntN(len(cs))]}
					}
					for _, c := range cs {
						if base == nil {
							base = validateJSON(t, v, eaMarshal(root))
							validations++
						}
						validations += eaCheck(t, v, src.version, files[fi], root, c, base, forbids, counts)
					}
				}
			}
			t.Logf("%s: %d resources, %d cases (%v), %d validations, %s", src.version, resources,
				counts["add"]+counts["empty"]+counts["shadow"], counts, validations, time.Since(start).Round(time.Millisecond))
			if resources < 100 || counts["add"] < 100 || counts["empty"] < 100 || counts["shadow"] < eaShadowMin {
				t.Errorf("only %d resources and cases %v", resources, counts)
			}
		})
	}
}

// eaCheck aplica la mutación c a una copia de root y compara con lo que dice la relación metamórfica.
// Devuelve cuántas validaciones hizo.
func eaCheck(t *testing.T, v *Validator, version, file string, root map[string]any, c eaCand, base *issue.Result, forbids bool, counts map[string]int) int {
	t.Helper()
	report := !c.primitive || forbids
	arrayEmpty := eaMakeLine(string(issue.DiagStructureArrayEmpty), c.elementPath())
	where := fmt.Sprintf("%s %s %s: %s", version, filepath.Base(file), []string{"add", "empty", "shadow", "addshadow"}[c.kind], c.elementPath())
	switch c.kind {
	case eaAdd, eaAddShadow:
		counts["add"]++
		name := c.key
		if c.kind == eaAddShadow {
			name = "_" + c.key
			counts["addshadow"]++
		}
		mutated, obj := eaClone(root, c.at)
		obj[name] = []any{}
		res := validateJSON(t, v, eaMarshal(mutated))
		want := eaLines(base)
		if report {
			want = append(want, arrayEmpty)
		}
		eaCompare(t, where, res, want, report)
		return 1
	default:
		name := c.key
		if c.kind == eaShadow {
			name = "_" + c.key
			counts["shadow"]++
		} else {
			counts["empty"]++
		}
		emptied, obj := eaClone(root, c.at)
		obj[name] = []any{}
		omitted, obj2 := eaClone(root, c.at)
		delete(obj2, name)
		twin := validateJSON(t, v, eaMarshal(omitted))
		want := eaLines(twin)
		if c.holder0 {
			// A2: la omisión deja {} y suma un objeto vacío que la escrita con [] no tiene. Un array vacío no
			// es contenido (HL7 7.0.4): la escrita con [] no recibe STRUCTURE_EMPTY_OBJECT sino ele-1, en el
			// mismo path, que la omitida no tiene (el objeto vacío ya se reporta).
			if c.shadowCtx {
				return 1 // el path del objeto vacío bajo un "_x" es otro (el del elemento): no se predice aquí
			}
			var ok bool
			if want, ok = eaRemove(want, eaMakeLine(string(issue.DiagStructureEmptyObject), c.path)); !ok {
				t.Errorf("%s: omitting leaves %s as {} and the twin has no STRUCTURE_EMPTY_OBJECT there: %q", where, c.path, eaLines(twin))
				return 2
			}
		}
		res := validateJSON(t, v, eaMarshal(emptied))
		if c.holder0 {
			// ele-1 es uno por texto de la invariante: HL7 7.0.4 da dos en un tipo de datos de R4B (los dos
			// textos, hl7diff-run-7.0.4.txt) y uno en un BackboneElement (sonda ValueSet.compose) y en las
			// demás releases. Aquí se exige entre uno y el máximo de la release; las sondas fijan el número.
			n := eaEle1At(res, c.path)
			if n < 1 || n > eaEle1Max(version) {
				t.Errorf("%s: %s holds only the empty array: %d ele-1, want 1..%d: %q", where, c.path, n, eaEle1Max(version), eaLines(res))
			}
			for range n {
				want = append(want, eaMakeLine(string(issue.DiagConstraintFailed), c.path))
			}
		}
		if report {
			want = append(want, arrayEmpty)
		}
		eaCompare(t, where, res, want, report)
		return 2
	}
}

// eaEle1At cuenta los ele-1 de res en path.
func eaEle1At(res *issue.Result, path string) int {
	n := 0
	for _, is := range res.Issues {
		if is.MessageID == string(issue.DiagConstraintFailed) && strings.Contains(is.Diagnostics, "ele-1:") &&
			strings.Join(is.Expression, ",") == path {
			n++
		}
	}
	return n
}

// eaEle1Max es el máximo de ele-1 que HL7 7.0.4 da sobre un objeto: R4B tiene dos textos de la invariante.
func eaEle1Max(version string) int {
	if version == "4.3.0" {
		return 2
	}
	return 1
}

func eaMakeLine(id, expr string) string {
	return id + " | " + string(issue.SeverityError) + " | " + expr
}

// eaCompare exige que res dé want y, si reporta el array vacío, que el issue tenga ubicación.
func eaCompare(t *testing.T, where string, res *issue.Result, want []string, report bool) {
	t.Helper()
	slices.Sort(want)
	if got := eaLines(res); !slices.Equal(got, want) {
		t.Errorf("%s\n got  %q\n want %q", where, diffOnly(got, want), diffOnly(want, got))
	}
	if !report {
		return
	}
	for _, is := range res.Issues {
		if is.MessageID == string(issue.DiagStructureArrayEmpty) && (is.Location == nil || is.Location.Line == 0) {
			t.Errorf("%s: %s at %s has no location", where, is.MessageID, strings.Join(is.Expression, ","))
		}
	}
}

// diffOnly son las líneas de a que b no tiene (con multiplicidad).
func diffOnly(a, b []string) []string {
	rest := slices.Clone(b)
	var out []string
	for _, l := range a {
		if i := slices.Index(rest, l); i >= 0 {
			rest = slices.Delete(rest, i, i+1)
		} else {
			out = append(out, l)
		}
	}
	return out
}
