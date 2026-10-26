package registry

import (
	"bytes"
	"encoding/json"
	"strings"
)

// prefixedValue es una propiedad de una ElementDefinition cuyo nombre empieza con un prefijo:
// fixed[x] o pattern[x] (fixedUri: sufijo "Uri"), o su hermano "_" (json.html#primitive).
type prefixedValue struct {
	value  json.RawMessage
	suffix string
	ok     bool
}

// prefixedValues son las respuestas de GetFixed, GetPattern, GetFixedElement y GetPatternElement
// para un raw, calculadas en una sola decodificación. Una vez creado no cambia, así que las copias
// por valor de la ElementDefinition y las goroutines lo usan sin copiarlo.
type prefixedValues struct {
	fixed, pattern               prefixedValue
	fixedElement, patternElement prefixedValue
}

// noPrefixedValues es la respuesta compartida de un raw sin fixed[x], pattern[x] ni sus "_": así,
// la mayoría de los elementos de un snapshot (el 92 % en R4) no asigna una propia.
var noPrefixedValues = &prefixedValues{}

func (p *prefixedValues) empty() bool {
	return !p.fixed.ok && !p.pattern.ok && !p.fixedElement.ok && !p.patternElement.ok
}

// extractPrefixedValues decodifica raw una vez y resuelve fixed[x], pattern[x] y sus hermanos "_".
// Con dos claves del mismo prefijo, que JSON FHIR válido no tiene (json.html#choice) pero
// mergeElements puede producir, elige la lexicográficamente menor; el "_" se busca para esa misma
// propiedad. Un raw nulo o inválido no tiene ninguna.
func extractPrefixedValues(raw json.RawMessage) *prefixedValues {
	if raw == nil || !mayHavePrefixed(raw) {
		return noPrefixedValues
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return noPrefixedValues
	}
	p := &prefixedValues{}
	p.fixed = smallestWithPrefix(obj, "fixed")
	p.pattern = smallestWithPrefix(obj, "pattern")
	p.fixedElement = siblingOf(obj, "fixed", p.fixed)
	p.patternElement = siblingOf(obj, "pattern", p.pattern)
	if p.empty() {
		return noPrefixedValues
	}
	return p
}

// mayHavePrefixed informa si raw puede tener una propiedad fixed…, pattern… o sus "_": sin esos
// textos no puede, salvo que un nombre esté escrito con escapes \uXXXX (RFC 8259 §7), que se
// decodifican antes de comparar. Evita decodificar el 92 % de los elementos (R4), que no tienen.
func mayHavePrefixed(raw []byte) bool {
	return bytes.Contains(raw, []byte("fixed")) || bytes.Contains(raw, []byte("pattern")) ||
		bytes.Contains(raw, []byte(`\u`))
}

// smallestWithPrefix devuelve la propiedad de obj de menor nombre entre las que empiezan con prefix.
func smallestWithPrefix(obj map[string]json.RawMessage, prefix string) prefixedValue {
	best := ""
	found := false
	for key := range obj {
		if strings.HasPrefix(key, prefix) && (!found || key < best) {
			best, found = key, true
		}
	}
	if !found {
		return prefixedValue{}
	}
	return prefixedValue{value: obj[best], suffix: strings.TrimPrefix(best, prefix), ok: true}
}

// siblingOf devuelve el hermano "_" de la propiedad elegida (fixedCode → _fixedCode); sin propiedad
// (un primitivo con extension y sin valor), cualquier "_" del prefijo.
func siblingOf(obj map[string]json.RawMessage, prefix string, chosen prefixedValue) prefixedValue {
	if chosen.ok {
		value, ok := obj["_"+prefix+chosen.suffix]
		return prefixedValue{value: value, ok: ok}
	}
	return smallestWithPrefix(obj, "_"+prefix)
}

// keepPrefixedValues guarda las respuestas de ed para su raw actual. Se llama antes de que la
// ElementDefinition sea visible para otras goroutines.
func (ed *ElementDefinition) keepPrefixedValues() {
	ed.prefixed = extractPrefixedValues(ed.raw)
	ed.prefixedSrc = ed.raw
}

// cachedPrefixed devuelve las respuestas guardadas si siguen siendo las del raw actual: el mismo
// slice, no solo los mismos bytes. Nil si no hay o si raw cambió después.
func (ed *ElementDefinition) cachedPrefixed() *prefixedValues {
	p := ed.prefixed
	if p == nil || !sameSlice(ed.prefixedSrc, ed.raw) {
		return nil
	}
	return p
}

// prefixedValuesOf devuelve las respuestas guardadas o, si no las hay, las calcula sin guardarlas.
func (ed *ElementDefinition) prefixedValuesOf() *prefixedValues {
	if p := ed.cachedPrefixed(); p != nil {
		return p
	}
	return extractPrefixedValues(ed.raw)
}

// sameSlice informa si a y b son el mismo slice: mismo largo y mismo arreglo de respaldo.
func sameSlice(a, b []byte) bool {
	if len(a) != len(b) || (a == nil) != (b == nil) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
}

// answer devuelve una copia del valor: los bytes son del llamador.
func (v prefixedValue) answer() (value json.RawMessage, suffix string, ok bool) {
	if !v.ok {
		return nil, "", false
	}
	return bytes.Clone(v.value), v.suffix, true
}
