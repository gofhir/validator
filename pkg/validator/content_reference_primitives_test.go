package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// Plan: docs/plans/2026-11-02-content-reference-primitives.md (§2 y §3 «Plan ajustado»), issue #189.
//
// Un elemento con contentReference no declara tipo: su contenido es el del elemento al que apunta
// (elementdefinition.html#ElementDefinition.contentReference). Un valor primitivo mal formado es un error
// en el elemento referido y bajo la contentReference (json.html#primitive; datatypes.html).

var crVersions = []string{"4.0.1", "4.3.0", "5.0.0"}

var (
	crMu         sync.Mutex
	crValidators = map[string]*Validator{}
)

// crValidator reutiliza un validador por versión: crearlo cuesta más que cualquier caso.
func crValidator(t testing.TB, version string, defs ...[]byte) *Validator {
	t.Helper()
	crMu.Lock()
	defer crMu.Unlock()
	key := version
	if len(defs) > 0 {
		key += "+defs"
	}
	if v := crValidators[key]; v != nil {
		return v
	}
	opts := []Option{WithVersion(version)}
	if len(defs) > 0 {
		opts = append(opts, WithConformanceResources(defs))
	}
	v, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	crValidators[key] = v
	return v
}

// crIsPrimitive es un issue de la fase de primitivos: el JSON del valor es de otro tipo, o su texto
// no cumple el formato del tipo.
func crIsPrimitive(is issue.Issue) bool {
	return is.MessageID == string(issue.DiagTypeWrongJSONType) || is.MessageID == string(issue.DiagTypeInvalidFormat)
}

// crPrimitiveIssues devuelve los issues de primitivos de res como «ID expresión», ordenados.
func crPrimitiveIssues(res *issue.Result) []string {
	var out []string
	for _, is := range res.Issues {
		if crIsPrimitive(is) {
			out = append(out, is.MessageID+" "+strings.Join(is.Expression, ","))
		}
	}
	slices.Sort(out)
	return out
}

func crValidate(t testing.TB, v *Validator, data any) []string {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Validate(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	return crPrimitiveIssues(res)
}

func crValidateRaw(t testing.TB, v *Validator, raw string) []string {
	t.Helper()
	res, err := v.Validate(context.Background(), []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return crPrimitiveIssues(res)
}

const (
	crWrong  = string(issue.DiagTypeWrongJSONType)
	crFormat = string(issue.DiagTypeInvalidFormat)
)

// crProbe es una sonda de docs/plans/evidence/2026-11-02-content-reference-primitives/probe/in; want son
// los issues de primitivos que informa HL7 6.10.2 (probe/hl7.txt), igual en R4, R4B y R5, con la
// propiedad por su nombre JSON en la extensión (HL7 escribe value[x]).
type crProbe struct {
	name string
	json string
	want []string
}

func crProbes() []crProbe {
	const (
		obs = `{"resourceType":"Observation","status":"final","code":{"text":"x"},"component":[{"code":{"text":"y"},"referenceRange":[%s]}]}`
		q   = `{"resourceType":"Questionnaire","status":"draft","item":[{"linkId":"a","type":"group","item":[%s]}]}`
	)
	return []crProbe{
		{"rr-comp-decstr", fmt.Sprintf(obs, `{"low":{"value":"1.5"}}`),
			[]string{crWrong + " Observation.component[0].referenceRange[0].low.value"}},
		{"rr-comp-empty", fmt.Sprintf(obs, `{"text":""}`),
			[]string{crFormat + " Observation.component[0].referenceRange[0].text"}},
		{"rr-comp-ok", fmt.Sprintf(obs, `{"low":{"value":1.5},"text":"t"}`), nil},
		{"q-req-str", fmt.Sprintf(q, `{"linkId":"b","type":"string","required":"yes"}`),
			[]string{crWrong + " Questionnaire.item[0].item[0].required"}},
		{"q-req-ok", fmt.Sprintf(q, `{"linkId":"b","type":"string","required":true}`), nil},
		{"q-def-space", fmt.Sprintf(q, `{"linkId":"b","type":"string","definition":"has space"}`),
			[]string{crFormat + " Questionnaire.item[0].item[0].definition"}},
		{"q-ext-str", fmt.Sprintf(q, `{"linkId":"b","type":"string","extension":[{"url":"http://example.org/x","valueBoolean":"true"}]}`),
			[]string{crWrong + " Questionnaire.item[0].item[0].extension[0].valueBoolean"}},
		{"q-deep-int", `{"resourceType":"Questionnaire","status":"draft","item":[{"linkId":"a","type":"group","item":[{"linkId":"b","type":"group","item":[{"linkId":"c","type":"integer","maxLength":"5"}]}]}]}`,
			[]string{crWrong + " Questionnaire.item[0].item[0].item[0].maxLength"}},
		// El recurso de la sonda rr-comp-decstr como entrada de un Bundle.
		{"held-rr-comp-decstr", fmt.Sprintf(`{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"urn:uuid:3f1c1b4e-2a5f-4c55-9d8e-0d7a8a6f1b00","resource":`+obs+`}]}`, `{"low":{"value":"1.5"}}`),
			[]string{crWrong + " Bundle.entry[0].resource.component[0].referenceRange[0].low.value"}},
	}
}

func TestContentReferencePrimitivesProbes(t *testing.T) {
	for _, version := range crVersions {
		v := crValidator(t, version)
		for _, p := range crProbes() {
			t.Run(version+"/"+p.name, func(t *testing.T) {
				if got := crValidateRaw(t, v, p.json); !slices.Equal(got, p.want) {
					t.Errorf("primitive issues = %q, HL7 reports %q", got, p.want)
				}
			})
		}
	}
}

// crInstance arma el JSON de un recurso de tipo sd.Type con leaf en path (un path del snapshot), los
// elementos intermedios como dice el snapshot (arreglo donde se repite), y el FHIRPath del padre de
// esa hoja en la instancia.
func crInstance(sd *registry.StructureDefinition, path string, leaf map[string]any) (root map[string]any, fhirPath string, ok bool) {
	segs := strings.Split(path, ".")
	if len(segs) < 2 || strings.Contains(path, "[x]") {
		return nil, "", false
	}
	var cur any = leaf
	fhir := make([]string, len(segs)-1)
	for i := len(segs) - 1; i >= 1; i-- {
		node := sd.Tree().ByID(strings.Join(segs[:i+1], "."))
		if node == nil {
			return nil, "", false
		}
		fhir[i-1] = segs[i]
		if node.Def.Repeats() {
			cur = []any{cur}
			fhir[i-1] += "[0]"
		}
		cur = map[string]any{segs[i]: cur}
	}
	root = cur.(map[string]any)
	root["resourceType"] = sd.Type
	return root, segs[0] + "." + strings.Join(fhir, "."), true
}

type crStep struct {
	name    string
	repeats bool
}

// crLeaf es una propiedad de tipo primitivo bajo un elemento: las propiedades desde el elemento hacia
// abajo y el tipo primitivo.
type crLeaf struct {
	steps []crStep
	typ   string
}

func (l crLeaf) String() string {
	names := make([]string, len(l.steps))
	for i, s := range l.steps {
		names[i] = s.name
	}
	return strings.Join(names, ".") + ":" + l.typ
}

func crTitle(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

// crLeaves lista las propiedades primitivas bajo node: sus hijos (las opciones de un choice, nombradas
// por los tipos de la base), las de un hijo en línea y, un nivel dentro de un datatype, sus hijos primitivos.
func crLeaves(reg *registry.Registry, node *registry.ElementNode, prefix []crStep, depth int) []crLeaf {
	var out []crLeaf
	for _, c := range node.Children {
		if c.Def.ContentReference != nil {
			continue
		}
		name := c.Name()
		for _, ty := range c.Def.Type {
			code := ty.Code
			if strings.HasPrefix(code, "http://hl7.org/fhirpath/") || reg.IsResourceType(code) {
				continue
			}
			prop := name
			if base, ok := strings.CutSuffix(name, "[x]"); ok {
				prop = base + crTitle(code)
			}
			steps := append(slices.Clone(prefix), crStep{prop, c.Def.Repeats()})
			switch {
			case reg.IsPrimitiveTypeCode(code):
				out = append(out, crLeaf{steps, code})
			case len(c.Children) > 0 && depth < 2 && prop == name:
				out = append(out, crLeaves(reg, c, steps, depth+1)...)
			case reg.IsDataType(code) && !reg.IsPrimitiveType(code):
				tsd := reg.GetByType(code)
				if tsd == nil || tsd.Tree().Root() == nil {
					continue
				}
				for _, dc := range tsd.Tree().Root().Children {
					for _, dt := range dc.Def.Type {
						if dc.Def.ContentReference != nil || !reg.IsPrimitiveTypeCode(dt.Code) || strings.HasSuffix(dc.Name(), "[x]") {
							continue
						}
						out = append(out, crLeaf{append(slices.Clone(steps), crStep{dc.Name(), dc.Def.Repeats()}), dt.Code})
					}
				}
			}
		}
	}
	return out
}

// crBadValues son valores mal escritos para algún tipo primitivo: uno de cada tipo JSON y textos y
// números que los formatos rechazan. Un valor puede ser válido para algunos tipos (y entonces no
// informa nada en ningún lugar); todo tipo primitivo rechaza al menos uno.
var crBadValues = []any{true, float64(42), 1.5, float64(-1), "x", "a  b\t", ""}

func crSet(root map[string]any, steps []crStep, value any) {
	var cur = value
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].repeats {
			cur = []any{cur}
		}
		cur = map[string]any{steps[i].name: cur}
	}
	for k, v := range cur.(map[string]any) {
		root[k] = v
	}
}

// crNormalize reescribe el prefijo from de las expresiones de los issues como to.
func crNormalize(issues []string, from, to string) []string {
	out := make([]string, 0, len(issues))
	for _, s := range issues {
		if strings.Contains(s, from) {
			s = strings.ReplaceAll(s, from, to)
		}
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

// Un primitivo mal formado bajo cada contentReference de los recursos núcleo se informa como en el
// elemento que la contentReference nombra (elementdefinition.html#ElementDefinition.contentReference).
// La expectativa es la relación metamórfica, no un resultado guardado: aquí no se escribe ningún nombre
// de recurso, elemento ni tipo.
func TestContentReferencePrimitivesMetamorphic(t *testing.T) {
	for _, version := range crVersions {
		t.Run(version, func(t *testing.T) {
			v := crValidator(t, version)
			reg := v.Registry()
			urls := reg.AllURLs()
			slices.Sort(urls)
			start := time.Now()
			var cases, elems, leavesN, vacuous int
			for _, url := range urls {
				sd := reg.GetByURL(url)
				if sd == nil || sd.Kind != "resource" || sd.Derivation != "specialization" || sd.Abstract || sd.Snapshot == nil {
					continue
				}
				for i := range sd.Snapshot.Element {
					e := &sd.Snapshot.Element[i]
					if e.ContentReference == nil {
						continue
					}
					node := sd.Tree().ByID(e.ID)
					target, res := reg.ContentReference(sd, node)
					if res != registry.ResolutionExact {
						t.Errorf("%s: contentReference %s does not resolve", e.ID, *e.ContentReference)
						continue
					}
					elems++
					leaves := crLeaves(reg, target, nil, 0)
					if len(leaves) == 0 {
						t.Errorf("%s: no primitive properties under %s", e.ID, target.Def.ID)
					}
					leavesN += len(leaves)
					for _, leaf := range leaves {
						reported := false
						for _, bad := range crBadValues {
							atT, prefT, okT := crInstance(sd, target.Def.Path, map[string]any{})
							atE, prefE, okE := crInstance(sd, e.Path, map[string]any{})
							if !okT || !okE {
								t.Errorf("%s: no instance path for %s or %s", e.ID, target.Def.Path, e.Path)
								break
							}
							leafT, leafE := leafOf(atT, prefT), leafOf(atE, prefE)
							crSet(leafT, leaf.steps, bad)
							crSet(leafE, leaf.steps, bad)
							gotT := crValidate(t, v, atT)
							gotE := crNormalize(crValidate(t, v, atE), prefE, prefT)
							cases++
							reported = reported || slices.ContainsFunc(gotT, func(s string) bool { return strings.Contains(s, prefT) })
							if !slices.Equal(gotT, gotE) {
								t.Errorf("%s -> %s %v = %#v\n at the referenced element: %q\n under the contentReference: %q", e.ID, target.Def.ID, leaf, bad, gotT, gotE)
							}
						}
						if !reported {
							vacuous++
							t.Errorf("%s -> %s %v: no value is reported at the referenced element", e.ID, target.Def.ID, leaf)
						}
					}
				}
			}
			t.Logf("%s: %d contentReference elements, %d properties, %d cases (%d validations, %d without a report at the referenced element), %s", version, elems, leavesN, cases, 2*cases, vacuous, time.Since(start).Round(time.Millisecond))
			if elems == 0 || cases < 100 {
				t.Errorf("only %d elements and %d cases", elems, cases)
			}
		})
	}
}

// leafOf baja desde root al objeto en fhir (el FHIRPath que devuelve crInstance), donde van las propiedades.
func leafOf(root map[string]any, fhir string) map[string]any {
	var cur any = root
	for _, seg := range strings.Split(fhir, ".")[1:] {
		name, idx := seg, false
		if before, ok := strings.CutSuffix(seg, "[0]"); ok {
			name, idx = before, true
		}
		cur = cur.(map[string]any)[name]
		if idx {
			cur = cur.([]any)[0]
		}
	}
	return cur.(map[string]any)
}

// Casos borde, cada uno contra la regla de que el elemento con contentReference se comporta como el que nombra.
func TestContentReferencePrimitivesEdges(t *testing.T) {
	const (
		obsRoot = `{"resourceType":"Observation","status":"final","code":{"text":"x"},"referenceRange":%s}`
		obsComp = `{"resourceType":"Observation","status":"final","code":{"text":"x"},"component":[{"code":{"text":"y"},"referenceRange":%s}]}`
		tsSetup = `{"resourceType":"TestScript","url":"http://example.org/ts","status":"draft","setup":{"action":[{"assert":%s}]}}`
		tsTest  = `{"resourceType":"TestScript","url":"http://example.org/ts","status":"draft","test":[{"action":[{"assert":%s}]}]}`
		bndLink = `{"resourceType":"Bundle","type":"collection","link":%s}`
		bndEnt  = `{"resourceType":"Bundle","type":"collection","entry":[{"link":%s,"fullUrl":"urn:uuid:3f1c1b4e-2a5f-4c55-9d8e-0d7a8a6f1b00","resource":{"resourceType":"Patient"}}]}`
	)
	same := func(t *testing.T, v *Validator, tmplT, tmplE, value, prefT, prefE string, wantAny bool) {
		t.Helper()
		gotT := crValidateRaw(t, v, fmt.Sprintf(tmplT, value))
		gotE := crNormalize(crValidateRaw(t, v, fmt.Sprintf(tmplE, value)), prefE, prefT)
		if wantAny && len(gotT) == 0 {
			t.Fatalf("the referenced element reports nothing for %s", value)
		}
		if !wantAny && len(gotT) != 0 {
			t.Fatalf("the referenced element reports %q for %s", gotT, value)
		}
		if !slices.Equal(gotT, gotE) {
			t.Errorf("value %s\n at the referenced element: %q\n under the contentReference: %q", value, gotT, gotE)
		}
	}
	for _, version := range crVersions {
		v := crValidator(t, version)
		t.Run(version+"/object where an array is expected", func(t *testing.T) {
			// elementdefinition.html: la referencia da las reglas del contenido; json.html#primitive: el primitivo
			// se revisa donde esté, aquí en un objeto escrito donde va un arreglo.
			same(t, v, obsRoot, obsComp, `{"low":{"value":"1.5"},"text":""}`, "Observation.referenceRange", "Observation.component[0].referenceRange", true)
		})
		t.Run(version+"/array of objects", func(t *testing.T) {
			same(t, v, obsRoot, obsComp, `[{"low":{"value":"1.5"},"text":""},{"high":{"value":true}}]`, "Observation.referenceRange", "Observation.component[0].referenceRange", true)
		})
	}
	v := crValidator(t, "5.0.0")
	t.Run("5.0.0/array under an element of max 1", func(t *testing.T) {
		// Un arreglo donde va un valor es de la fase estructural: no se revisan primitivos dentro, ni en el
		// elemento referido ni bajo la contentReference (como en todo elemento que no se repite).
		same(t, v, tsSetup, tsTest, `[{"label":5}]`, "TestScript.setup.action[0].assert", "TestScript.test[0].action[0].assert", false)
		// El mismo contenido como el único valor que admite el elemento se informa en ambos.
		same(t, v, tsSetup, tsTest, `{"label":5,"direction":7,"resource":"has space"}`, "TestScript.setup.action[0].assert", "TestScript.test[0].action[0].assert", true)
	})
	t.Run("5.0.0/bundle entry link", func(t *testing.T) {
		// Bundle.entry.link es #Bundle.link, en una entrada que además lleva un recurso.
		same(t, v, bndLink, bndEnt, `[{"relation":5,"url":"has space"}]`, "Bundle.link", "Bundle.entry[0].link", true)
		same(t, v, bndLink, bndEnt, `[{"relation":"self","url":"http://example.org/a"}]`, "Bundle.link", "Bundle.entry[0].link", false)
	})
}

func crTestdata(t *testing.T) [][]byte {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "content-reference", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no definitions in testdata/content-reference: %v", err)
	}
	defs := make([][]byte, 0, len(files))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs = append(defs, b)
	}
	return defs
}

// Perfiles cuyos elementos usan contentReference como no lo hacen los recursos del núcleo: "url#id" a un
// elemento de otra StructureDefinition de tipo datatype, una cadena de contentReferences y un destino que
// es un slice (R4). Lo esperado son los formatos de las propiedades (datatypes.html), en el path de la instancia.
func TestContentReferencePrimitivesProfiles(t *testing.T) {
	const profile = "http://example.org/fhir/StructureDefinition/cref-profile"
	v := crValidator(t, "4.0.1", crTestdata(t)...)
	obs := func(extra string) string {
		return `{"resourceType":"Observation","meta":{"profile":["` + profile + `"]},"status":"final","code":{"text":"x"},` + extra + `}`
	}
	tests := []struct {
		name string
		json string
		want []string
	}{
		// (a) url#id a otra definición, a un elemento de tipo Quantity: sus hijos son los del datatype.
		{"datatype target, wrong value", obs(`"viaModel":[{"value":"1.5","unit":3}]`),
			[]string{crWrong + " Observation.viaModel[0].unit", crWrong + " Observation.viaModel[0].value"}},
		{"datatype target, bad format", obs(`"viaModel":[{"system":"has space"}]`),
			[]string{crFormat + " Observation.viaModel[0].system"}},
		{"datatype target, valid", obs(`"viaModel":[{"value":1.5,"unit":"mg","system":"http://unitsofmeasure.org"}]`), nil},
		// (b) cadena: chainA -> chainB -> chainC, que tiene los hijos.
		{"chain, wrong boolean", obs(`"chainA":{"flag":"yes"}`),
			[]string{crWrong + " Observation.chainA.flag"}},
		{"chain, bad dateTime", obs(`"chainA":{"when":"yesterday"}`),
			[]string{crFormat + " Observation.chainA.when"}},
		{"chain, valid", obs(`"chainA":{"flag":true,"when":"2020-01-01"}`), nil},
		// (b') cadena a través de otra definición: chainOut -> model#chainA -> chainB -> chainC.
		{"chain through another definition", obs(`"chainOut":{"flag":"yes"}`),
			[]string{crWrong + " Observation.chainOut.flag"}},
		// (c) el destino es un slice: sus hijos son los del slice.
		{"slice target", obs(`"viaSlice":[{"flag":"yes"}]`),
			[]string{crWrong + " Observation.viaSlice[0].flag"}},
		{"slice target, valid", obs(`"viaSlice":[{"flag":false}]`), nil},
		// Un ciclo de contentReferences (definición mal formada) termina sin visitar nada.
		{"cycle", obs(`"loopA":{"flag":"yes"}`), nil},
		// (d) destino NO cargado, pero el snapshot desenrolla un hijo boolean bajo el elemento: mandan los
		// hijos del propio snapshot, como registry.ChildrenOf (elementdefinition.html#ElementDefinition.contentReference).
		{"unrolled child, target not loaded, wrong", obs(`"missingRef":{"flag":"yes"}`),
			[]string{crWrong + " Observation.missingRef.flag"}},
		{"unrolled child, target not loaded, valid", obs(`"missingRef":{"flag":true}`), nil},
		// (e) el destino carga, pero el elemento tiene un hijo desenrollado string: manda el hijo propio y no el
		// boolean del destino (registry.ChildrenOf; elementdefinition.html#ElementDefinition.contentReference).
		{"unrolled child before the target", obs(`"ownFirst":{"flag":true}`),
			[]string{crWrong + " Observation.ownFirst.flag"}},
		// (f) lo mismo en un salto intermedio: midA -> midB, y midB tiene el hijo propio string antes de
		// seguir a chainC (el orden rige en cada salto, no solo en el primero).
		{"unrolled child at an intermediate hop", obs(`"midA":{"flag":true}`),
			[]string{crWrong + " Observation.midA.flag"}},
		// (g) k0 -> ... -> k8 son registry.MaxContentReferenceHops saltos; k8 no resuelve pero tiene el hijo
		// propio string: se mira antes de la cota, igual que registry.ChildrenOf.
		{"unrolled child at the node of the last hop", obs(`"k0":{"flag":true}`),
			[]string{crWrong + " Observation.k0.flag"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := crValidateRaw(t, v, tt.json); !slices.Equal(got, tt.want) {
				t.Errorf("primitive issues = %q, want %q", got, tt.want)
			}
		})
	}
}
