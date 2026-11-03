package structural

import (
	"fmt"
	"slices"

	"github.com/gofhir/validator/v2/internal/versionorder"
	"github.com/gofhir/validator/v2/pkg/issue"
)

// emptyPrimitiveArrayReleases son los releases de FHIR cuyo json.html dice que un array nunca está
// vacío, y por eso rechazan el de un primitivo (given: []) y el de su "_x" (_given: []): DSTU2 (1.0),
// STU3 (3.0), R4 (4.0) y R4B (4.3), «Just as in XML, JSON objects and arrays are never empty, and
// properties never have null values (except for a special case documented below). Omit a property if it
// is empty» (json.html#2.6.2.1, «Comparison with XML»). R5 ya no tiene esa frase: un "_x" más corto que
// sus valores infiere nulls, y un array de puros nulls solo se recomienda omitir (json.html#2.1.6.4.2,
// SHOULD). Una versión fuera de la lista, o desconocida, sigue la regla de R5, el texto vigente
// (ADR-003). El array vacío de un complejo o un recurso se rechaza en todas.
var emptyPrimitiveArrayReleases = []string{"1.0", "3.0", "4.0", "4.3"}

// emptyPrimitiveArraysForbidden reporta si el json.html de la versión de FHIR dada rechaza el array
// vacío de un primitivo y el de su "_x" (emptyPrimitiveArrayReleases). Una versión vacía no lo rechaza.
func emptyPrimitiveArraysForbidden(version string) bool {
	return version != "" && slices.ContainsFunc(emptyPrimitiveArrayReleases, func(release string) bool {
		return versionorder.SameRelease(release, version)
	})
}

// Shapes a JSON value takes, as the diagnostics name them.
const (
	foundObject    = "an object"
	foundArray     = "an array"
	foundNull      = "null"
	foundPrimitive = "a primitive value"
)

// foundShape names the shape of a JSON value.
func foundShape(value any) string {
	switch value.(type) {
	case map[string]any:
		return foundObject
	case []any:
		return foundArray
	case nil:
		return foundNull
	}
	return foundPrimitive
}

// repeats reports whether the element resolved repeats (registry.ElementDefinition.Repeats).
func repeats(resolved *resolvedElement) bool {
	return resolved.elemDef.Repeats()
}

// primitiveKind reports whether a value of the element is a primitive (a simple value) or else an
// object (a complex type, a resource, an element whose content is another's), and whether that is
// known: an element of several types is not, outside a choice.
func (v *Validator) primitiveKind(resolved *resolvedElement) (primitive, known bool) {
	if resolved.contentRefPath != "" || v.registry.HoldsResource(resolved.elemDef) {
		return false, true
	}
	typeName := resolved.resolvedType
	if typeName == "" {
		return false, false
	}
	return v.registry.IsPrimitiveTypeCode(typeName), true
}

// checkShape reports a value of key, an element of data at path, written otherwise than the
// element says (json.html): an array for an element that repeats and a single value otherwise, an
// object for a complex type or a resource, a simple value for a primitive (a null only as an array's
// item whose "_key" item holds its id and extensions). It returns the value to check the content
// of, at path, and false when there is none: an array written for an element that does not repeat
// (the HL7 validator checks nothing in it). A single value written where an array goes is checked
// as written, at the element's path without an index, as HL7's parser names what it finds in it.
// With ctx.edits, it records how the HL7 validator's parser leaves the instance (normalize).
func (v *Validator) checkShape(data map[string]any, key string, value any, resolved *resolvedElement, path string, ctx *validationContext, result *issue.Result) (any, bool) {
	primitive, known := v.primitiveKind(resolved)
	items, isArray := value.([]any)
	if isArray && len(items) == 0 && repeats(resolved) {
		// Antes de saber el tipo: solo depende de que el elemento se repita (un no repetido escrito [] deja
		// el error de forma de abajo)
		v.checkEmptyArray(data, key, path, primitive, ctx, result)
		return value, true
	}
	if !known {
		return value, true
	}
	switch {
	case isArray && !repeats(resolved):
		v.reportShape(primitive, key, foundArray, path, result)
		ctx.edits.drop(data, key, primitive)
		return nil, false
	case isArray:
		v.checkItems(data, key, items, primitive, path, ctx, result)
		return value, true
	case repeats(resolved):
		result.AddErrorWithID(issue.DiagStructureArrayExpected,
			map[string]any{"element": key, "found": foundShape(value)}, path)
		if m, ok := value.(map[string]any); ok && !primitive {
			if _, empty := v.emptyObject(ctx, primitive, m); empty {
				v.reportEmpty(key, path, result)
				ctx.edits.empty(data, m)
			}
			// the array's one item, as HL7 reads it
			ctx.edits.wrap(data, key, path)
			return m, true
		}
		ctx.edits.drop(data, key, primitive) // HL7 drops it
		return nil, false
	}
	// A single value has no null: the "_key" sibling holds a primitive's id and extensions when the
	// value is absent, not null (json.html#primitive).
	if m, empty := v.emptyObject(ctx, primitive, value); empty {
		v.reportEmpty(key, path, result)
		ctx.edits.empty(data, m)
		return value, true
	}
	if misshapenItem(primitive, value, nil) {
		v.reportShape(primitive, key, foundShape(value), path, result)
		ctx.edits.drop(data, key, primitive)
	}
	return value, true
}

// checkItems revisa los items de key, un array de data en path (un elemento que se repite), y reporta
// los que están escritos de otro modo que como dice el elemento; con ctx.edits, registra cómo los deja
// el parser de HL7.
func (v *Validator) checkItems(data map[string]any, key string, items []any, primitive bool, path string, ctx *validationContext, result *issue.Result) {
	siblings, _ := data["_"+key].([]any)
	var reported map[string]bool // a primitive array's items are reported at the array: once per shape
	for i, item := range items {
		var sibling any
		if i < len(siblings) {
			sibling = siblings[i]
		}
		if m, empty := v.emptyObject(ctx, primitive, item); empty {
			v.reportEmpty(key, fmt.Sprintf("%s[%d]", path, i), result)
			ctx.edits.empty(data, m)
			continue
		}
		if !misshapenItem(primitive, item, sibling) {
			continue
		}
		ctx.edits.dropItem(data, key, i, primitive, path)
		found := foundShape(item)
		if !primitive {
			v.reportShape(primitive, key, found, fmt.Sprintf("%s[%d]", path, i), result)
		} else if !reported[found] {
			// HL7 names a primitive array's item by the array ("The JSON array for 'given' is
			// invalid at index 1"), and reports it once.
			if reported == nil {
				reported = map[string]bool{}
			}
			reported[found] = true
			v.reportShape(primitive, key, found, path, result)
		}
	}
}

// checkEmptyArray reporta key, una propiedad de data en path que se repite, escrita como array vacío:
// json.html dice que una propiedad sin valores se omite. Se reporta en el elemento, sin índice, como lo
// nombra el parser de HL7, y solo sobre la fuente (ctx.edits, igual que un objeto vacío). El de un
// primitivo se reporta solo donde el json.html de su release dice que los arrays nunca están vacíos
// (emptyPrimitiveArraysForbidden). Reporte o no, data se juzga por su contenido tal como está escrito:
// tiene una propiedad, así que no es un elemento sin contenido (ele-1).
func (v *Validator) checkEmptyArray(data map[string]any, key, path string, primitive bool, ctx *validationContext, result *issue.Result) {
	if ctx.edits == nil {
		return
	}
	ctx.edits.holders.add(data)
	if primitive && !emptyPrimitiveArraysForbidden(v.registry.FHIRVersion()) {
		return
	}
	result.AddErrorWithID(issue.DiagStructureArrayEmpty, map[string]any{"element": key}, path)
}

// emptyObject reports whether value, one value of a complex element or a resource an element
// holds, is written as an empty object: an element has a value or children
// (ele-1), and HL7's parser reports such an object ("Object must have some content"). It is left in
// the instance (cardinality counts it, as HL7's does), and it and the element that holds it are
// judged by their content as written, not again by ele-1. It is judged on the source, as Normalize
// reads it (ctx.edits): an object the normalization empties had content.
func (v *Validator) emptyObject(ctx *validationContext, primitive bool, value any) (m map[string]any, empty bool) {
	m, ok := value.(map[string]any)
	return m, ok && len(m) == 0 && ctx.edits != nil && !primitive
}

// reportEmpty reports a value of key at path written as an empty object.
func (v *Validator) reportEmpty(key, path string, result *issue.Result) {
	result.AddErrorWithID(issue.DiagStructureEmptyObject, map[string]any{"element": key}, path)
}

// misshapenItem reports whether value, one value of an element, is not of the element's kind: a
// primitive is a simple value, or a null where sibling, its "_key" item, holds its id and
// extensions; another type's value is an object.
func misshapenItem(primitive bool, value, sibling any) bool {
	switch found := foundShape(value); {
	case primitive && (found == foundObject || found == foundArray):
		return true
	case primitive && found == foundNull:
		_, extended := sibling.(map[string]any)
		return !extended
	default:
		return !primitive && found != foundObject
	}
}

// reportShape reports a value of key at path found where a simple value (a primitive's) or an
// object goes.
func (v *Validator) reportShape(primitive bool, key, found, path string, result *issue.Result) {
	id := issue.DiagStructureObjectExpected
	if primitive {
		id = issue.DiagStructureValueExpected
	}
	result.AddErrorWithID(id, map[string]any{"element": key, "found": found}, path)
}

// checkShadowShape reports a primitive's "_key" sibling (value, of data, at path, its element's)
// written otherwise than json.html#primitive says: an array of objects or nulls, aligned with the
// values, when the element repeats; an object otherwise. It reports whether value has that shape.
// HL7's parser reads a primitive's value and its "_key" together: one misshapen, it drops both (the
// element, or the item and its value).
func (v *Validator) checkShadowShape(data map[string]any, key string, value any, resolved *resolvedElement, path string, ctx *validationContext, result *issue.Result) bool {
	sibling := "_" + key
	items, isArray := value.([]any)
	if !repeats(resolved) {
		m, ok := value.(map[string]any)
		if !ok {
			result.AddErrorWithID(issue.DiagStructureObjectExpected,
				map[string]any{"element": sibling, "found": foundShape(value)}, path)
			ctx.edits.drop(data, key, true)
			return false
		}
		if len(m) == 0 && ctx.edits != nil {
			// no id nor extensions: an empty object, at its element, as HL7's parser reports it
			v.reportEmpty(sibling, path, result)
		}
		return true
	}
	if !isArray {
		result.AddErrorWithID(issue.DiagStructureArrayExpected,
			map[string]any{"element": sibling, "found": foundShape(value)}, path)
		ctx.edits.drop(data, key, true)
		return false
	}
	if len(items) == 0 {
		v.checkEmptyArray(data, sibling, path, true, ctx, result)
		return true
	}
	reported := map[string]bool{}
	for i, item := range items {
		f := foundShape(item)
		if m, ok := item.(map[string]any); ok && len(m) == 0 && ctx.edits != nil && !reported["empty"] {
			reported["empty"] = true
			v.reportEmpty(sibling, path, result)
		}
		if f == foundObject || f == foundNull {
			continue
		}
		ctx.edits.dropItem(data, key, i, true, path)
		if !reported[f] {
			// at the element, once per shape, as HL7 reports it
			reported[f] = true
			result.AddErrorWithID(issue.DiagStructureObjectExpected,
				map[string]any{"element": sibling, "found": f}, path)
		}
	}
	return true
}
