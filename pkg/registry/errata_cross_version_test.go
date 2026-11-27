package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/fhirpath"

	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/specs"
)

// Errata de un snapshot generado con otra versión que la de su base (R3-P2; plan
// docs/plans/2026-11-27-errata-cross-version.md, §3 v6 y §5).
//
// La regla (plan §2): una constraint heredada se rige por la definición de donde se hereda
// (ElementDefinition.constraint.source: «Reference to original source of constraint»,
// elementdefinition.html#ElementDefinition.constraint.source); la que un perfil declara en su diferencial está escrita
// en la versión del perfil.
//
// Los valores esperados salen de los packages oficiales embebidos (pkg/specs: R4 4.0.1, R4B 4.3.0, R5 5.0.0), leídos
// aquí del JSON publicado, o de la definición oficial que se cita junto a cada uno. Ninguno sale de errata.go, salvo en
// TestErrataCrossVersionAgainstSource, que compara las dos cosas.

const ecvCore = "http://hl7.org/fhir/StructureDefinition/"

// ecvVersions son los releases de FHIR de los core embebidos; ecvProfileVersions, los que un perfil puede declarar
// (vacía: sin declarar, y se corrige con la versión validada).
var (
	ecvVersions        = []string{"4.0.1", "4.3.0", "5.0.0"}
	ecvProfileVersions = []string{"4.0.1", "4.3.0", "5.0.0", ""}
)

// ecvUSCoreVsOne es vs-1 como lo republica US Core 9.0.0 (hl7.fhir.us.core#9.0.0, StructureDefinition/
// us-core-vital-signs, Observation.effective[x], source vitalsigns): aplica solo a un dateTime. R4, R4B y R5 publican
// «($this as dateTime).toString().length() >= 8», que falla en un Period. No está en un package embebido.
const ecvUSCoreVsOne = `$this is dateTime implies $this.toString().length() >= 10`

// --- Los packages oficiales, leídos del JSON publicado.

// ecvOfficialElement es lo que se lee de un elemento del snapshot publicado.
type ecvOfficialElement struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Constraint []struct {
		Key        string `json:"key"`
		Severity   string `json:"severity"`
		Human      string `json:"human"`
		Expression string `json:"expression"`
		Source     string `json:"source"`
	} `json:"constraint"`
	Type []struct {
		Code      string `json:"code"`
		Extension []struct {
			URL      string `json:"url"`
			ValueURL string `json:"valueUrl"`
		} `json:"extension"`
	} `json:"type"`
}

var ecvOfficialCache sync.Map // versión -> func() (map[string]json.RawMessage, error): URL -> StructureDefinition publicada

// ecvOfficialSDs da, por URL, el JSON publicado de las StructureDefinition del package core embebido de version (sin
// ninguna errata: es el archivo del package).
func ecvOfficialSDs(t testing.TB, version string) map[string]json.RawMessage {
	t.Helper()
	load, _ := ecvOfficialCache.LoadOrStore(version, sync.OnceValues(func() (map[string]json.RawMessage, error) {
		packages, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages(version)[:1])
		if err != nil {
			return nil, err
		}
		out := map[string]json.RawMessage{}
		packages[0].EachResource(func(data json.RawMessage, example bool) {
			var peek struct {
				ResourceType string `json:"resourceType"`
				URL          string `json:"url"`
			}
			if !example && json.Unmarshal(data, &peek) == nil && peek.ResourceType == "StructureDefinition" {
				out[peek.URL] = data
			}
		})
		return out, nil
	}))
	sds, err := load.(func() (map[string]json.RawMessage, error))()
	if err != nil {
		t.Fatalf("cannot load the embedded core %s: %v", version, err)
	}
	return sds
}

// ecvOfficialElements da los elementos del snapshot publicado de la definición name del core de version.
func ecvOfficialElements(t testing.TB, version, name string) []ecvOfficialElement {
	t.Helper()
	raw, ok := ecvOfficialSDs(t, version)[ecvCore+name]
	if !ok {
		t.Fatalf("%s is not in the embedded core %s", name, version)
	}
	var sd struct {
		Snapshot struct {
			Element []ecvOfficialElement `json:"element"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(raw, &sd); err != nil {
		t.Fatal(err)
	}
	return sd.Snapshot.Element
}

// ecvOfficialConstraint da la constraint de clave key de la definición name del core de version (exactamente una).
func ecvOfficialConstraint(t testing.TB, version, name, key string) (element, expression, source string) {
	t.Helper()
	n := 0
	for _, e := range ecvOfficialElements(t, version, name) {
		for _, c := range e.Constraint {
			if c.Key == key {
				element, expression, source = e.ID, c.Expression, c.Source
				n++
			}
		}
	}
	if n != 1 {
		t.Fatalf("%s %s: %d constraints %s published, want 1", version, name, n, key)
	}
	return
}

// ecvExpression es la expresión publicada de una constraint de version.
func ecvExpression(t testing.TB, version, name, key string) string {
	t.Helper()
	_, expr, _ := ecvOfficialConstraint(t, version, name, key)
	return expr
}

// ecvOfficialFHIRType da el fhir-type que la definición name de version publica en el tipo del elemento id.
func ecvOfficialFHIRType(t testing.TB, version, name, id string) string {
	t.Helper()
	for _, e := range ecvOfficialElements(t, version, name) {
		if e.ID != id {
			continue
		}
		for _, ty := range e.Type {
			for _, x := range ty.Extension {
				if x.URL == fhirTypeExtension {
					return x.ValueURL
				}
			}
		}
	}
	t.Fatalf("%s %s has no fhir-type on %s", version, name, id)
	return ""
}

// --- Construcción de perfiles y de registros.

// ecvRegistry da un registro con el core de version, que se puede cargar sin cambiar el compartido.
func ecvRegistry(t testing.TB, version string) *Registry {
	t.Helper()
	return sharedVersion(t, version).Clone()
}

func ecvJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// ecvSD escribe una StructureDefinition de restricción; version "" no declara fhirVersion.
func ecvSD(url, version, kind, typ, base string, diff ...string) string {
	var b strings.Builder
	b.WriteString(`{"resourceType":"StructureDefinition","url":` + ecvJSON(url) + `,"name":"Ecv","status":"active"`)
	if version != "" {
		b.WriteString(`,"fhirVersion":` + ecvJSON(version))
	}
	b.WriteString(`,"kind":` + ecvJSON(kind) + `,"abstract":false,"type":` + ecvJSON(typ) + `,"baseDefinition":` + ecvJSON(base))
	b.WriteString(`,"derivation":"constraint","differential":{"element":[` + strings.Join(diff, ",") + `]}}`)
	return b.String()
}

// ecvConstraintJSON escribe una constraint del diferencial.
func ecvConstraintJSON(key, expression string) string {
	return `{"key":` + ecvJSON(key) + `,"severity":"error","human":"x","expression":` + ecvJSON(expression) + `}`
}

// ecvConstraintFrom es ecvConstraintJSON con el source de la constraint (R4B y R5 lo publican).
func ecvConstraintFrom(key, expression, source string) string {
	return `{"key":` + ecvJSON(key) + `,"severity":"error","human":"x","expression":` + ecvJSON(expression) + `,"source":` + ecvJSON(source) + `}`
}

// ecvElementJSON escribe un elemento del diferencial con sus constraints.
func ecvElementJSON(id string, constraints ...string) string {
	return `{"id":` + ecvJSON(id) + `,"path":` + ecvJSON(strings.SplitN(id, ":", 2)[0]) + `,"constraint":[` + strings.Join(constraints, ",") + `]}`
}

// ecvLoad carga definiciones en r por el camino de los packages, el que aplica la errata al cargar.
func ecvLoad(t testing.TB, r *Registry, defs ...string) {
	t.Helper()
	pkg, err := loader.NewLoader("").LoadFromResourcesWithMeta("ecv.test", "1.0.0", asBytes(defs))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
}

func asBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// ecvSnapshot genera el snapshot de la definición url (sin notas: todo el diferencial se ubicó).
func ecvSnapshot(t testing.TB, r *Registry, url string) *StructureDefinition {
	t.Helper()
	sd := r.GetByURL(url)
	if sd == nil {
		t.Fatalf("%s is not loaded", url)
	}
	if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatalf("%s: %v", url, err)
	}
	if n := sd.SnapshotNotes(); len(n) > 0 {
		t.Fatalf("%s leaves differential elements out: %v", url, n)
	}
	return sd
}

// ecvElement da el elemento id del snapshot de sd.
func ecvElement(t testing.TB, sd *StructureDefinition, id string) *ElementDefinition {
	t.Helper()
	for i := range sd.Snapshot.Element {
		if sd.Snapshot.Element[i].ID == id {
			return &sd.Snapshot.Element[i]
		}
	}
	t.Fatalf("%s has no element %s", sd.URL, id)
	return nil
}

// ecvWith da los elementos del snapshot de sd cuyo id empieza con prefix y que tienen la constraint key.
func ecvWith(t testing.TB, sd *StructureDefinition, prefix, key string) []*ElementDefinition {
	t.Helper()
	var out []*ElementDefinition
	for i := range sd.Snapshot.Element {
		e := &sd.Snapshot.Element[i]
		if strings.HasPrefix(e.ID, prefix) && constraintKeys(e)[key] {
			out = append(out, e)
		}
	}
	return out
}

// ecvConstraintOf da la constraint de clave key de e (exactamente una).
func ecvConstraintOf(t testing.TB, e *ElementDefinition, key string) Constraint {
	t.Helper()
	var found []Constraint
	for _, c := range e.Constraint {
		if c.Key == key {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s: %d constraints %s, want 1 (have %v)", e.ID, len(found), key, constraintKeys(e))
	}
	return found[0]
}

// ecvFHIRType da el fhir-type del primer tipo de e.
func ecvFHIRType(t testing.TB, e *ElementDefinition) string {
	t.Helper()
	for _, ty := range e.Type {
		for _, x := range ty.Extension {
			if x.URL == fhirTypeExtension {
				return x.ValueURL
			}
		}
	}
	t.Fatalf("%s has no fhir-type", e.ID)
	return ""
}

// ecvRawConstraint da la constraint de clave key en el raw de e (nil si no está).
func ecvRawConstraint(t testing.TB, e *ElementDefinition, key string) map[string]json.RawMessage {
	t.Helper()
	m, err := rawToMap(e.raw)
	if err != nil {
		t.Fatal(err)
	}
	var cs []map[string]json.RawMessage
	if m["constraint"] != nil {
		if err := json.Unmarshal(m["constraint"], &cs); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range cs {
		var k string
		_ = json.Unmarshal(c["key"], &k)
		if k == key {
			return c
		}
	}
	return nil
}

// ecvRawExpression es la expresión de la constraint key en el raw de e.
func ecvRawExpression(t testing.TB, e *ElementDefinition, key string) string {
	t.Helper()
	c := ecvRawConstraint(t, e, key)
	if c == nil {
		t.Fatalf("%s: the raw has no constraint %s", e.ID, key)
	}
	var s string
	_ = json.Unmarshal(c["expression"], &s)
	return s
}

// ecvAgree exige que el struct y el raw de e digan lo mismo de la constraint key: la expresión esperada.
func ecvAgree(t testing.TB, e *ElementDefinition, key, want string) {
	t.Helper()
	if got := ecvConstraintOf(t, e, key).Expression; got != want {
		t.Errorf("%s %s (struct):\n got %s\nwant %s", e.ID, key, got, want)
	}
	if got := ecvRawExpression(t, e, key); got != want {
		t.Errorf("%s %s (raw):\n got %s\nwant %s", e.ID, key, got, want)
	}
}

func ecvTag(versions []string) string {
	parts := make([]string, len(versions))
	for i, v := range versions {
		parts[i] = cmpOr(v, "none")
	}
	return strings.Join(parts, "-")
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ecvTrueOn dice si la expresión da true sobre el recurso JSON data.
func ecvTrueOn(t testing.TB, data, expression string) (bool, error) {
	t.Helper()
	got, err := fhirpath.Evaluate([]byte(data), expression)
	if err != nil {
		return false, err
	}
	v, ok := got.SingletonBoolean()
	return ok && v, nil
}

// --- A1: cadenas core -> perfil -> perfil con releases mezclados.

// ecvChainCase es una constraint heredada que una cadena de perfiles de cualquier mezcla de releases conserva
// corregida, con su source.
type ecvChainCase struct {
	name    string
	sd      string // la definición core de la que parte la cadena
	typ     string
	kind    string
	element string // el elemento que lleva la constraint
	key     string
	cores   []string
	depth   int
	// want da la expresión esperada: la de la definición oficial que la corrige, o que la publica ya corregida.
	want func(t testing.TB) string
}

var ecvChainCases = []ecvChainCase{{
	// ref-1: R4 y R4B publican su ref-1 sin «reference.exists() implies»; R5 lo publica con él
	// (hl7.fhir.r5.core#5.0.0 StructureDefinition/Reference, ref-1). Heredada de cualquier core, queda la de R5.
	name: "ref-1", sd: "Reference", typ: "Reference", kind: "complex-type", element: "Reference", key: "ref-1",
	cores: ecvVersions, depth: 3,
	want: func(t testing.TB) string { return ecvExpression(t, "5.0.0", "Reference", "ref-1") },
}, {
	// tim-9: R4 y R4B tratan when, que se repite, con in; R5 prueba cada when (hl7.fhir.r5.core#5.0.0
	// StructureDefinition/Timing, tim-9).
	name: "tim-9", sd: "Timing", typ: "Timing", kind: "complex-type", element: "Timing.repeat", key: "tim-9",
	cores: ecvVersions, depth: 2,
	want: func(t testing.TB) string { return ecvExpression(t, "5.0.0", "Timing", "tim-9") },
}, {
	// bdl-8: R4 falla en una entry sin fullUrl; R4B y R5 la aplican solo donde hay una (hl7.fhir.r4b.core#4.3.0
	// StructureDefinition/Bundle, bdl-8).
	name: "bdl-8", sd: "Bundle", typ: "Bundle", kind: "resource", element: "Bundle.entry", key: "bdl-8",
	cores: ecvVersions, depth: 3,
	want: func(t testing.TB) string { return ecvExpression(t, "4.3.0", "Bundle", "bdl-8") },
}, {
	// bdl-15: R4 y R4B lo dicen solo en prosa (bundle.html#bundle-unique); R5 lo publica como invariante
	// (hl7.fhir.r5.core#5.0.0 StructureDefinition/Bundle, bdl-15). Todo perfil de Bundle lo trae.
	name: "bdl-15", sd: "Bundle", typ: "Bundle", kind: "resource", element: "Bundle", key: "bdl-15",
	cores: ecvVersions, depth: 3,
	want: func(t testing.TB) string { return ecvExpression(t, "5.0.0", "Bundle", "bdl-15") },
}, {
	name: "vs-1", sd: "vitalsigns", typ: "Observation", kind: "resource", element: "Observation.effective[x]", key: "vs-1",
	cores: ecvVersions, depth: 2,
	want: func(testing.TB) string { return ecvUSCoreVsOne },
}, {
	// csd-0: R4 la publica sin «name.exists() implies»; R4B la aplica solo donde hay un name (hl7.fhir.r4b.core#4.3.0
	// StructureDefinition/CodeSystem, csd-0). R5 no la publica con esta clave (cnl-0).
	name: "csd-0", sd: "CodeSystem", typ: "CodeSystem", kind: "resource", element: "CodeSystem", key: "csd-0",
	cores: []string{"4.0.1", "4.3.0"}, depth: 2,
	want: func(t testing.TB) string { return ecvExpression(t, "4.3.0", "CodeSystem", "csd-0") },
}}

// ecvVersionTuples da todas las secuencias de n releases de perfil.
func ecvVersionTuples(n int) [][]string {
	out := [][]string{{}}
	for i := 0; i < n; i++ {
		next := make([][]string, 0, len(ecvProfileVersions)*len(out))
		for _, p := range out {
			for _, v := range ecvProfileVersions {
				next = append(next, append(slices.Clone(p), v))
			}
		}
		out = next
	}
	return out
}

// ecvChain carga una cadena de perfiles de la definición core, uno por versión, cada uno derivado del anterior.
func ecvChain(t testing.TB, r *Registry, c ecvChainCase, prefix string, versions []string) []*StructureDefinition {
	t.Helper()
	defs := make([]string, 0, len(versions))
	base := ecvCore + c.sd
	for i, v := range versions {
		url := fmt.Sprintf("http://example.org/ecv/%s-%s-%d", prefix, ecvTag(versions), i)
		defs = append(defs, ecvSD(url, v, c.kind, c.typ, base, `{"id":`+ecvJSON(c.typ)+`,"path":`+ecvJSON(c.typ)+`,"short":"s"}`))
		base = url
	}
	ecvLoad(t, r, defs...)
	out := make([]*StructureDefinition, len(versions))
	for i := range versions {
		out[i] = ecvSnapshot(t, r, fmt.Sprintf("http://example.org/ecv/%s-%s-%d", prefix, ecvTag(versions), i))
	}
	return out
}

// ecvCheckChain exige que toda definición de la cadena traiga la constraint corregida, con su source.
func ecvCheckChain(t *testing.T, c ecvChainCase, core string, chain []*StructureDefinition) {
	t.Helper()
	want := c.want(t)
	for i, sd := range chain {
		e := ecvElement(t, sd, c.element)
		cn := ecvConstraintOf(t, e, c.key)
		if cn.Expression != want {
			t.Errorf("link %d (%s): %s %s expression\n got %s\nwant %s", i, cmpOr(sd.FHIRVersion, "no version"), c.element, c.key, cn.Expression, want)
		}
		// El source original de la constraint es la definición core que la publica
		// (elementdefinition.html#ElementDefinition.constraint.source); R4 no publica sources.
		if wantSrc := ecvCore + c.sd; cn.Source != wantSrc {
			t.Errorf("link %d (%s): %s %s source %q, want %q", i, cmpOr(sd.FHIRVersion, "no version"), c.element, c.key, cn.Source, wantSrc)
		}
		// El raw corregido es el que heredan las copias de la siguiente generación; bdl-15 se agrega solo al struct.
		if c.key == "bdl-15" {
			if core != "5.0.0" && ecvRawConstraint(t, e, c.key) != nil {
				t.Errorf("link %d: bdl-15, which core %s does not publish, is in the raw", i, core)
			}
			continue
		}
		if got := ecvRawExpression(t, e, c.key); got != want {
			t.Errorf("link %d (%s): %s %s raw expression\n got %s\nwant %s", i, cmpOr(sd.FHIRVersion, "no version"), c.element, c.key, got, want)
		}
	}
}

// Una constraint heredada, de la definición core de cualquier versión por una cadena de perfiles de cualquier mezcla
// de releases (también sin declarar), queda corregida y con su source (A1). Plan §2: se rige por la definición de
// donde se hereda.
func TestErrataCrossVersionA1Chains(t *testing.T) {
	for _, c := range ecvChainCases {
		for _, core := range c.cores {
			r := ecvRegistry(t, core)
			for _, versions := range ecvVersionTuples(c.depth) {
				t.Run(c.name+"/core "+core+"/"+ecvTag(versions), func(t *testing.T) {
					ecvCheckChain(t, c, core, ecvChain(t, r, c, "a1-"+c.name+"-"+core, versions))
				})
			}
		}
	}
}

// El clon de un registro regenera sus perfiles desde su JSON publicado (clone.go): la corrección llega igual.
func TestErrataCrossVersionA1Clone(t *testing.T) {
	c := ecvChainCases[0]
	for _, core := range ecvVersions {
		r := ecvRegistry(t, core)
		versions := []string{"4.0.1", "4.3.0", "5.0.0"}
		chain := ecvChain(t, r, c, "clone-"+core, versions)
		clone := r.Clone()
		for i, sd := range chain {
			cs := clone.GetByURL(sd.URL)
			if cs == nil || cs == sd {
				t.Fatalf("core %s link %d: the clone does not hold its own copy", core, i)
			}
		}
		for i := range chain {
			cs := ecvSnapshot(t, clone, chain[i].URL)
			t.Run(fmt.Sprintf("core %s link %d", core, i), func(t *testing.T) {
				ecvCheckChain(t, c, core, []*StructureDefinition{cs})
			})
		}
	}
}

// --- A2, D2, E2: la procedencia de una constraint.

// Una constraint que un perfil declara en su diferencial está escrita en la versión del perfil: la errata de
// otra versión no la reescribe aunque su expresión sea la publicada de ese otro (A2); csd-0 en R4 («name.matches(...)»,
// hl7.fhir.r4.core#4.0.1 StructureDefinition/CodeSystem) falla sin name; en un perfil R4B, la misma expresión es lo que
// el perfil escribió.
func TestErrataCrossVersionA2OwnConstraintKeepsItsVersion(t *testing.T) {
	published := ecvExpression(t, "4.0.1", "CodeSystem", "csd-0")
	corrected := ecvExpression(t, "4.3.0", "CodeSystem", "csd-0")
	for _, tt := range []struct {
		version, want string
	}{
		{"4.3.0", published}, // escrita en R4B: es lo que el perfil dice
		{"5.0.0", published}, // ídem R5
		{"4.0.1", corrected}, // escrita en R4: es el texto defectuoso de R4
		{"4.0.0", corrected}, // 4.0.0 es R4 también
		{"", corrected},      // sin versión se corrige con la validada (R4)
	} {
		t.Run(cmpOr(tt.version, "none"), func(t *testing.T) {
			r := ecvRegistry(t, "4.0.1")
			url := "http://example.org/ecv/a2-" + cmpOr(tt.version, "none")
			ecvLoad(t, r, ecvSD(url, tt.version, "resource", "CodeSystem", ecvCore+"CodeSystem",
				ecvElementJSON("CodeSystem", ecvConstraintJSON("own-name", published))))
			sd := ecvSnapshot(t, r, url)
			ecvAgree(t, ecvElement(t, sd, "CodeSystem"), "own-name", tt.want)
		})
	}
}

// D2: una guía R4B declara una constraint con la expresión publicada de csd-0 en R4; un perfil R4 que la deriva y solo
// cambia min la hereda sin que la errata de R4 la toque, y {} sigue sin cumplirla (la expresión no es true sobre un
// CodeSystem sin name, como la publica R4B en su propia guía).
func TestErrataCrossVersionD2InheritedFromAGuide(t *testing.T) {
	published := ecvExpression(t, "4.0.1", "CodeSystem", "csd-0")
	r := ecvRegistry(t, "4.0.1")
	guide := "http://example.org/ecv/d2-guide"
	derived := "http://example.org/ecv/d2-derived"
	ecvLoad(t, r,
		ecvSD(guide, "4.3.0", "resource", "CodeSystem", ecvCore+"CodeSystem",
			ecvElementJSON("CodeSystem", ecvConstraintJSON("required-name", published))),
		ecvSD(derived, "4.0.1", "resource", "CodeSystem", guide, `{"id":"CodeSystem.url","path":"CodeSystem.url","min":1}`))
	for _, url := range []string{guide, derived} {
		sd := ecvSnapshot(t, r, url)
		ecvAgree(t, ecvElement(t, sd, "CodeSystem"), "required-name", published)
	}
	got := ecvConstraintOf(t, ecvElement(t, ecvSnapshot(t, r, derived), "CodeSystem"), "required-name").Expression
	if ok, err := ecvTrueOn(t, `{"resourceType":"CodeSystem","status":"active","content":"complete"}`, got); err != nil || ok {
		t.Errorf("a CodeSystem with no name satisfies the inherited %s: %v %v", got, ok, err)
	}
}

// E2: un diferencial R4 que redeclara la clave que hereda de una guía R4B, con la misma expresión (HL7 exige que una
// misma clave sea la misma constraint: «A constraint with the same key must be the same constraint»), no cambia la
// heredada. El perfil que la redeclara con otra expresión no es válido y queda fuera del alcance (plan §3).
func TestErrataCrossVersionE2RedeclaredKeyKeepsTheInherited(t *testing.T) {
	published := ecvExpression(t, "4.0.1", "CodeSystem", "csd-0")
	r := ecvRegistry(t, "4.0.1")
	guide := "http://example.org/ecv/e2-guide"
	derived := "http://example.org/ecv/e2-derived"
	ecvLoad(t, r,
		ecvSD(guide, "4.3.0", "resource", "CodeSystem", ecvCore+"CodeSystem",
			ecvElementJSON("CodeSystem", ecvConstraintJSON("required-name", published))),
		ecvSD(derived, "4.0.1", "resource", "CodeSystem", guide,
			ecvElementJSON("CodeSystem", ecvConstraintJSON("required-name", published))))
	ecvAgree(t, ecvElement(t, ecvSnapshot(t, r, derived), "CodeSystem"), "required-name", published)
}

// --- B1: el diferencial sin base.path.

// Un Patient.id R4 que redeclara el tipo sin base.path queda con el fhir-type id que R5 publica para Resource.id
// (hl7.fhir.r5.core#5.0.0 StructureDefinition/Resource, Resource.id; resource.html: «id : id»).
func TestErrataCrossVersionB1DifferentialRedeclaresTheType(t *testing.T) {
	want := ecvOfficialFHIRType(t, "5.0.0", "Resource", "Resource.id")
	if want != "id" {
		t.Fatalf("R5 publishes Resource.id as %q", want)
	}
	r := ecvRegistry(t, "4.0.1")
	url := "http://example.org/ecv/b1-id"
	ecvLoad(t, r, ecvSD(url, "4.0.1", "resource", "Patient", ecvCore+"Patient",
		`{"id":"Patient.id","path":"Patient.id","type":[{"code":"http://hl7.org/fhirpath/System.String","extension":[{"url":`+ecvJSON(fhirTypeExtension)+`,"valueUrl":"string"}]}]}`))
	e := ecvElement(t, ecvSnapshot(t, r, url), "Patient.id")
	if got := ecvFHIRType(t, e); got != want {
		t.Errorf("Patient.id fhir-type %s, want %s", got, want)
	}
	if got := ecvRawFHIRType(t, e); got != want {
		t.Errorf("Patient.id fhir-type in the raw %s, want %s", got, want)
	}
}

// ecvRawFHIRType es el valueUrl del fhir-type en el raw de e.
func ecvRawFHIRType(t testing.TB, e *ElementDefinition) string {
	t.Helper()
	var obj struct {
		Type []struct {
			Extension []struct {
				URL      string `json:"url"`
				ValueURL string `json:"valueUrl"`
			} `json:"extension"`
		} `json:"type"`
	}
	if err := json.Unmarshal(e.raw, &obj); err != nil {
		t.Fatal(err)
	}
	for _, ty := range obj.Type {
		for _, x := range ty.Extension {
			if x.URL == fhirTypeExtension {
				return x.ValueURL
			}
		}
	}
	t.Fatalf("%s: no fhir-type in the raw", e.ID)
	return ""
}

// Observation.effectiveDateTime, un elemento que el diferencial renombra a partir de effective[x] y que trae vs-1 en
// su texto de R4, queda con el vs-1 que US Core 9.0.0 republica corregido. La errata se identifica con el elemento
// de la base en esa posición, porque el diferencial no trae base.path (el path es otro: effectiveDateTime).
func TestErrataCrossVersionB1RenamedChoiceKeepsItsErratum(t *testing.T) {
	published := ecvExpression(t, "4.0.1", "vitalsigns", "vs-1")
	r := ecvRegistry(t, "4.0.1")
	url := "http://example.org/ecv/b1-vs"
	ecvLoad(t, r, ecvSD(url, "4.0.1", "resource", "Observation", ecvCore+"Observation",
		`{"id":"Observation.effectiveDateTime","path":"Observation.effectiveDateTime","type":[{"code":"dateTime"}],"constraint":[`+
			ecvConstraintJSON("vs-1", published)+`]}`))
	sd := ecvSnapshot(t, r, url)
	found := ecvWith(t, sd, "Observation.effective", "vs-1")
	if len(found) == 0 {
		t.Fatal("no element under Observation.effective carries vs-1")
	}
	for _, e := range found {
		ecvAgree(t, e, "vs-1", ecvUSCoreVsOne)
	}
}

// --- B4: el parche toca solo la hoja.

// ecvDiffs lista las rutas en que dos valores JSON decodificados difieren (el orden de las claves no cuenta).
func ecvDiffs(path string, a, b any, out *[]string) {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			*out = append(*out, path)
			return
		}
		keys := map[string]bool{}
		for k := range av {
			keys[k] = true
		}
		for k := range bv {
			keys[k] = true
		}
		for k := range keys {
			ecvDiffs(path+"."+k, av[k], bv[k], out)
		}
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			*out = append(*out, path)
			return
		}
		for i := range av {
			ecvDiffs(fmt.Sprintf("%s[%d]", path, i), av[i], bv[i], out)
		}
	default:
		if !reflect.DeepEqual(a, b) {
			*out = append(*out, path)
		}
	}
}

var ecvLeaf = regexp.MustCompile(`\.(constraint\[\d+\]\.expression|type\[\d+\]\.extension\[\d+\]\.valueUrl)$`)

// Un perfil que escribe una constraint con miembros que el modelo no conoce (xpath, extension, _expression, source) y
// un tipo con otras claves de extensión, y un elemento con fixed[x]: al cargarlo, la errata cambia el miembro expression y el
// valueUrl, y todo otro miembro conserva su valor JSON (B4). El raw y el struct coinciden en la hoja corregida.
func TestErrataCrossVersionB4LeafPatch(t *testing.T) {
	published := ecvExpression(t, "4.0.1", "Reference", "ref-1")
	corrected := ecvExpression(t, "5.0.0", "Reference", "ref-1")
	refElement := `{"id":"Reference","path":"Reference","fixedDecimal":1.50,"_fixedDecimal":{"extension":[{"url":"http://example.org/x","valueString":"keep"}]},` +
		`"patternString":"p","constraint":[{"key":"ref-1","severity":"error","human":"h","expression":` + ecvJSON(published) + `,"xpath":"keep",` +
		`"source":"http://example.org/own","extension":[{"url":"http://example.org/x","valueBoolean":true}],"_expression":{"id":"keep"}}]}`
	idElement := `{"id":"Resource.id","path":"Resource.id","type":[{"code":"http://hl7.org/fhirpath/System.String","extension":[` +
		`{"url":` + ecvJSON(fhirTypeExtension) + `,"valueUrl":"string","_valueUrl":{"id":"keep"}},{"url":"http://example.org/other","valueString":"keep"}]}]}`
	data := ecvSD("http://example.org/ecv/b4", "4.0.1", "complex-type", "Reference", ecvCore+"Reference", refElement, idElement)
	r := ecvRegistry(t, "4.0.1")
	ecvLoad(t, r, data)
	sd := r.GetByURL("http://example.org/ecv/b4")
	var orig struct {
		Differential struct {
			Element []json.RawMessage `json:"element"`
		} `json:"differential"`
	}
	if err := json.Unmarshal([]byte(data), &orig); err != nil {
		t.Fatal(err)
	}
	for i, e := range sd.Differential.Element {
		var before, after any
		_ = json.Unmarshal(orig.Differential.Element[i], &before)
		if err := json.Unmarshal(e.raw, &after); err != nil {
			t.Fatalf("%s: the raw is not JSON: %v", e.ID, err)
		}
		var diffs []string
		ecvDiffs("", before, after, &diffs)
		sort.Strings(diffs)
		if len(diffs) == 0 {
			t.Errorf("%s: nothing was corrected", e.ID)
		}
		for _, d := range diffs {
			if !ecvLeaf.MatchString(d) {
				t.Errorf("%s: the patch changed %s, which is no corrected leaf", e.ID, d)
			}
		}
		if len(diffs) > 1 {
			t.Errorf("%s: %d leaves changed, want 1: %v", e.ID, len(diffs), diffs)
		}
	}
	re := &sd.Differential.Element[0]
	ecvAgree(t, re, "ref-1", corrected)
	// El decimal conserva su precisión (json.html#primitive: 1.50 no es 1.5), y la caché de fixed[x] sigue siendo la del raw.
	if v, suffix, ok := re.GetFixed(); !ok || suffix != "Decimal" || string(v) != "1.50" {
		t.Errorf("fixed %s %q %v, want 1.50 Decimal", v, suffix, ok)
	}
	if re.cachedPrefixed() == nil {
		t.Error("the prefixed-values cache was not kept for the patched raw (F1)")
	}
	if got := ecvRawFHIRType(t, &sd.Differential.Element[1]); got != "id" {
		t.Errorf("Resource.id fhir-type in the raw %s, want id", got)
	}
	if got := ecvFHIRType(t, &sd.Differential.Element[1]); got != "id" {
		t.Errorf("Resource.id fhir-type (struct) %s, want id", got)
	}
}

// Los core publicados, con la errata aplicada, difieren del JSON del package solo en las hojas corregidas
// (constraint[].expression y type[].extension[].valueUrl); en cada una el raw y el struct coinciden; y el struct solo
// tiene de más la constraint agregada (bdl-15). Ni las claves reordenadas ni ningún otro miembro cambian.
func TestErrataCrossVersionRawOnlyDiffersInTheCorrectedLeaves(t *testing.T) {
	for _, version := range ecvVersions {
		t.Run(version, func(t *testing.T) {
			r := sharedVersion(t, version)
			official := ecvOfficialSDs(t, version)
			leaves, checked := 0, 0
			for url, rawSD := range official {
				sd := r.GetByURL(url)
				if sd == nil || sd.Snapshot == nil || sd.PackageID == "" {
					continue
				}
				// Solo las definiciones que el registro tomó del core: un package de extensión (p. ej.
				// hl7.fhir.uv.extensions.r4) publica otra definición con la misma URL, y esa no se compara con el core.
				if !strings.Contains(sd.PackageID, ".core") {
					continue
				}
				if testing.Short() && !strings.Contains(url, "StructureDefinition/") {
					continue
				}
				var pub struct {
					Snapshot struct {
						Element []json.RawMessage `json:"element"`
					} `json:"snapshot"`
				}
				if err := json.Unmarshal(rawSD, &pub); err != nil {
					t.Fatal(err)
				}
				if len(pub.Snapshot.Element) != len(sd.Snapshot.Element) {
					t.Errorf("%s: %d elements, the package has %d", url, len(sd.Snapshot.Element), len(pub.Snapshot.Element))
					continue
				}
				for i := range pub.Snapshot.Element {
					e := &sd.Snapshot.Element[i]
					var before, after any
					_ = json.Unmarshal(pub.Snapshot.Element[i], &before)
					if err := json.Unmarshal(e.raw, &after); err != nil {
						t.Fatalf("%s %s: %v", url, e.ID, err)
					}
					var diffs []string
					ecvDiffs("", before, after, &diffs)
					checked++
					for _, d := range diffs {
						leaves++
						if !ecvLeaf.MatchString(d) {
							t.Errorf("%s %s: the raw differs from the package at %s", url, e.ID, d)
						}
					}
					// raw y struct: las mismas constraints en el mismo orden; el struct puede tener las agregadas al final.
					var rc struct {
						Constraint []struct{ Key, Expression string } `json:"constraint"`
					}
					_ = json.Unmarshal(e.raw, &rc)
					if len(e.Constraint) < len(rc.Constraint) {
						t.Errorf("%s %s: %d constraints in the struct, %d in the raw", url, e.ID, len(e.Constraint), len(rc.Constraint))
						continue
					}
					for j, c := range rc.Constraint {
						if e.Constraint[j].Key != c.Key || e.Constraint[j].Expression != c.Expression {
							t.Errorf("%s %s constraint %s: struct and raw disagree\n struct %s\n    raw %s", url, e.ID, c.Key, e.Constraint[j].Expression, c.Expression)
						}
					}
					for _, c := range e.Constraint[len(rc.Constraint):] {
						if c.Key != "bdl-15" {
							t.Errorf("%s %s: constraint %s only in the struct", url, e.ID, c.Key)
						}
					}
				}
			}
			if checked == 0 {
				t.Fatal("no element checked")
			}
			t.Logf("%d elements, %d corrected leaves", checked, leaves)
		})
	}
}

// --- C1, C2: lo que se copia de otra definición.

// Un diferencial que solo cambia min en Patient.id hereda el tipo corregido (C1): el fhir-type que R4B y R5 publican
// para Resource.id es id (hl7.fhir.r5.core#5.0.0 StructureDefinition/Resource, Resource.id).
func TestErrataCrossVersionC1InheritedType(t *testing.T) {
	for _, version := range []string{"4.0.1", "4.0.0", ""} {
		t.Run(cmpOr(version, "none"), func(t *testing.T) {
			r := ecvRegistry(t, "4.0.1")
			url := "http://example.org/ecv/c1-" + cmpOr(version, "none")
			ecvLoad(t, r, ecvSD(url, version, "resource", "Patient", ecvCore+"Patient", `{"id":"Patient.id","path":"Patient.id","min":1}`))
			e := ecvElement(t, ecvSnapshot(t, r, url), "Patient.id")
			if got := ecvFHIRType(t, e); got != "id" {
				t.Errorf("Patient.id fhir-type %s, want id", got)
			}
			if got := ecvRawFHIRType(t, e); got != "id" {
				t.Errorf("Patient.id fhir-type in the raw %s, want id", got)
			}
		})
	}
}

// ecvTimingInstance es un Timing con un offset y dos when, que tim-9 de R4 y R4B no puede evaluar y R5 sí.
const ecvTimingInstance = `{"offset":1,"when":["MORN","EVE"]}`

// Un perfil R5 que despliega un Timing escrito en R4 (un Dosage que restringe timing con el perfil de tipo y su
// repeat) hereda el tim-9 corregido (C2). La instancia {"offset":1,"when":["MORN","EVE"]} da el mismo resultado que con
// el tim-9 de R5 (hl7.fhir.r5.core#5.0.0 StructureDefinition/Timing, tim-9): true. Con el publicado de R4, un error.
func TestErrataCrossVersionC2UnrolledFromAnotherVersion(t *testing.T) {
	corrected := ecvExpression(t, "5.0.0", "Timing", "tim-9")
	published := ecvExpression(t, "4.0.1", "Timing", "tim-9")
	if ok, err := ecvTrueOn(t, ecvTimingInstance, corrected); err != nil || !ok {
		t.Fatalf("the R5 tim-9 on the instance: %v %v", ok, err)
	}
	if _, err := ecvTrueOn(t, ecvTimingInstance, published); err == nil {
		t.Fatal("the R4 tim-9 evaluates the instance: the scenario proves nothing")
	}
	for _, profileVersion := range []string{"5.0.0", "4.3.0", ""} {
		t.Run(cmpOr(profileVersion, "none"), func(t *testing.T) {
			r := ecvRegistry(t, "4.0.1")
			tp := "http://example.org/ecv/c2-timing-" + cmpOr(profileVersion, "none")
			host := "http://example.org/ecv/c2-dosage-" + cmpOr(profileVersion, "none")
			ecvLoad(t, r,
				ecvSD(tp, "4.0.1", "complex-type", "Timing", ecvCore+"Timing", `{"id":"Timing","path":"Timing","short":"r4 timing"}`),
				ecvSD(host, profileVersion, "complex-type", "Dosage", ecvCore+"Dosage",
					`{"id":"Dosage.timing","path":"Dosage.timing","type":[{"code":"Timing","profile":[`+ecvJSON(tp)+`]}]}`,
					`{"id":"Dosage.timing.repeat","path":"Dosage.timing.repeat","min":1}`))
			e := ecvElement(t, ecvSnapshot(t, r, host), "Dosage.timing.repeat")
			ecvAgree(t, e, "tim-9", corrected)
			got := ecvConstraintOf(t, e, "tim-9").Expression
			if ok, err := ecvTrueOn(t, ecvTimingInstance, got); err != nil || !ok {
				t.Errorf("tim-9 on %s: %v %v", ecvTimingInstance, ok, err)
			}
		})
	}
}

// Lo mismo cuando el hijo se despliega por una contentReference: una guía R5 que restringe Questionnaire.item.item,
// donde enableWhen (que copia los hijos de Questionnaire.item) hereda el que-7 de R4B (hl7.fhir.r4b.core#4.3.0
// StructureDefinition/Questionnaire, Questionnaire.item.enableWhen, que-7: «answer is boolean»), no el de R4 («Boolean»).
func TestErrataCrossVersionC2ContentReference(t *testing.T) {
	want := ecvExpression(t, "4.3.0", "Questionnaire", "que-7")
	for _, version := range []string{"5.0.0", "4.3.0", "4.0.1", ""} {
		t.Run(cmpOr(version, "none"), func(t *testing.T) {
			r := ecvRegistry(t, "4.0.1")
			url := "http://example.org/ecv/c2-que-" + cmpOr(version, "none")
			ecvLoad(t, r, ecvSD(url, version, "resource", "Questionnaire", ecvCore+"Questionnaire",
				`{"id":"Questionnaire.item.item.enableWhen","path":"Questionnaire.item.item.enableWhen","min":1}`))
			e := ecvElement(t, ecvSnapshot(t, r, url), "Questionnaire.item.item.enableWhen")
			ecvAgree(t, e, "que-7", want)
		})
	}
}

// --- D1: la constraint agregada y la que el perfil declara.

// ecvBundleDefinitions carga un perfil de Bundle con versión version, que declara bdl-15 con expression (o no la
// declara con ""), sobre base.
func ecvBundleDefinition(url, version, base, bdl15 string) string {
	diff := `{"id":"Bundle","path":"Bundle"}`
	if bdl15 != "" {
		diff = ecvElementJSON("Bundle", ecvConstraintJSON("bdl-15", bdl15))
	}
	return ecvSD(url, version, "resource", "Bundle", base, diff)
}

// Un perfil R4 de Bundle que declara su propio bdl-15 con otra expresión la conserva; uno que no la declara hereda la
// que R5 publica como invariante (hl7.fhir.r5.core#5.0.0 StructureDefinition/Bundle, bdl-15), también en una cadena
// R4 -> R5 -> R5, y tampoco la pierde el que la hereda por una base de otra versión (D1, A1).
func TestErrataCrossVersionD1AddedAndDeclaredBdl15(t *testing.T) {
	r5 := ecvExpression(t, "5.0.0", "Bundle", "bdl-15")
	own := "type = 'collection' or entry.all(fullUrl.exists())"
	for _, core := range []string{"4.0.1", "4.3.0"} {
		t.Run("core "+core, func(t *testing.T) {
			r := ecvRegistry(t, core)
			p := func(n string) string { return "http://example.org/ecv/d1-" + core + "-" + n }
			ecvLoad(t, r,
				ecvBundleDefinition(p("own"), "4.0.1", ecvCore+"Bundle", own),
				ecvBundleDefinition(p("inherits"), "4.0.1", ecvCore+"Bundle", ""),
				ecvBundleDefinition(p("mid"), "5.0.0", p("inherits"), ""),
				ecvBundleDefinition(p("leaf"), "5.0.0", p("mid"), ""),
				ecvBundleDefinition(p("own-mid"), "5.0.0", p("own"), ""),
				ecvBundleDefinition(p("own-leaf"), "5.0.0", p("own-mid"), ""),
			)
			for _, tt := range []struct{ name, want string }{
				{"own", own}, {"inherits", r5}, {"mid", r5}, {"leaf", r5}, {"own-mid", own}, {"own-leaf", own},
			} {
				e := ecvElement(t, ecvSnapshot(t, r, p(tt.name)), "Bundle")
				var got []string
				for _, c := range e.Constraint {
					if c.Key == "bdl-15" {
						got = append(got, c.Expression)
					}
				}
				if len(got) != 1 || got[0] != tt.want {
					t.Errorf("%s: bdl-15 %q\nwant %q", tt.name, got, tt.want)
				}
			}
		})
	}
}

// --- E1: las copias de la misma generación.

// Un Patient.contained.id R4 que redeclara el tipo sin base.path, y un slice de contained que solo cambia min: los dos
// quedan con el fhir-type id (hl7.fhir.r5.core#5.0.0 StructureDefinition/Resource, Resource.id). El slice copia del
// diferencial ya ubicado (E1).
func TestErrataCrossVersionE1SliceCopiesTheCorrectedType(t *testing.T) {
	r := ecvRegistry(t, "4.0.1")
	url := "http://example.org/ecv/e1-contained"
	ecvLoad(t, r, ecvSD(url, "4.0.1", "resource", "Patient", ecvCore+"Patient",
		`{"id":"Patient.contained.id","path":"Patient.contained.id","type":[{"code":"http://hl7.org/fhirpath/System.String","extension":[{"url":`+ecvJSON(fhirTypeExtension)+`,"valueUrl":"string"}]}]}`,
		`{"id":"Patient.contained","path":"Patient.contained","slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"open"}}`,
		`{"id":"Patient.contained:mine","path":"Patient.contained","sliceName":"mine"}`,
		`{"id":"Patient.contained:mine.id","path":"Patient.contained.id","min":0}`))
	sd := ecvSnapshot(t, r, url)
	for _, id := range []string{"Patient.contained.id", "Patient.contained:mine.id"} {
		e := ecvElement(t, sd, id)
		if got := ecvFHIRType(t, e); got != "id" {
			t.Errorf("%s fhir-type %s, want id", id, got)
		}
		if got := ecvRawFHIRType(t, e); got != "id" {
			t.Errorf("%s fhir-type in the raw %s, want id", id, got)
		}
	}
}

// Un effectiveDateTime renombrado con un reslice: el elemento renombrado y su reslice quedan con el vs-1 corregido
// (US Core 9.0.0, hl7.fhir.us.core#9.0.0 StructureDefinition/us-core-vital-signs, vs-1).
func TestErrataCrossVersionE1ResliceCopiesTheCorrectedConstraint(t *testing.T) {
	published := ecvExpression(t, "4.0.1", "vitalsigns", "vs-1")
	r := ecvRegistry(t, "4.0.1")
	url := "http://example.org/ecv/e1-vs"
	ecvLoad(t, r, ecvSD(url, "4.0.1", "resource", "Observation", ecvCore+"Observation",
		`{"id":"Observation.effectiveDateTime","path":"Observation.effectiveDateTime","type":[{"code":"dateTime"}],"constraint":[`+ecvConstraintJSON("vs-1", published)+`]}`,
		`{"id":"Observation.effective[x]:effectiveDateTime/extra","path":"Observation.effective[x]","sliceName":"effectiveDateTime/extra","min":0}`))
	sd := ecvSnapshot(t, r, url)
	found := ecvWith(t, sd, "Observation.effective", "vs-1")
	// el renombrado y su reslice: dos elementos con vs-1, los dos corregidos
	if len(found) < 2 {
		t.Fatalf("%d elements under Observation.effective carry vs-1, want the renamed one and its reslice", len(found))
	}
	for _, e := range found {
		ecvAgree(t, e, "vs-1", ecvUSCoreVsOne)
	}
}

// --- E4: el core raíz de la cadena.

// ecvRootSD escribe un core raíz sintético de type Bundle: una especialización (resource, o logical con una base que no
// se resuelve) con su snapshot, declarado en la versión version.
func ecvRootSD(url, version, kind, base string) string {
	b := ""
	if base != "" {
		b = `,"baseDefinition":` + ecvJSON(base)
	}
	return `{"resourceType":"StructureDefinition","url":` + ecvJSON(url) + `,"version":` + ecvJSON(version) + `,"name":"EcvRoot","status":"active","fhirVersion":` + ecvJSON(version) +
		`,"kind":` + ecvJSON(kind) + `,"abstract":false,"type":"Bundle","derivation":"specialization"` + b +
		`,"snapshot":{"element":[{"id":"Bundle","path":"Bundle","min":0,"max":"*","base":{"path":"Bundle","min":0,"max":"*"}}]}}`
}

// Las constraints que una errata agrega se agregan con la versión del core raíz de la cadena (la primera definición
// specialization que se alcanza siguiendo baseDefinition): bdl-15, que R4 y R4B no publican como invariante, está en un
// perfil de cualquier versión cuya raíz es R4 o R4B, y no se agrega si la raíz es R5 (que la publica ella o no la tiene).
func TestErrataCrossVersionE4RootCore(t *testing.T) {
	r5 := ecvExpression(t, "5.0.0", "Bundle", "bdl-15")
	for _, tt := range []struct {
		name       string
		kind       string
		rootBase   string // baseDefinition de la raíz: nunca se sigue
		rootVer    string
		midBase    func(root string) string // cómo nombra el perfil intermedio a su base
		leafVer    string
		wantAdded  bool
		wantReason string
	}{
		{"specialization R4, versionless base", "resource", "", "4.0.1", func(r string) string { return r }, "5.0.0", true, "root is R4"},
		{"specialization R4, base with a version (|4.0.1)", "resource", "", "4.0.1", func(r string) string { return r + "|4.0.1" }, "5.0.0", true, "root is R4"},
		{"logical specialization R4B stops at the first specialization", "logical", "http://example.org/ecv/e4-unresolvable", "4.3.0", func(r string) string { return r }, "5.0.0", true, "root is R4B"},
		{"specialization R4 with a profile of another version on top", "resource", "http://example.org/ecv/e4-unresolvable", "4.0.1", func(r string) string { return r }, "4.3.0", true, "root is R4"},
		{"specialization R5", "resource", "", "5.0.0", func(r string) string { return r }, "4.0.1", false, "root is R5: no erratum adds it"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := ecvRegistry(t, "5.0.0")
			root := "http://example.org/ecv/e4-root-" + strings.ReplaceAll(strings.Fields(tt.name)[0]+tt.rootVer, ".", "")
			mid, leaf := root+"-mid", root+"-leaf"
			ecvLoad(t, r,
				ecvRootSD(root, tt.rootVer, tt.kind, tt.rootBase),
				ecvBundleDefinition(mid, tt.leafVer, tt.midBase(root), ""),
				ecvBundleDefinition(leaf, tt.leafVer, mid, ""))
			e := ecvElement(t, ecvSnapshot(t, r, leaf), "Bundle")
			has := constraintKeys(e)["bdl-15"]
			if has != tt.wantAdded {
				t.Fatalf("bdl-15 present=%v, want %v (%s)", has, tt.wantAdded, tt.wantReason)
			}
			if has {
				if got := ecvConstraintOf(t, e, "bdl-15").Expression; got != r5 {
					t.Errorf("bdl-15\n got %s\nwant %s", got, r5)
				}
			}
		})
	}
}

// ecvTimingSource escribe una definición escrita en R4 con un tim-9 publicado en un hijo: la raíz es de type root
// (Extension, Timing o un modelo lógico), y el hijo deriva de Timing.repeat.
func ecvTimingSource(url, root, kind, published string) string {
	return `{"resourceType":"StructureDefinition","url":` + ecvJSON(url) + `,"version":"4.0.1","name":"EcvSource","status":"active","fhirVersion":"4.0.1","kind":` + ecvJSON(kind) +
		`,"abstract":false,"type":` + ecvJSON(root) + `,"derivation":"specialization","snapshot":{"element":[` +
		`{"id":` + ecvJSON(root) + `,"path":` + ecvJSON(root) + `,"min":0,"max":"*","base":{"path":` + ecvJSON(root) + `,"min":0,"max":"*"}},` +
		`{"id":` + ecvJSON(root+".repeat") + `,"path":` + ecvJSON(root+".repeat") + `,"min":0,"max":"1","base":{"path":"Timing.repeat","min":0,"max":"1"},` +
		`"type":[{"code":"Element"}],"constraint":[` + ecvConstraintJSON("tim-9", published) + `]}]}}`
}

// Un perfil R5 que despliega los hijos de una definición escrita en R4 hereda el tim-9 corregido, sea cual sea la raíz
// de esa definición (Extension, un tipo de datos, un modelo lógico) y como la nombre: por el perfil de tipo del
// elemento, con o sin versión en el canónico (E4, C2).
func TestErrataCrossVersionE4UnrolledFromAnotherRoot(t *testing.T) {
	corrected := ecvExpression(t, "5.0.0", "Timing", "tim-9")
	published := ecvExpression(t, "4.0.1", "Timing", "tim-9")
	for _, root := range []struct{ typ, kind string }{{"Timing", "complex-type"}, {"Extension", "complex-type"}, {"EcvEnvelope", "logical"}} {
		for _, canonical := range []string{"", "|4.0.1"} {
			t.Run(root.typ+canonical, func(t *testing.T) {
				r := ecvRegistry(t, "5.0.0")
				src := "http://example.org/ecv/e4-src-" + root.typ + strings.TrimPrefix(canonical, "|")
				host := src + "-host"
				ecvLoad(t, r,
					ecvTimingSource(src, root.typ, root.kind, published),
					ecvSD(host, "5.0.0", "complex-type", "Dosage", ecvCore+"Dosage",
						`{"id":"Dosage.timing","path":"Dosage.timing","type":[{"code":`+ecvJSON(root.typ)+`,"profile":[`+ecvJSON(src+canonical)+`]}]}`,
						`{"id":"Dosage.timing.repeat","path":"Dosage.timing.repeat","min":1}`))
				e := ecvElement(t, ecvSnapshot(t, r, host), "Dosage.timing.repeat")
				ecvAgree(t, e, "tim-9", corrected)
			})
		}
	}
}

// Una contentReference externa (un canónico con versión y fragmento) trae del destino el tim-9 corregido (E4).
func TestErrataCrossVersionE4ExternalContentReference(t *testing.T) {
	corrected := ecvExpression(t, "5.0.0", "Timing", "tim-9")
	published := ecvExpression(t, "4.0.1", "Timing", "tim-9")
	r := ecvRegistry(t, "5.0.0")
	src := "http://example.org/ecv/e4-cr-source"
	hostBase := "http://example.org/ecv/e4-cr-hostbase"
	host := "http://example.org/ecv/e4-cr-host"
	ecvLoad(t, r,
		ecvTimingSource(src, "Timing", "complex-type", published),
		`{"resourceType":"StructureDefinition","url":`+ecvJSON(hostBase)+`,"name":"EcvHost","status":"active","fhirVersion":"5.0.0","kind":"logical","abstract":false,"type":"EcvHost","derivation":"specialization","snapshot":{"element":[`+
			`{"id":"EcvHost","path":"EcvHost","min":0,"max":"*","base":{"path":"EcvHost","min":0,"max":"*"}},`+
			`{"id":"EcvHost.part","path":"EcvHost.part","min":0,"max":"1","base":{"path":"EcvHost.part","min":0,"max":"1"},"contentReference":`+ecvJSON(src+"|4.0.1#Timing")+`}]}}`,
		ecvSD(host, "5.0.0", "logical", "EcvHost", hostBase, `{"id":"EcvHost.part.repeat","path":"EcvHost.part.repeat","min":1}`))
	e := ecvElement(t, ecvSnapshot(t, r, host), "EcvHost.part.repeat")
	ecvAgree(t, e, "tim-9", corrected)
}

// --- E3: un Bundle anidado.

// Un Bundle en Bundle.entry.resource y otro en Parameters.parameter.resource, con sus descendientes desplegados,
// reciben bdl-15 por su propia definición: la raíz del Bundle anidado no se despliega (unroll copia los descendientes),
// y la definición de Bundle de R4 la trae (plan §3, E3; bundle.html#bundle-unique).
func TestErrataCrossVersionE3NestedBundle(t *testing.T) {
	for _, tt := range []struct{ outer, parent string }{{"Bundle", "Bundle.entry.resource"}, {"Parameters", "Parameters.parameter.resource"}} {
		t.Run(tt.outer, func(t *testing.T) {
			r := ecvRegistry(t, "4.0.1")
			url := "http://example.org/ecv/e3-" + tt.outer
			ecvLoad(t, r, ecvSD(url, "4.0.1", "resource", tt.outer, ecvCore+tt.outer,
				`{"id":`+ecvJSON(tt.parent)+`,"path":`+ecvJSON(tt.parent)+`,"type":[{"code":"Bundle"}]}`,
				`{"id":`+ecvJSON(tt.parent+".type")+`,"path":`+ecvJSON(tt.parent+".type")+`,"min":1}`))
			sd := ecvSnapshot(t, r, url)
			descendants := 0
			for _, e := range sd.Snapshot.Element {
				if strings.HasPrefix(e.ID, tt.parent+".") {
					descendants++
				}
			}
			if descendants == 0 {
				t.Fatalf("no descendant of %s unrolled", tt.parent)
			}
			ecvElement(t, sd, tt.parent+".entry.fullUrl") // los de Bundle.entry.fullUrl bajo el Bundle anidado
			if !constraintKeys(&r.GetByType("Bundle").Snapshot.Element[0])["bdl-15"] {
				t.Error("the Bundle definition the nested resource is validated with has no bdl-15")
			}
		})
	}
}

// --- B3: el contrato de versión.

// Una definición que no declara versión se corrige con la versión elegida antes de cargar (SetFHIRVersion; B3): R4
// corrige el ref-1 publicado de R4, y R4B el de R4B, cada uno con su errata, y ninguno el del otro.
func TestErrataCrossVersionB3ChosenVersionBeforeLoading(t *testing.T) {
	r4, r4b, r5 := ecvExpression(t, "4.0.1", "Reference", "ref-1"), ecvExpression(t, "4.3.0", "Reference", "ref-1"), ecvExpression(t, "5.0.0", "Reference", "ref-1")
	// R4B publica ref-1 con su source (hl7.fhir.r4b.core#4.3.0 StructureDefinition/Reference); R4 no.
	for _, tt := range []struct{ name, chosen, published, source, want string }{
		{"R4 text under 4.0.1", "4.0.1", r4, "", r5}, {"R4B text under 4.3.0", "4.3.0", r4b, ecvCore + "Reference", r5},
		{"R4B text under 4.0.1", "4.0.1", r4b, ecvCore + "Reference", r4b}, {"R4 text under 4.3.0", "4.3.0", r4, "", r4},
		{"R4 text under 5.0.0", "5.0.0", r4, "", r4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			r.SetFHIRVersion(tt.chosen)
			url := "http://example.org/ecv/b3"
			c := ecvConstraintJSON("ref-1", tt.published)
			if tt.source != "" {
				c = ecvConstraintFrom("ref-1", tt.published, tt.source)
			}
			ecvLoad(t, r, ecvSD(url, "", "complex-type", "Reference", ecvCore+"Reference", ecvElementJSON("Reference", c)))
			sd := r.GetByURL(url)
			if got := sd.Differential.Element[0].Constraint[0].Expression; got != tt.want {
				t.Errorf("ref-1 loaded under %s\n got %s\nwant %s", tt.chosen, got, tt.want)
			}
			if got := ecvRawExpression(t, &sd.Differential.Element[0], "ref-1"); got != tt.want {
				t.Errorf("ref-1 (raw) loaded under %s\n got %s\nwant %s", tt.chosen, got, tt.want)
			}
		})
	}
}

// --- inRelease: 4.0.0 y 4.0.1 son R4.

// Una errata de R4 aplica a una definición escrita para 4.0.0 y para 4.0.1 (los dos son la release R4: el defecto está
// publicado en ambas), y no a una de otra release.
func TestErrataCrossVersionInRelease(t *testing.T) {
	r4, r5 := ecvExpression(t, "4.0.1", "Reference", "ref-1"), ecvExpression(t, "5.0.0", "Reference", "ref-1")
	for _, tt := range []struct{ version, want string }{
		{"4.0.0", r5}, {"4.0.1", r5}, {"4.3.0", r4}, {"5.0.0", r4}, {"3.0.2", r4},
	} {
		t.Run(tt.version, func(t *testing.T) {
			r := New()
			r.SetFHIRVersion("4.0.1")
			url := "http://example.org/ecv/release-" + tt.version
			ecvLoad(t, r, ecvSD(url, tt.version, "complex-type", "Reference", ecvCore+"Reference", ecvElementJSON("Reference", ecvConstraintJSON("ref-1", r4))))
			if got := r.GetByURL(url).Differential.Element[0].Constraint[0].Expression; got != tt.want {
				t.Errorf("fhirVersion %s: ref-1\n got %s\nwant %s", tt.version, got, tt.want)
			}
		})
	}
	// La constraint que una errata agrega también: una raíz declarada 4.0.0 recibe bdl-15, igual que una 4.0.1.
	for _, v := range []string{"4.0.0", "4.0.1"} {
		t.Run("bdl-15 root "+v, func(t *testing.T) {
			r := ecvRegistry(t, "5.0.0")
			root, leaf := "http://example.org/ecv/release-root-"+v, "http://example.org/ecv/release-leaf-"+v
			ecvLoad(t, r, ecvRootSD(root, v, "resource", ""), ecvBundleDefinition(leaf, "5.0.0", root, ""))
			if !constraintKeys(ecvElement(t, ecvSnapshot(t, r, leaf), "Bundle"))["bdl-15"] {
				t.Error("no bdl-15 under a root declared " + v)
			}
		})
	}
}

// --- Las erratas contra su fuente (A5, B5).

var ecvSourceRe = regexp.MustCompile(`^hl7\.fhir\.(r4|r4b|r5)\.core#(\d+\.\d+\.\d+) StructureDefinition/([A-Za-z]+), ([A-Za-z0-9.\[\]-]+)`)

// ecvEmbeddedSource separa una fuente de errata «hl7.fhir.r4b.core#4.3.0 StructureDefinition/Bundle, bdl-8»: la
// versión del core embebido, la definición y lo que se cita (una clave de constraint o un id de elemento); ok es false si la
// fuente es de un package que no está embebido.
func ecvEmbeddedSource(source string) (version, sd, what string, ok bool) {
	m := ecvSourceRe.FindStringSubmatch(source)
	if m == nil {
		return "", "", "", false
	}
	return m[2], m[3], m[4], true
}

var ecvAllConstraintsCache sync.Map // versión -> func() map[string]map[string]bool

// ecvAllConstraints da, por clave, las expresiones que el core de version publica con ella.
func ecvAllConstraints(t testing.TB, version string) map[string]map[string]bool {
	t.Helper()
	load, _ := ecvAllConstraintsCache.LoadOrStore(version, sync.OnceValue(func() map[string]map[string]bool {
		out := map[string]map[string]bool{}
		for _, raw := range ecvOfficialSDs(t, version) {
			var sd struct {
				Snapshot struct {
					Element []ecvOfficialElement `json:"element"`
				} `json:"snapshot"`
			}
			if json.Unmarshal(raw, &sd) != nil {
				continue
			}
			for _, e := range sd.Snapshot.Element {
				for _, c := range e.Constraint {
					if out[c.Key] == nil {
						out[c.Key] = map[string]bool{}
					}
					out[c.Key][c.Expression] = true
				}
			}
		}
		return out
	}))
	return load.(func() map[string]map[string]bool)()
}

// Cada errata cuya fuente es un core embebido (R4, R4B o R5) tiene su corrected igual a lo que esa definición oficial
// publica, y su published igual a lo que publica el core de su versión. Las que citan packages que no están
// embebidos (R6 snapshot1, US Core 9.0.0) se saltan con su nombre: quedan en el ADR-004 con la verificación a mano.
func TestErrataCrossVersionAgainstSource(t *testing.T) {
	skip := func(t *testing.T, source string) {
		t.Helper()
		t.Skipf("the source %q is not in an embedded package (R4, R4B or R5 core)", source)
	}
	for _, er := range typeErrata {
		t.Run("type "+er.fhirVersion+" "+er.path, func(t *testing.T) {
			version, sd, element, ok := ecvEmbeddedSource(er.source)
			if !ok {
				skip(t, er.source)
			}
			if got := ecvOfficialFHIRType(t, version, sd, element); got != er.corrected {
				t.Errorf("corrected %q, the source publishes %q", er.corrected, got)
			}
			if got := ecvOfficialFHIRType(t, er.fhirVersion, "Resource", er.path); got != er.published {
				t.Errorf("published %q, core %s publishes %q", er.published, er.fhirVersion, got)
			}
		})
	}
	for _, er := range constraintErrata {
		t.Run("constraint "+er.fhirVersion+" "+er.key+" "+cmpOr(er.path, er.from), func(t *testing.T) {
			version, sd, key, ok := ecvEmbeddedSource(er.source)
			if !ok {
				skip(t, er.source)
			}
			if key != er.key {
				t.Fatalf("the source names %s, the erratum is %s", key, er.key)
			}
			if got := ecvExpression(t, version, sd, key); got != er.corrected {
				t.Errorf("corrected\n got %s\nsource %s", er.corrected, got)
			}
			if !ecvAllConstraints(t, er.fhirVersion)[er.key][er.published] {
				t.Errorf("core %s publishes no %s with the expression %q", er.fhirVersion, er.key, er.published)
			}
		})
	}
	for _, er := range addedConstraints {
		t.Run("added "+er.fhirVersion+" "+er.constraint.Key, func(t *testing.T) {
			version, sd, key, ok := ecvEmbeddedSource(er.source)
			if !ok {
				skip(t, er.source)
			}
			for _, e := range ecvOfficialElements(t, version, sd) {
				for _, c := range e.Constraint {
					if c.Key != key {
						continue
					}
					got := Constraint{Key: c.Key, Severity: c.Severity, Human: c.Human, Expression: c.Expression, Source: c.Source}
					if got != er.constraint {
						t.Errorf("added constraint\n got %+v\nsource %+v", er.constraint, got)
					}
					if !strings.HasPrefix(e.ID, er.path) || e.ID != er.path {
						t.Errorf("the source publishes it on %s, the erratum adds it on %s", e.ID, er.path)
					}
					return
				}
			}
			t.Errorf("the source publishes no %s", key)
		})
	}
	for _, er := range expressionErrata {
		t.Run("expression "+er.fhirVersion+" "+er.published, func(t *testing.T) {
			version, sd, key, ok := ecvEmbeddedSource(er.source)
			if !ok {
				skip(t, er.source)
			}
			if got := ecvExpression(t, version, sd, key); got != er.corrected {
				t.Errorf("corrected\n got %s\nsource %s", er.corrected, got)
			}
			if got := ecvExpression(t, er.fhirVersion, sd, key); got != er.published {
				t.Errorf("published\n got %s\ncore %s %s", er.published, er.fhirVersion, got)
			}
		})
	}
	for _, er := range contextErrata {
		t.Run("context "+er.fhirVersion+" "+er.url, func(t *testing.T) {
			version, sd, _, ok := ecvEmbeddedSource(er.source)
			if !ok {
				skip(t, er.source)
			}
			// la fuente es el tipo de cada primitivo, que usa la extensión en string.value
			used := false
			for _, e := range ecvOfficialElements(t, version, sd) {
				for _, ty := range e.Type {
					for _, x := range ty.Extension {
						used = used || x.URL == er.url
					}
				}
			}
			if !used {
				t.Errorf("%s of core %s does not use %s", sd, version, er.url)
			}
		})
	}
}

// La expresión de una constraint es opcional en R4: corregir ref-1 no agrega «expression» a una constraint ajena que
// solo trae xpath; su raw queda igual que la entrada.
func TestErrataCrossVersionPatchRawKeepsAbsentExpression(t *testing.T) {
	xpathOnly := `{"key":"own-1","severity":"error","human":"XPath only","xpath":"true()"}`
	raw := `{"id":"Reference","path":"Reference","constraint":[` +
		ecvConstraintJSON("ref-1", ecvExpression(t, "4.0.1", "Reference", "ref-1")) + `,` + xpathOnly + `]}`
	var e ElementDefinition
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatal(err)
	}
	e.raw = []byte(raw)
	elems := []ElementDefinition{e}
	correctElements("4.0.1", elems)
	if got := ecvExpression(t, "4.0.1", "Reference", "ref-1"); ecvRawExpression(t, &elems[0], "ref-1") == got {
		t.Fatalf("ref-1 was not corrected: the test does not exercise patchRaw")
	}
	var want, got any
	if err := json.Unmarshal([]byte(xpathOnly), &want); err != nil {
		t.Fatal(err)
	}
	c := ecvRawConstraint(t, &elems[0], "own-1")
	if c == nil {
		t.Fatalf("the raw lost own-1: %s", elems[0].raw)
	}
	b, _ := json.Marshal(c)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("XPath-only constraint changed:\n got %s\nwant %s", b, xpathOnly)
	}
}

// Un perfil sobre el snapshot publicado de Bundle R4 conserva bdl-15 (la errata lo agrega desde R5) aunque su ascendencia
// no se resuelva o sea cíclica: hereda bdl-15 del snapshot de su base, sin adivinar una versión.
func TestErrataCrossVersionUnresolvedAncestryKeepsAddedConstraint(t *testing.T) {
	want := ecvExpression(t, "5.0.0", "Bundle", "bdl-15")
	for _, ancestry := range []string{"missing", "cycle"} {
		t.Run(ancestry, func(t *testing.T) {
			r := ecvRegistry(t, "4.0.1")
			base := "http://example.org/ecv/ancestry-base"
			parent := "http://example.org/ecv/ancestry-missing"
			if ancestry == "cycle" {
				parent = base
			}
			var published map[string]json.RawMessage
			if err := json.Unmarshal(ecvOfficialSDs(t, "4.0.1")[ecvCore+"Bundle"], &published); err != nil {
				t.Fatal(err)
			}
			published["url"] = json.RawMessage(ecvJSON(base))
			published["derivation"] = json.RawMessage(`"constraint"`)
			published["baseDefinition"] = json.RawMessage(ecvJSON(parent))
			delete(published, "differential")
			ecvLoad(t, r, ecvJSON(published),
				ecvSD(base+"-leaf", "4.0.1", "resource", "Bundle", base, `{"id":"Bundle","path":"Bundle","short":"leaf"}`))
			sd := ecvSnapshot(t, r, base+"-leaf")
			if !constraintKeys(ecvElement(t, sd, "Bundle"))["bdl-15"] {
				t.Fatalf("the snapshot lost bdl-15")
			}
			// La constraint que la errata agrega vive solo en el struct, no en el raw.
			if got := ecvConstraintOf(t, ecvElement(t, sd, "Bundle"), "bdl-15").Expression; got != want {
				t.Errorf("bdl-15:\n got %s\nwant %s", got, want)
			}
		})
	}
}

// Un perfil R4 con snapshot publicado cuya ascendencia pasa por una definición R5 y luego no se resuelve o es cíclica
// conserva bdl-15 en el perfil derivado: lo hereda de su base, no de una versión adivinada de la cadena.
func TestErrataCrossVersionMixedBrokenAncestryKeepsAddedConstraint(t *testing.T) {
	want := ecvExpression(t, "5.0.0", "Bundle", "bdl-15")
	for _, ending := range []string{"missing", "cycle"} {
		t.Run(ending, func(t *testing.T) {
			r := ecvRegistry(t, "4.0.1")
			base, parent := "http://example.org/ecv/mixed-r4-base", "http://example.org/ecv/mixed-r5-parent"
			end := "http://example.org/ecv/mixed-missing"
			if ending == "cycle" {
				end = base
			}
			var published map[string]json.RawMessage
			if err := json.Unmarshal(ecvOfficialSDs(t, "4.0.1")[ecvCore+"Bundle"], &published); err != nil {
				t.Fatal(err)
			}
			published["url"] = json.RawMessage(ecvJSON(base))
			published["derivation"] = json.RawMessage(`"constraint"`)
			published["baseDefinition"] = json.RawMessage(ecvJSON(parent))
			delete(published, "differential")
			ecvLoad(t, r,
				ecvSD(parent, "5.0.0", "resource", "Bundle", end, `{"id":"Bundle","path":"Bundle"}`),
				ecvJSON(published),
				ecvSD(base+"-leaf", "4.0.1", "resource", "Bundle", base, `{"id":"Bundle","path":"Bundle","short":"leaf"}`))
			if !constraintKeys(ecvElement(t, r.GetByURL(base), "Bundle"))["bdl-15"] {
				t.Fatal("the published base lacks bdl-15")
			}
			sd := ecvSnapshot(t, r, base+"-leaf")
			if !constraintKeys(ecvElement(t, sd, "Bundle"))["bdl-15"] {
				t.Fatal("the derived profile lost bdl-15")
			}
			if got := ecvConstraintOf(t, ecvElement(t, sd, "Bundle"), "bdl-15").Expression; got != want {
				t.Errorf("bdl-15:\n got %s\nwant %s", got, want)
			}
		})
	}
}

// Un slice de un elemento al que una errata agregó una constraint la hereda del elemento base (plan §3 punto 4: la
// constraint agregada vive en el struct y se hereda del elemento base, también en un slice de él). Hoy la única
// errata real es bdl-15, en la raíz de Bundle, que no tiene slices; la prueba agrega una errata sintética a
// addedConstraints sobre Patient.identifier de R4 y la restaura al terminar (no corre en paralelo).
func TestErrataCrossVersionSliceChildInheritsAddedConstraint(t *testing.T) {
	saved := addedConstraints
	t.Cleanup(func() { addedConstraints = saved })
	synthetic := Constraint{Key: "ecv-slice-1", Severity: "error", Human: "x", Expression: "system.exists() or value.exists()"}
	addedConstraints = append(slices.Clone(saved), addedConstraint{
		fhirVersion: fhirR4, path: "Patient.identifier", constraint: synthetic, source: "synthetic",
	})

	r := ecvRegistry(t, "4.0.1")
	base, derived := "http://example.org/ecv/slice-base", "http://example.org/ecv/slice-derived"
	var published map[string]json.RawMessage
	if err := json.Unmarshal(ecvOfficialSDs(t, "4.0.1")[ecvCore+"Patient"], &published); err != nil {
		t.Fatal(err)
	}
	published["url"] = json.RawMessage(ecvJSON(base))
	published["derivation"] = json.RawMessage(`"constraint"`)
	published["baseDefinition"] = json.RawMessage(ecvJSON(ecvCore + "Patient"))
	delete(published, "differential")
	ecvLoad(t, r, ecvJSON(published), ecvSD(derived, "4.0.1", "resource", "Patient", base,
		`{"id":"Patient.identifier","path":"Patient.identifier","slicing":{"discriminator":[{"type":"value","path":"system"}],"rules":"open"}}`,
		`{"id":"Patient.identifier:s1","path":"Patient.identifier","sliceName":"s1"}`,
		`{"id":"Patient.identifier:s1.system","path":"Patient.identifier.system","min":1}`))

	if !constraintKeys(ecvElement(t, r.GetByURL(base), "Patient.identifier"))[synthetic.Key] {
		t.Fatal("the published base lacks the added constraint")
	}
	sd := ecvSnapshot(t, r, derived)
	for _, id := range []string{"Patient.identifier", "Patient.identifier:s1"} {
		if got := ecvConstraintOf(t, ecvElement(t, sd, id), synthetic.Key).Expression; got != synthetic.Expression {
			t.Errorf("%s: %s:\n got %s\nwant %s", id, synthetic.Key, got, synthetic.Expression)
		}
	}
}

// TestErrataCrossVersionAddedConstraintsAreOnResourceRoots fija el alcance de inheritAddedConstraints (plan §3,
// punto 4; code review, ronda 3): hereda una constraint agregada del elemento de la base con el mismo id, o del
// elemento dividido, pero no de un hijo desplegado de un tipo ni de un contentReference. Basta mientras toda errata
// que agrega una constraint esté en la raíz de un recurso. Si este test falla, la herencia tiene que cubrir esos casos
// antes de agregar la errata.
func TestErrataCrossVersionAddedConstraintsAreOnResourceRoots(t *testing.T) {
	for _, er := range addedConstraints {
		if strings.Contains(er.path, ".") {
			t.Errorf("%s %s: an erratum adds a constraint below a resource root (%s)", er.fhirVersion, er.constraint.Key, er.path)
			continue
		}
		// La raíz de un tipo de datos también se despliega bajo otro elemento: tiene que ser un recurso, según su
		// definición en el core de la versión de la errata.
		sd := sharedVersion(t, er.fhirVersion).GetByURL("http://hl7.org/fhir/StructureDefinition/" + er.path)
		if sd == nil || sd.Kind != "resource" {
			t.Errorf("%s %s: %s is not a resource in its core", er.fhirVersion, er.constraint.Key, er.path)
		}
	}
}
