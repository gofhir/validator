package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Oráculo de T1 y T2: la extracción de fixed[x]/pattern[x] tal como estaba en 74afca6
// (registry.go:229-261), copiada sin cambios. Es la referencia de un refactor: el resultado no
// debe cambiar. Recorre un mapa, así que con dos claves del mismo prefijo su respuesta es
// aleatoria; por eso T1 y T2 exigen además que no haya empates (ver prefixKeys).
func oracleValue(raw json.RawMessage, prefix string) (value json.RawMessage, suffix string, ok bool) {
	if raw == nil {
		return nil, "", false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, "", false
	}
	for key, value := range obj {
		if strings.HasPrefix(key, prefix) {
			return value, strings.TrimPrefix(key, prefix), true
		}
	}
	return nil, "", false
}

func oracleElement(raw json.RawMessage, prefix string) (json.RawMessage, bool) {
	if _, typeSuffix, ok := oracleValue(raw, prefix); ok {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, false
		}
		value, ok := obj["_"+prefix+typeSuffix]
		return value, ok
	}
	value, _, ok := oracleValue(raw, "_"+prefix)
	return value, ok
}

// prefixKeys cuenta las claves de raw que empiezan con prefix.
func prefixKeys(t *testing.T, raw json.RawMessage, prefix string) int {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return 0
	}
	n := 0
	for key := range obj {
		if strings.HasPrefix(key, prefix) {
			n++
		}
	}
	return n
}

// checkAgainstOracle compara las cuatro respuestas de ed con el oráculo.
func checkAgainstOracle(t *testing.T, where string, ed *ElementDefinition) {
	t.Helper()
	for _, prefix := range []string{"fixed", "pattern", "_fixed", "_pattern"} {
		if n := prefixKeys(t, ed.raw, prefix); n > 1 {
			t.Fatalf("%s: %d keys start with %q: the oracle is not deterministic there", where, n, prefix)
		}
	}
	gotV, gotS, gotOK := ed.GetFixed()
	wantV, wantS, wantOK := oracleValue(ed.raw, "fixed")
	if gotOK != wantOK || gotS != wantS || !bytes.Equal(gotV, wantV) {
		t.Errorf("%s GetFixed = (%s, %q, %v), want (%s, %q, %v)", where, gotV, gotS, gotOK, wantV, wantS, wantOK)
	}
	gotV, gotS, gotOK = ed.GetPattern()
	wantV, wantS, wantOK = oracleValue(ed.raw, "pattern")
	if gotOK != wantOK || gotS != wantS || !bytes.Equal(gotV, wantV) {
		t.Errorf("%s GetPattern = (%s, %q, %v), want (%s, %q, %v)", where, gotV, gotS, gotOK, wantV, wantS, wantOK)
	}
	gotE, gotOK := ed.GetFixedElement()
	wantE, wantOK := oracleElement(ed.raw, "fixed")
	if gotOK != wantOK || !bytes.Equal(gotE, wantE) {
		t.Errorf("%s GetFixedElement = (%s, %v), want (%s, %v)", where, gotE, gotOK, wantE, wantOK)
	}
	gotE, gotOK = ed.GetPatternElement()
	wantE, wantOK = oracleElement(ed.raw, "pattern")
	if gotOK != wantOK || !bytes.Equal(gotE, wantE) {
		t.Errorf("%s GetPatternElement = (%s, %v), want (%s, %v)", where, gotE, gotOK, wantE, wantOK)
	}
}

var prefixedVersions = []string{"4.0.1", "4.3.0", "5.0.0"}

// T1: toda ElementDefinition de los snapshots publicados en los packages embebidos (core,
// terminología, extensiones) tiene su valor guardado vigente, y sus cuatro respuestas son las del
// oráculo. Además, metamórfico: reordenar las claves del JSON no cambia las respuestas.
func TestPrefixedValuesPublishedSnapshots(t *testing.T) {
	for _, version := range prefixedVersions {
		t.Run(version, func(t *testing.T) {
			r := sharedVersion(t, version)
			elements, withValue := 0, 0
			for _, sd := range r.all {
				snapshot := sd.storedSnapshot()
				if snapshot == nil {
					continue
				}
				for i := range snapshot.Element {
					ed := &snapshot.Element[i]
					elements++
					where := sd.URL + " " + ed.ID
					if ed.cachedPrefixed() == nil {
						t.Errorf("%s: no prefixed values kept", where)
					}
					// Con -short (make check, bajo -race) se comparan solo los elementos que
					// mencionan los prefijos; sin -short, todos, y también reordenados.
					if testing.Short() && !mentionsPrefixes(ed.raw) {
						continue
					}
					checkAgainstOracle(t, where, ed)
					if _, _, ok := oracleValue(ed.raw, "fixed"); ok {
						withValue++
					} else if _, _, ok := oracleValue(ed.raw, "pattern"); ok {
						withValue++
					}
					if !testing.Short() {
						checkReordered(t, where, ed)
					}
				}
			}
			if elements == 0 || withValue == 0 {
				t.Fatalf("%d elements, %d with fixed or pattern: the test checks nothing", elements, withValue)
			}
			t.Logf("%d snapshot elements, %d with fixed[x] or pattern[x]", elements, withValue)
		})
	}
}

// mentionsPrefixes informa si raw contiene alguno de los textos fixed o pattern: un elemento que no
// los contiene no puede tener esas propiedades.
func mentionsPrefixes(raw json.RawMessage) bool {
	return bytes.Contains(raw, []byte("fixed")) || bytes.Contains(raw, []byte("pattern"))
}

// checkReordered comprueba que el mismo elemento con las claves en otro orden (el de un mapa
// serializado: alfabético) da las mismas respuestas.
func checkReordered(t *testing.T, where string, ed *ElementDefinition) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(ed.raw, &obj); err != nil {
		t.Fatalf("%s: %v", where, err)
	}
	reordered, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("%s: %v", where, err)
	}
	var other ElementDefinition
	other.raw = reordered
	for _, get := range []func(*ElementDefinition) (json.RawMessage, string, bool){
		(*ElementDefinition).GetFixed, (*ElementDefinition).GetPattern,
	} {
		v1, s1, ok1 := get(ed)
		v2, s2, ok2 := get(&other)
		if ok1 != ok2 || s1 != s2 || !jsonEqual(v1, v2) {
			t.Errorf("%s: reordered keys answer (%s, %q, %v), original (%s, %q, %v)", where, v2, s2, ok2, v1, s1, ok1)
		}
	}
}

func jsonEqual(a, b json.RawMessage) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	var x, y bytes.Buffer
	if json.Compact(&x, a) != nil || json.Compact(&y, b) != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(x.Bytes(), y.Bytes())
}

// T2: los snapshots regenerados desde el diferencial de cada perfil del core (y de US Core 6.1.0,
// si está en la caché) tienen el valor guardado vigente en cada elemento, y responden lo mismo que
// el oráculo.
func TestPrefixedValuesGeneratedSnapshots(t *testing.T) {
	for _, version := range prefixedVersions {
		t.Run(version, func(t *testing.T) {
			checkRegenerated(t, sharedVersion(t, version).Clone(), "")
		})
	}
	t.Run("hl7.fhir.us.core#6.1.0", func(t *testing.T) {
		if testing.Short() {
			t.Skip("regenerates a whole guide; run without -short")
		}
		checkRegenerated(t, guideRegistry(t, "hl7.fhir.us.core", "6.1.0"), "hl7.fhir.us.core#6.1.0")
	})
}

// checkRegenerated regenera el snapshot de cada perfil (del package packageID, o de todos si es
// vacío) que publica snapshot y diferencial, y revisa sus elementos.
func checkRegenerated(t *testing.T, r *Registry, packageID string) {
	t.Helper()
	profiles, elements := 0, 0
	for _, published := range r.all {
		if published.Derivation != DerivationConstraint || published.Snapshot == nil || published.Differential == nil {
			continue
		}
		if packageID != "" && published.PackageID != packageID {
			continue
		}
		var sd StructureDefinition
		if err := json.Unmarshal(published.raw, &sd); err != nil {
			t.Fatal(err)
		}
		sd.Snapshot = nil
		if err := r.EnsureSnapshot(context.Background(), &sd); err != nil {
			continue // los snapshots que no se pueden generar son de otros tests
		}
		profiles++
		for i := range sd.Snapshot.Element {
			ed := &sd.Snapshot.Element[i]
			elements++
			where := sd.URL + " " + ed.ID + " (generated)"
			if ed.cachedPrefixed() == nil {
				t.Errorf("%s: no prefixed values kept", where)
			}
			if testing.Short() && !mentionsPrefixes(ed.raw) {
				continue
			}
			checkAgainstOracle(t, where, ed)
		}
	}
	if profiles == 0 {
		t.Fatal("no profile regenerated: the test checks nothing")
	}
	t.Logf("%d profiles regenerated, %d elements", profiles, elements)
}

// T2: las ElementDefinition cuyo raw no pasó por la extracción responden lo mismo que el oráculo,
// y un valor guardado deja de usarse si raw cambia.
func TestPrefixedValuesWithoutExtraction(t *testing.T) {
	const withFixed = `{"path":"Extension.url","fixedUri":"http://example.org/a","_fixedUri":{"id":"x"}}`
	const withPattern = `{"path":"Observation.code","patternCodeableConcept":{"text":"b"}}`

	t.Run("raw assigned directly", func(t *testing.T) {
		ed := ElementDefinition{raw: json.RawMessage(withFixed)}
		checkAgainstOracle(t, "direct", &ed)
	})
	t.Run("SetRaw twice", func(t *testing.T) {
		var ed ElementDefinition
		ed.SetRaw(json.RawMessage(withFixed))
		ed.SetRaw(json.RawMessage(withPattern))
		checkAgainstOracle(t, "SetRaw", &ed)
		if _, _, ok := ed.GetFixed(); ok {
			t.Error("GetFixed answers for the raw set before")
		}
	})
	t.Run("nil raw", func(t *testing.T) {
		var ed ElementDefinition
		checkAgainstOracle(t, "nil", &ed)
	})
	t.Run("invalid JSON", func(t *testing.T) {
		ed := ElementDefinition{raw: json.RawMessage(`{"fixedUri":`)}
		checkAgainstOracle(t, "invalid", &ed)
	})
	t.Run("raw replaced after extraction", func(t *testing.T) {
		ed := extracted(t, withFixed)
		if ed.cachedPrefixed() == nil {
			t.Fatal("no prefixed values kept after the snapshot was decoded")
		}
		ed.raw = json.RawMessage(withPattern)
		if ed.cachedPrefixed() != nil {
			t.Error("prefixed values of the previous raw are still used")
		}
		checkAgainstOracle(t, "replaced", ed)
	})
	// Un raw nuevo del mismo largo: el valor guardado se reconoce por el arreglo, no por el largo.
	const sameLengthA = `{"path":"Extension.url","fixedUri":"http://example.org/aaa"}`
	const sameLengthB = `{"path":"Extension.url","fixedUri":"http://example.org/bbb"}`
	t.Run("raw replaced by one of the same length", func(t *testing.T) {
		ed := extracted(t, sameLengthA)
		ed.raw = json.RawMessage(sameLengthB)
		if v, _, _ := ed.GetFixed(); string(v) != `"http://example.org/bbb"` {
			t.Errorf("GetFixed = %s after raw changed to a same-length JSON", v)
		}
	})
	t.Run("SetRaw with one of the same length", func(t *testing.T) {
		ed := extracted(t, sameLengthA)
		ed.SetRaw(json.RawMessage(sameLengthB))
		if v, _, _ := ed.GetFixed(); string(v) != `"http://example.org/bbb"` {
			t.Errorf("GetFixed = %s after SetRaw with a same-length JSON", v)
		}
	})
}

// extracted decodifica el elemento como parte de un snapshot, el camino que guarda los valores.
func extracted(t *testing.T, element string) *ElementDefinition {
	t.Helper()
	var s Snapshot
	if err := json.Unmarshal([]byte(`{"element":[`+element+`]}`), &s); err != nil {
		t.Fatal(err)
	}
	return &s.Element[0]
}

// T3: casos escritos a mano desde la especificación. Cada caso se prueba por el camino que guarda
// los valores (un snapshot decodificado) y por el que no (raw asignado directo).
func TestPrefixedValuesCases(t *testing.T) {
	type answer struct {
		value  string // JSON compacto; vacío: no existe
		suffix string
	}
	tests := []struct {
		name                   string
		element                string
		fixed, pattern         answer
		fixedElem, patternElem string // JSON compacto; vacío: no existe
	}{
		{
			// json.html#choice: el nombre del elemento seguido del tipo con la primera letra mayúscula.
			name:    "fixedUri",
			element: `{"path":"Extension.url","fixedUri":"http://example.org/ext"}`,
			fixed:   answer{`"http://example.org/ext"`, "Uri"},
		},
		{
			name:    "patternCodeableConcept",
			element: `{"path":"Observation.code","patternCodeableConcept":{"coding":[{"system":"http://loinc.org","code":"8480-6"}]}}`,
			pattern: answer{`{"coding":[{"system":"http://loinc.org","code":"8480-6"}]}`, "CodeableConcept"},
		},
		{
			// json.html#primitive: el id y las extensiones de un primitivo van en la propiedad con "_".
			name:      "fixedCode with its _fixedCode",
			element:   `{"path":"Observation.status","fixedCode":"final","_fixedCode":{"extension":[{"url":"http://example.org/e","valueString":"x"}]}}`,
			fixed:     answer{`"final"`, "Code"},
			fixedElem: `{"extension":[{"url":"http://example.org/e","valueString":"x"}]}`,
		},
		{
			// json.html#primitive: un primitivo puede tener extensiones y no tener valor.
			name:      "_fixedCode without a value",
			element:   `{"path":"Observation.status","_fixedCode":{"id":"a"}}`,
			fixedElem: `{"id":"a"}`,
		},
		{
			// json.html#primitive: el "_" acompaña a la propiedad del mismo nombre, no a otra.
			name:    "fixedUri with the _ of another type",
			element: `{"path":"Extension.url","fixedUri":"http://example.org/ext","_fixedCode":{"id":"a"}}`,
			fixed:   answer{`"http://example.org/ext"`, "Uri"},
		},
		{
			// RFC 8259 §7: un nombre puede escribirse con escapes \uXXXX; "\u0066ixedUri" es fixedUri.
			name:    "fixedUri written with an escape",
			element: `{"path":"Extension.url","\u0066ixedUri":"http://example.org/ext"}`,
			fixed:   answer{`"http://example.org/ext"`, "Uri"},
		},
		{
			name:    "neither fixed nor pattern",
			element: `{"path":"Patient.name","min":0,"max":"*"}`,
		},
		{
			// eld-8 prohíbe fixed y pattern a la vez; extraerlos no valida la definición: se
			// devuelven ambos, como hasta ahora.
			name:    "fixed and pattern together",
			element: `{"path":"Patient.gender","fixedCode":"female","patternCode":"male"}`,
			fixed:   answer{`"female"`, "Code"},
			pattern: answer{`"male"`, "Code"},
		},
		{
			// Dos claves del mismo prefijo no son JSON FHIR válido (json.html#choice: una sola
			// propiedad por elemento de elección), pero mergeElements puede producirlas. Regla
			// declarada en el plan: la clave lexicográficamente menor, y su propio "_".
			name:      "two fixed keys",
			element:   `{"path":"Extension.url","fixedUri":"http://example.org/ext","fixedCode":"c","_fixedUri":{"id":"u"},"_fixedCode":{"id":"c"}}`,
			fixed:     answer{`"c"`, "Code"},
			fixedElem: `{"id":"c"}`,
		},
	}
	for _, tt := range tests {
		paths := map[string]*ElementDefinition{
			"kept":     extracted(t, tt.element),
			"computed": {raw: json.RawMessage(tt.element)},
		}
		for pathName, ed := range paths {
			t.Run(tt.name+"/"+pathName, func(t *testing.T) {
				for _, c := range []struct {
					what string
					want answer
					get  func() (json.RawMessage, string, bool)
				}{
					{"GetFixed", tt.fixed, ed.GetFixed},
					{"GetPattern", tt.pattern, ed.GetPattern},
				} {
					v, s, ok := c.get()
					if ok != (c.want.value != "") || !jsonEqualString(v, c.want.value) || s != c.want.suffix {
						t.Errorf("%s = (%s, %q, %v), want (%s, %q)", c.what, v, s, ok, c.want.value, c.want.suffix)
					}
				}
				for _, c := range []struct {
					what string
					want string
					get  func() (json.RawMessage, bool)
				}{
					{"GetFixedElement", tt.fixedElem, ed.GetFixedElement},
					{"GetPatternElement", tt.patternElem, ed.GetPatternElement},
				} {
					v, ok := c.get()
					if ok != (c.want != "") || !jsonEqualString(v, c.want) {
						t.Errorf("%s = (%s, %v), want %s", c.what, v, ok, c.want)
					}
				}
			})
		}
	}
}

func jsonEqualString(got json.RawMessage, want string) bool {
	if want == "" {
		return got == nil
	}
	return jsonEqual(got, json.RawMessage(want))
}

// T4: los bytes devueltos son del llamador. Cambiarlos, o cambiar los pasados a SetRaw, no cambia
// las respuestas siguientes, ni en el elemento, ni en una copia por valor, ni en un Clone del
// registro.
func TestPrefixedValuesAreNotShared(t *testing.T) {
	const element = `{"path":"Observation.status","fixedCode":"final","_fixedCode":{"id":"a"},"patternCode":"x","_patternCode":{"id":"b"}}`

	scribble := func(b []byte) {
		for i := range b {
			b[i] = '#'
		}
	}
	check := func(t *testing.T, ed *ElementDefinition) {
		t.Helper()
		v, _, _ := ed.GetFixed()
		e, _ := ed.GetFixedElement()
		p, _, _ := ed.GetPattern()
		pe, _ := ed.GetPatternElement()
		for _, b := range [][]byte{v, e, p, pe} {
			scribble(b)
		}
		if v, _, _ := ed.GetFixed(); string(v) != `"final"` {
			t.Errorf("GetFixed = %s after the caller changed a previous answer", v)
		}
		if e, _ := ed.GetFixedElement(); string(e) != `{"id":"a"}` {
			t.Errorf("GetFixedElement = %s after the caller changed a previous answer", e)
		}
		if p, _, _ := ed.GetPattern(); string(p) != `"x"` {
			t.Errorf("GetPattern = %s after the caller changed a previous answer", p)
		}
		if pe, _ := ed.GetPatternElement(); string(pe) != `{"id":"b"}` {
			t.Errorf("GetPatternElement = %s after the caller changed a previous answer", pe)
		}
	}

	t.Run("kept", func(t *testing.T) { check(t, extracted(t, element)) })
	t.Run("computed", func(t *testing.T) { check(t, &ElementDefinition{raw: json.RawMessage(element)}) })
	t.Run("copy by value", func(t *testing.T) {
		ed := extracted(t, element)
		cp := *ed
		check(t, &cp)
		check(t, ed)
	})
	t.Run("SetRaw input", func(t *testing.T) {
		input := []byte(element)
		var ed ElementDefinition
		ed.SetRaw(input)
		scribble(input)
		if v, _, ok := ed.GetFixed(); !ok || string(v) != `"final"` {
			t.Errorf("GetFixed = %s, %v after the caller changed the bytes given to SetRaw", v, ok)
		}
	})
	t.Run("registry clone", func(t *testing.T) {
		r := sharedVersion(t, "4.0.1")
		url := profileWithPattern(t, r)
		ed := firstWithPattern(t, r.GetByURL(url))
		want, _, _ := ed.GetPattern()
		clone := r.Clone()
		cloned := firstWithPattern(t, clone.GetByURL(url))
		v, _, _ := cloned.GetPattern()
		scribble(v)
		if got, _, _ := ed.GetPattern(); !bytes.Equal(got, want) {
			t.Errorf("the original's pattern changed through its clone: %s", got)
		}
	})
}

// profileWithPattern devuelve la URL del primer perfil cargado con algún pattern[x] en su snapshot.
func profileWithPattern(t *testing.T, r *Registry) string {
	t.Helper()
	for _, sd := range r.all {
		if sd.Snapshot == nil || sd.Derivation != DerivationConstraint {
			continue
		}
		for i := range sd.Snapshot.Element {
			if _, _, ok := sd.Snapshot.Element[i].GetPattern(); ok {
				return sd.URL
			}
		}
	}
	t.Fatal("no profile with pattern[x]")
	return ""
}

func firstWithPattern(t *testing.T, sd *StructureDefinition) *ElementDefinition {
	t.Helper()
	if sd == nil || sd.Snapshot == nil {
		t.Fatal("profile not loaded")
	}
	for i := range sd.Snapshot.Element {
		if _, _, ok := sd.Snapshot.Element[i].GetPattern(); ok {
			return &sd.Snapshot.Element[i]
		}
	}
	t.Fatal("no element with pattern[x]")
	return nil
}
