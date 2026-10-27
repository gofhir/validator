package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/specs"
)

// Los elementos de elección de ElementDefinition (fixed[x], pattern[x]…) son un solo elemento al
// fusionar el diferencial con la base (#175; plan docs/plans/2026-10-25-snapshot-choice-merge.md).
// Los valores esperados salen de la especificación (json.html#choice: un elemento de elección es una
// sola propiedad; json.html#primitive: su id y sus extension van en el hermano "_") y de HL7 6.10.2
// (ProfileUtilities.updateFromDefinition: base.setFixed(derived.getFixed().copy()) reemplaza el valor
// entero; corrida en testdata/snapshot-choice-merge/hl7/outcomes.json).

// choiceVersions es cada versión FHIR de los packages embebidos, con la de sus perfiles de prueba.
var choiceVersions = []string{"4.0.1", "4.3.0", "5.0.0"}

// testChoice es un elemento de elección directo de ElementDefinition, leído del package.
type testChoice struct {
	name  string   // "fixed"
	types []string // los type.code, en el orden del package
}

// choicesOf lee del package de la versión los elementos ElementDefinition.<nombre>[x] y sus tipos.
func choicesOf(t *testing.T, version string) []testChoice {
	t.Helper()
	sd := sharedVersion(t, version).GetByURL("http://hl7.org/fhir/StructureDefinition/ElementDefinition")
	if sd == nil || sd.Snapshot == nil {
		t.Fatalf("FHIR %s: no ElementDefinition definition with a snapshot", version)
	}
	var choices []testChoice
	for _, e := range sd.Snapshot.Element {
		name, ok := strings.CutSuffix(strings.TrimPrefix(e.Path, sd.Type+"."), "[x]")
		if !ok || strings.Contains(name, ".") || name == e.Path {
			continue
		}
		c := testChoice{name: name}
		for _, ty := range e.Type {
			c.types = append(c.types, ty.Code)
		}
		choices = append(choices, c)
	}
	if len(choices) == 0 {
		t.Fatalf("FHIR %s: ElementDefinition has no choice elements", version)
	}
	return choices
}

// prop es el nombre JSON de un elemento de elección con un tipo (json.html#choice: el tipo con la
// primera letra en mayúscula).
func prop(name, typ string) string {
	r, size := utf8.DecodeRuneInString(typ)
	return name + string(unicode.ToUpper(r)) + typ[size:]
}

// hasType informa si la versión declara el tipo para el elemento de elección.
func hasType(choices []testChoice, name, typ string) bool {
	for _, c := range choices {
		if c.name == name {
			return slices.Contains(c.types, typ)
		}
	}
	return false
}

// testProfile es un perfil de Observation cuyo diferencial pone props en el elemento id.
func testProfile(version, id, base, elementID string, props map[string]any) string {
	if !strings.HasPrefix(base, "http") {
		base = "http://example.org/StructureDefinition/" + base
	}
	elem := map[string]any{"id": elementID, "path": elementID}
	if name, ok := strings.CutPrefix(elementID, "Observation.value[x]:"); ok {
		elem["path"] = "Observation.value[x]"
		elem["sliceName"] = name
	}
	maps.Copy(elem, props)
	sd := map[string]any{
		"resourceType": "StructureDefinition", "id": id, "url": "http://example.org/StructureDefinition/" + id,
		"name": strings.ReplaceAll(id, "-", ""), "status": "draft", "fhirVersion": version, "kind": "resource",
		"abstract": false, "type": "Observation", "derivation": "constraint", "baseDefinition": base,
		"differential": map[string]any{"element": []any{elem}},
	}
	raw, err := json.Marshal(sd)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// withProfiles devuelve una copia del registro compartido de la versión con los perfiles cargados.
func withProfiles(t *testing.T, version string, sds ...string) *Registry {
	t.Helper()
	r := sharedVersion(t, version).Clone()
	loadProfiles(t, r, sds...)
	return r
}

func loadProfiles(t *testing.T, r *Registry, sds ...string) {
	t.Helper()
	resources := map[string]json.RawMessage{}
	for i, sd := range sds {
		resources["sd"+strconv.Itoa(i)] = json.RawMessage(sd)
	}
	pkg := &loader.Package{Name: "example.choice", Version: "0.0.1", Resources: resources}
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatalf("LoadFromPackages: %v", err)
	}
}

// snapshotElement genera el snapshot del perfil id y devuelve su elemento elementID como mapa.
func snapshotElement(t *testing.T, r *Registry, id, elementID string) (elem map[string]json.RawMessage, notes []string) {
	t.Helper()
	sd := r.GetByURL("http://example.org/StructureDefinition/" + id)
	if sd == nil {
		t.Fatalf("profile %s not loaded", id)
	}
	if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatalf("EnsureSnapshot(%s): %v", id, err)
	}
	for i := range sd.Snapshot.Element {
		e := &sd.Snapshot.Element[i]
		if e.ID == elementID {
			m, err := rawToMap(e.raw)
			if err != nil {
				t.Fatal(err)
			}
			return m, sd.SnapshotNotes()
		}
	}
	t.Fatalf("%s: no element %s in the snapshot", id, elementID)
	return nil, nil
}

// familyKeys devuelve las propiedades del elemento que son del elemento de elección name, en
// cualquiera de los tipos de la versión, con o sin "_".
func familyKeys(elem map[string]json.RawMessage, choices []testChoice, name string) []string {
	var keys []string
	for k := range elem {
		for _, c := range choices {
			if c.name != name {
				continue
			}
			for _, ty := range c.types {
				if p := prop(name, ty); k == p || k == "_"+p {
					keys = append(keys, k)
				}
			}
		}
	}
	slices.Sort(keys)
	return keys
}

// withoutFamily devuelve el elemento sin las propiedades del elemento de elección name.
func withoutFamily(elem map[string]json.RawMessage, choices []testChoice, name string) map[string]string {
	drop := familyKeys(elem, choices, name)
	out := map[string]string{}
	for k, v := range elem {
		if !slices.Contains(drop, k) {
			out[k] = string(v)
		}
	}
	return out
}

var testExtension = map[string]any{"extension": []any{map[string]any{"url": "http://example.org/StructureDefinition/note", "valueString": "x"}}}

// Para cada elemento de elección de ElementDefinition, en cada versión, y en los dos sentidos del
// orden alfabético de los tipos: si el diferencial trae el elemento con otro tipo, el snapshot tiene
// solo la propiedad del diferencial, sin el valor ni el "_" de la base, y el resto del elemento es el
// de la base (json.html#choice; HL7 setFixed).
func TestMergeReplacesWholeChoiceElement(t *testing.T) {
	for _, version := range choiceVersions {
		choices := choicesOf(t, version)
		for _, c := range choices {
			first, last := c.types[0], c.types[len(c.types)-1]
			for _, pair := range [][2]string{{first, last}, {last, first}} {
				baseType, diffType := pair[0], pair[1]
				t.Run(fmt.Sprintf("%s/%s/%s-to-%s", version, c.name, baseType, diffType), func(t *testing.T) {
					a := testProfile(version, "a", "http://hl7.org/fhir/StructureDefinition/Observation", "Observation.status",
						map[string]any{prop(c.name, baseType): "base", "_" + prop(c.name, baseType): testExtension, "short": "from a"})
					b := testProfile(version, "b", "a", "Observation.status", map[string]any{prop(c.name, diffType): "diff"})
					r := withProfiles(t, version, a, b)
					aElem, _ := snapshotElement(t, r, "a", "Observation.status")
					bElem, _ := snapshotElement(t, r, "b", "Observation.status")

					if got, want := familyKeys(bElem, choices, c.name), []string{prop(c.name, diffType)}; !slices.Equal(got, want) {
						t.Errorf("properties of %s[x] = %v, want %v", c.name, got, want)
					}
					if got := string(bElem[prop(c.name, diffType)]); got != `"diff"` {
						t.Errorf("%s = %s, want the differential's \"diff\"", prop(c.name, diffType), got)
					}
					if got, want := withoutFamily(bElem, choices, c.name), withoutFamily(aElem, choices, c.name); !maps.Equal(got, want) {
						t.Errorf("the rest of the element changed:\n got %v\nwant %v", got, want)
					}
				})
			}
		}
	}
}

// Casos con un valor esperado de la referencia, en R4, R4B y R5.
func TestMergeChoiceCases(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	tests := []struct {
		name string
		base map[string]any // el diferencial del perfil base (a), sobre Observation.status
		diff map[string]any // el diferencial del perfil derivado (b)
		// want son las propiedades de fixed[x] y pattern[x] del snapshot de b, con su valor JSON.
		want map[string]string
	}{
		{
			// profiling.html: lo que el diferencial dice reemplaza a lo de la base.
			name: "same type",
			base: map[string]any{"fixedString": "a"},
			diff: map[string]any{"fixedString": "b"},
			want: map[string]string{"fixedString": `"b"`},
		},
		{
			// HL7 6.10.2, perfil value-drops-ext: una instancia sin la extensión es válida, así que el
			// "_" de la base se pierde cuando el diferencial trae solo el valor.
			name: "same type, the base's extensions are replaced",
			base: map[string]any{"fixedString": "a", "_fixedString": testExtension},
			diff: map[string]any{"fixedString": "a"},
			want: map[string]string{"fixedString": `"a"`},
		},
		{
			name: "another type, the base's extensions go with its value",
			base: map[string]any{"fixedString": "a", "_fixedString": testExtension},
			diff: map[string]any{"fixedInteger": 5},
			want: map[string]string{"fixedInteger": `5`},
		},
		{
			// HL7 6.10.2: hasFixed() es verdadero para un primitivo que solo tiene extension, y setFixed
			// reemplaza el valor entero (ProfileUtilities.java:2779-2786).
			name: "only extensions, same type",
			base: map[string]any{"fixedString": "a"},
			diff: map[string]any{"_fixedString": testExtension},
			want: map[string]string{"_fixedString": mustJSON(testExtension)},
		},
		{
			name: "only extensions, another type",
			base: map[string]any{"fixedString": "a"},
			diff: map[string]any{"_fixedInteger": testExtension},
			want: map[string]string{"_fixedInteger": mustJSON(testExtension)},
		},
		{
			name: "value and extensions",
			base: map[string]any{"fixedString": "a"},
			diff: map[string]any{"fixedInteger": 5, "_fixedInteger": testExtension},
			want: map[string]string{"fixedInteger": `5`, "_fixedInteger": mustJSON(testExtension)},
		},
		{
			// HL7 6.10.2, perfil adds-pattern: las instancias "a" y "b" fallan cada una contra el otro
			// valor, así que fixed[x] y pattern[x] se conservan los dos.
			name: "fixed and pattern are two elements",
			base: map[string]any{"fixedString": "a"},
			diff: map[string]any{"patternString": "b"},
			want: map[string]string{"fixedString": `"a"`, "patternString": `"b"`},
		},
	}
	for _, version := range choiceVersions {
		choices := choicesOf(t, version)
		for _, tt := range tests {
			t.Run(version+"/"+tt.name, func(t *testing.T) {
				a := testProfile(version, "a", observation, "Observation.status", tt.base)
				b := testProfile(version, "b", "a", "Observation.status", tt.diff)
				elem, _ := snapshotElement(t, withProfiles(t, version, a, b), "b", "Observation.status")
				got := map[string]string{}
				for _, name := range []string{"fixed", "pattern"} {
					for _, k := range familyKeys(elem, choices, name) {
						got[k] = string(elem[k])
					}
				}
				if !maps.Equal(got, tt.want) {
					t.Errorf("fixed[x] and pattern[x] = %v, want %v", got, tt.want)
				}
			})
		}
	}
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// Una propiedad con el nombre de un elemento de elección y un tipo que la versión no declara no es
// una propiedad de ElementDefinition (json.html#choice): queda fuera del snapshot, con una nota, y
// el valor de la base se conserva. En la versión que sí declara el tipo, reemplaza al de la base.
// Los tipos salen de lo que cambia de una versión a otra, medido en el plan (§1): integer64 solo existe
// en R5, Meta no existe en R4B y Contributor no existe en R5.
func TestMergeChoiceTypeOfAnotherVersion(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	for _, version := range choiceVersions {
		choices := choicesOf(t, version)
		for _, typ := range []string{"integer64", "Meta", "Contributor"} {
			t.Run(version+"/"+typ, func(t *testing.T) {
				diffProp := prop("fixed", typ)
				a := testProfile(version, "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
				b := testProfile(version, "b", "a", "Observation.status", map[string]any{diffProp: "v"})
				elem, notes := snapshotElement(t, withProfiles(t, version, a, b), "b", "Observation.status")
				got := familyKeys(elem, choices, "fixed")
				if _, raw := elem[diffProp]; raw && !hasType(choices, "fixed", typ) {
					got = append(got, diffProp)
				}
				if hasType(choices, "fixed", typ) {
					if want := []string{diffProp}; !slices.Equal(got, want) {
						t.Errorf("fixed[x] = %v, want %v", got, want)
					}
					return
				}
				if want := []string{"fixedString"}; !slices.Equal(got, want) {
					t.Errorf("fixed[x] = %v, want %v (FHIR %s has no type %s)", got, want, version, typ)
				}
				if !slices.ContainsFunc(notes, func(n string) bool { return strings.Contains(n, diffProp) }) {
					t.Errorf("no snapshot note names %s: %v", diffProp, notes)
				}
			})
		}
	}
}

// Un slice de tipo copia el elemento de elección del que es slice (profiling.html#slicing). Si su
// diferencial fija otro tipo, el slice tiene solo el suyo, y value[x] conserva el de la base.
func TestMergeChoiceInTypeSlice(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	for _, version := range choiceVersions {
		choices := choicesOf(t, version)
		t.Run(version, func(t *testing.T) {
			a := testProfile(version, "a", observation, "Observation.value[x]", map[string]any{"fixedString": "a"})
			b := testProfile(version, "b", "a", "Observation.value[x]:valueInteger",
				map[string]any{"type": []any{map[string]any{"code": "integer"}}, "fixedInteger": 5})
			r := withProfiles(t, version, a, b)
			slice, _ := snapshotElement(t, r, "b", "Observation.value[x]:valueInteger")
			if got, want := familyKeys(slice, choices, "fixed"), []string{"fixedInteger"}; !slices.Equal(got, want) {
				t.Errorf("slice fixed[x] = %v, want %v", got, want)
			}
			parent, _ := snapshotElement(t, r, "b", "Observation.value[x]")
			if got, want := familyKeys(parent, choices, "fixed"), []string{"fixedString"}; !slices.Equal(got, want) {
				t.Errorf("value[x] fixed[x] = %v, want %v", got, want)
			}
		})
	}
}

// En una cadena A→B→C, cada derivado reemplaza el valor que recibe.
func TestMergeChoiceAlongAChain(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	for _, version := range choiceVersions {
		choices := choicesOf(t, version)
		t.Run(version, func(t *testing.T) {
			a := testProfile(version, "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
			b := testProfile(version, "b", "a", "Observation.status", map[string]any{"fixedInteger": 5})
			c := testProfile(version, "c", "b", "Observation.status", map[string]any{"fixedBoolean": true})
			elem, _ := snapshotElement(t, withProfiles(t, version, a, b, c), "c", "Observation.status")
			if got, want := familyKeys(elem, choices, "fixed"), []string{"fixedBoolean"}; !slices.Equal(got, want) {
				t.Errorf("fixed[x] = %v, want %v", got, want)
			}
		})
	}
}

// Metamórfico: el orden de las propiedades en el JSON del diferencial no cambia el snapshot.
func TestMergeChoiceKeyOrder(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	ext := mustJSON(testExtension)
	a := testProfile("4.0.1", "a", observation, "Observation.status", map[string]any{"fixedString": "a", "_fixedString": testExtension})
	var want map[string]json.RawMessage
	for _, order := range []string{
		`"id":"Observation.status","path":"Observation.status","fixedInteger":5,"_fixedInteger":` + ext,
		`"_fixedInteger":` + ext + `,"fixedInteger":5,"path":"Observation.status","id":"Observation.status"`,
		`"fixedInteger":5,"id":"Observation.status","_fixedInteger":` + ext + `,"path":"Observation.status"`,
	} {
		b := strings.Replace(testProfile("4.0.1", "b", "a", "Observation.status", nil),
			`{"id":"Observation.status","path":"Observation.status"}`, "{"+order+"}", 1)
		got, _ := snapshotElement(t, withProfiles(t, "4.0.1", a, b), "b", "Observation.status")
		if want == nil {
			want = got
			continue
		}
		if !maps.EqualFunc(got, want, func(x, y json.RawMessage) bool { return string(x) == string(y) }) {
			t.Errorf("order %s:\n got %v\nwant %v", order, got, want)
		}
	}
}

// coreResource devuelve un recurso del package core embebido de la versión, por su URL.
func coreResource(t *testing.T, version, url string) json.RawMessage {
	t.Helper()
	packages, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages(version))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range packages {
		if raw, ok := p.Resources[url]; ok {
			return raw
		}
	}
	t.Fatalf("FHIR %s: %s is not in the embedded packages", version, url)
	return nil
}

// fakeResolver entrega las StructureDefinitions que tiene, por URL.
type fakeResolver map[string]json.RawMessage

func (f fakeResolver) ResolveProfile(_ context.Context, url, _ string) ([]byte, error) {
	return f[url], nil
}

// Sin la definición de ElementDefinition no se sabe qué propiedades son un mismo elemento de
// elección: la generación falla con un error, en vez de fusionar clave por clave (plan §2.4,
// aprobado por el usuario). El error no es permanente: con la definición cargada después, o
// entregada por el ProfileResolver, la generación funciona.
func TestMergeChoiceNeedsElementDefinition(t *testing.T) {
	const (
		observation = "http://hl7.org/fhir/StructureDefinition/Observation"
		edURL       = "http://hl7.org/fhir/StructureDefinition/ElementDefinition"
	)
	a := testProfile("4.0.1", "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
	b := testProfile("4.0.1", "b", "a", "Observation.status", map[string]any{"fixedInteger": 5})
	obs := string(coreResource(t, "4.0.1", observation))
	ed := string(coreResource(t, "4.0.1", edURL))

	t.Run("loaded later", func(t *testing.T) {
		r := New()
		loadProfiles(t, r, obs, a, b)
		sd := r.GetByURL("http://example.org/StructureDefinition/b")
		err := r.EnsureSnapshot(context.Background(), sd)
		if err == nil || !strings.Contains(err.Error(), edURL) {
			t.Fatalf("EnsureSnapshot without %s: err = %v, want an error naming it", edURL, err)
		}
		loadProfiles(t, r, ed)
		sd = r.GetByURL("http://example.org/StructureDefinition/b")
		if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
			t.Fatalf("EnsureSnapshot after loading %s: %v", edURL, err)
		}
	})

	t.Run("from the resolver", func(t *testing.T) {
		r := New()
		loadProfiles(t, r, obs, a, b)
		r.SetResolver(fakeResolver{edURL: json.RawMessage(ed)})
		elem, _ := snapshotElement(t, r, "b", "Observation.status")
		if _, ok := elem["fixedString"]; ok {
			t.Errorf("fixedString survived: %v", elem)
		}
	})
}

// ADR-001 (docs/adr/ADR-001-elementdefinition-metamodel.md): la constante elementDefinitionURL nombra
// el tipo de los elementos de un snapshot. En cada versión, la definición de StructureDefinition
// tiene que decir lo mismo.
func TestElementDefinitionURLIsTheSnapshotElementType(t *testing.T) {
	for _, version := range choiceVersions {
		sd := sharedVersion(t, version).GetByURL("http://hl7.org/fhir/StructureDefinition/StructureDefinition")
		if sd == nil || sd.Snapshot == nil {
			t.Fatalf("FHIR %s: no StructureDefinition definition", version)
		}
		var codes []string
		for _, e := range sd.Snapshot.Element {
			if e.Path == "StructureDefinition.snapshot.element" {
				for _, ty := range e.Type {
					codes = append(codes, ty.Code)
				}
			}
		}
		if want := []string{strings.TrimPrefix(elementDefinitionURL, "http://hl7.org/fhir/StructureDefinition/")}; !slices.Equal(codes, want) {
			t.Errorf("FHIR %s: StructureDefinition.snapshot.element has types %v, want %v", version, codes, want)
		}
	}
}

// Un registro puede tener la definición de ElementDefinition de R4 y la de R5. Cada perfil usa la de
// su versión (StructureDefinition.fhirVersion): integer64 es un tipo de fixed[x] en R5 y no en R4.
func TestMergeChoiceUsesTheProfilesVersion(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	r := sharedVersion(t, "4.0.1").Clone()
	loadProfiles(t, r, string(coreResource(t, "5.0.0", elementDefinitionURL)))
	for _, tt := range []struct {
		version string
		want    []string // las propiedades de fixed[x] del snapshot del perfil derivado
	}{
		{"4.0.1", []string{"fixedString"}},    // R4 no tiene integer64: queda fuera, con nota
		{"5.0.0", []string{"fixedInteger64"}}, // R5 sí: reemplaza al de la base
	} {
		t.Run(tt.version, func(t *testing.T) {
			r := r.Clone()
			a := testProfile(tt.version, "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
			b := testProfile(tt.version, "b", "a", "Observation.status", map[string]any{"fixedInteger64": "5"})
			loadProfiles(t, r, a, b)
			elem, _ := snapshotElement(t, r, "b", "Observation.status")
			var got []string
			for k := range elem {
				if strings.HasPrefix(k, "fixed") {
					got = append(got, k)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("fixed[x] = %v, want %v", got, tt.want)
			}
		})
	}
}

// countingResolver cuenta las llamadas y falla las primeras fail.
type countingResolver struct {
	defs  map[string]json.RawMessage
	fail  int
	calls []string
}

func (c *countingResolver) ResolveProfile(_ context.Context, url, version string) ([]byte, error) {
	c.calls = append(c.calls, url+"|"+version)
	if len(c.calls) <= c.fail {
		return nil, fmt.Errorf("resolver unavailable")
	}
	return c.defs[url], nil
}

// La definición de ElementDefinition se busca primero en el registro: si está cargada, el resolver no
// se consulta, aunque el perfil declare una versión que no está (4.0.0, de la misma release). Si no
// está, una falla del resolver es un error que no queda guardado: el siguiente intento lo consulta de
// nuevo (code review, C2 y F1).
func TestMergeChoiceElementDefinitionResolution(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	a := testProfile("4.0.0", "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
	b := testProfile("4.0.0", "b", "a", "Observation.status", map[string]any{"fixedInteger": 5})

	// El resolver no tiene la 4.0.0 del perfil, pero sí la 4.0.1, de la misma release.
	t.Run("same release from the resolver", func(t *testing.T) {
		r := New()
		loadProfiles(t, r, string(coreResource(t, "4.0.1", observation)), a, b)
		r.SetResolver(&countingResolver{defs: map[string]json.RawMessage{elementDefinitionURL: coreResource(t, "4.0.1", elementDefinitionURL)}})
		elem, _ := snapshotElement(t, r, "b", "Observation.status")
		if _, ok := elem["fixedString"]; ok {
			t.Errorf("fixedString survived: %v", elem)
		}
	})

	t.Run("loaded", func(t *testing.T) {
		r := withProfiles(t, "4.0.1", a, b)
		res := &countingResolver{}
		r.SetResolver(res)
		elem, _ := snapshotElement(t, r, "b", "Observation.status")
		if _, ok := elem["fixedString"]; ok {
			t.Errorf("fixedString survived: %v", elem)
		}
		for _, c := range res.calls {
			if strings.HasPrefix(c, elementDefinitionURL) {
				t.Errorf("the resolver was asked for %s, which is loaded", c)
			}
		}
	})

	// Un intento consulta al resolver dos veces (la versión y, si no, la misma release): fallan las dos.
	t.Run("resolver fails", func(t *testing.T) {
		r := New()
		a := testProfile("4.0.1", "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
		b := testProfile("4.0.1", "b", "a", "Observation.status", map[string]any{"fixedInteger": 5})
		loadProfiles(t, r, string(coreResource(t, "4.0.1", observation)), a, b)
		res := &countingResolver{defs: map[string]json.RawMessage{elementDefinitionURL: coreResource(t, "4.0.1", elementDefinitionURL)}, fail: 2}
		r.SetResolver(res)
		sd := r.GetByURL("http://example.org/StructureDefinition/b")
		if err := r.EnsureSnapshot(context.Background(), sd); !errors.Is(err, errNoElementDefinition) {
			t.Fatalf("EnsureSnapshot while the resolver failed: err = %v, want %v", err, errNoElementDefinition)
		}
		if want := []string{elementDefinitionURL + "|4.0.1", elementDefinitionURL + "|"}; !slices.Equal(res.calls, want) {
			t.Errorf("resolver calls = %v, want %v", res.calls, want)
		}
		elem, _ := snapshotElement(t, r, "b", "Observation.status")
		if _, ok := elem["fixedString"]; ok {
			t.Errorf("fixedString survived: %v", elem)
		}
	})
}

// Una falla permanente de la definición de ElementDefinition (aquí, una sin snapshot ni diferencial)
// no es permanente para el perfil: con una definición completa cargada después, se genera (C3, F3).
func TestMergeChoiceDefinitionFailureIsNotPermanent(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	a := testProfile("4.0.1", "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
	b := testProfile("4.0.1", "b", "a", "Observation.status", map[string]any{"fixedInteger": 5})
	stub := `{"resourceType":"StructureDefinition","url":"` + elementDefinitionURL + `","version":"0.0.1","name":"Stub","type":"ElementDefinition","kind":"complex-type","abstract":false,"derivation":"specialization"}`
	r := New()
	loadProfiles(t, r, string(coreResource(t, "4.0.1", observation)), a, b, stub)
	sd := r.GetByURL("http://example.org/StructureDefinition/b")
	if err := r.EnsureSnapshot(context.Background(), sd); err == nil {
		t.Fatal("EnsureSnapshot succeeded with a definition that has no snapshot")
	}
	loadProfiles(t, r, string(coreResource(t, "4.0.1", elementDefinitionURL)))
	if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatalf("EnsureSnapshot after loading the complete definition: %v", err)
	}
}

// Una propiedad de la base con un tipo que la versión no declara también es del elemento de elección,
// así que se va con él cuando el diferencial lo reemplaza (C1). La base viene armada a mano: los
// snapshots que genera el registro ya no la tienen.
func TestMergeChoiceDropsTheBasesForeignProperty(t *testing.T) {
	r := sharedVersion(t, "5.0.0")
	choices, err := r.choiceIndexFor(context.Background(), "5.0.0")
	if err != nil {
		t.Fatal(err)
	}
	base := &Snapshot{Element: []ElementDefinition{makeDiffElement(t, map[string]any{
		"id": "Observation.status", "path": "Observation.status",
		"fixedContributor": map[string]any{"type": "author", "name": "x"}, "_fixedContributor": testExtension,
	})}}
	diff := &Differential{Element: []ElementDefinition{makeDiffElement(t, map[string]any{
		"id": "Observation.status", "path": "Observation.status", "fixedString": "a",
	})}}
	snapshot, _, err := r.Clone().applyDifferential(context.Background(), base, diff, choices)
	if err != nil {
		t.Fatal(err)
	}
	elem, err := rawToMap(snapshot.Element[0].raw)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for k := range elem {
		if strings.Contains(k, "fixed") {
			got = append(got, k)
		}
	}
	slices.Sort(got)
	if want := []string{"fixedString"}; !slices.Equal(got, want) {
		t.Errorf("fixed[x] = %v, want %v", got, want)
	}
}

// Con la definición de otra release cargada, un perfil R5 no usa sus tipos: si hay resolver, se le
// pide la de R5 (code review, ronda 2, C5). El tipo integer64 de fixed[x] existe en R5 y no en R4.
func TestMergeChoiceAsksTheResolverForTheProfilesRelease(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	r := sharedVersion(t, "4.0.1").Clone()
	res := &countingResolver{defs: map[string]json.RawMessage{elementDefinitionURL: coreResource(t, "5.0.0", elementDefinitionURL)}}
	r.SetResolver(res)
	a := testProfile("5.0.0", "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
	b := testProfile("5.0.0", "b", "a", "Observation.status", map[string]any{"fixedInteger64": "5"})
	loadProfiles(t, r, a, b)
	elem, _ := snapshotElement(t, r, "b", "Observation.status")
	var got []string
	for k := range elem {
		if strings.HasPrefix(k, "fixed") {
			got = append(got, k)
		}
	}
	if want := []string{"fixedInteger64"}; !slices.Equal(got, want) {
		t.Errorf("fixed[x] = %v, want %v (resolver calls: %v)", got, want, res.calls)
	}
}

// El resolver nunca entrega otra release: si solo tiene la R4, un perfil R5 no se genera. Sin resolver,
// se usa la definición cargada aunque sea de otra release (code review, ronda 3, F12).
func TestMergeChoiceOtherRelease(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	a := testProfile("5.0.0", "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
	b := testProfile("5.0.0", "b", "a", "Observation.status", map[string]any{"fixedInteger": 5})

	t.Run("from the resolver", func(t *testing.T) {
		r := New()
		loadProfiles(t, r, string(coreResource(t, "4.0.1", observation)), a, b)
		res := &countingResolver{defs: map[string]json.RawMessage{elementDefinitionURL: coreResource(t, "4.0.1", elementDefinitionURL)}}
		r.SetResolver(res)
		sd := r.GetByURL("http://example.org/StructureDefinition/b")
		if err := r.EnsureSnapshot(context.Background(), sd); !errors.Is(err, errNoElementDefinition) {
			t.Fatalf("err = %v, want %v", err, errNoElementDefinition)
		}
		if want := []string{elementDefinitionURL + "|5.0.0", elementDefinitionURL + "|"}; !slices.Equal(res.calls, want) {
			t.Errorf("resolver calls = %v, want %v", res.calls, want)
		}
	})

	t.Run("loaded, no resolver", func(t *testing.T) {
		r := New()
		loadProfiles(t, r, string(coreResource(t, "4.0.1", observation)), string(coreResource(t, "4.0.1", elementDefinitionURL)), a, b)
		elem, _ := snapshotElement(t, r, "b", "Observation.status")
		if _, ok := elem["fixedString"]; ok {
			t.Errorf("fixedString survived: %v", elem)
		}
	})
}

// Una definición que no declara su versión no dice ser de otra release (F10).
func TestMergeChoiceDefinitionWithoutFHIRVersion(t *testing.T) {
	const observation = "http://hl7.org/fhir/StructureDefinition/Observation"
	var def map[string]any
	if err := json.Unmarshal(coreResource(t, "4.0.1", elementDefinitionURL), &def); err != nil {
		t.Fatal(err)
	}
	delete(def, "fhirVersion")
	a := testProfile("4.0.0", "a", observation, "Observation.status", map[string]any{"fixedString": "a"})
	b := testProfile("4.0.0", "b", "a", "Observation.status", map[string]any{"fixedInteger": 5})
	r := New()
	loadProfiles(t, r, string(coreResource(t, "4.0.1", observation)), mustJSON(def), a, b)
	r.SetResolver(&countingResolver{})
	elem, _ := snapshotElement(t, r, "b", "Observation.status")
	if _, ok := elem["fixedString"]; ok {
		t.Errorf("fixedString survived: %v", elem)
	}
}
