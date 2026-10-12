package structural

import (
	"fmt"

	"github.com/gofhir/validator/v2/pkg/issue"
)

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
// object for a complex type or a resource, a simple value for a primitive (a null only where its
// "_key" sibling holds its id and extensions). It returns the value to check the content of, at
// path, and false when there is none: an array written for an element that does not repeat (the
// HL7 validator checks nothing in it). A single value written where an array goes is checked as
// written, at the element's path without an index, as HL7's parser names what it finds in it.
func (v *Validator) checkShape(data map[string]any, key string, value any, resolved *resolvedElement, path string, result *issue.Result) (any, bool) {
	primitive, known := v.primitiveKind(resolved)
	if !known {
		return value, true
	}
	items, isArray := value.([]any)
	switch {
	case isArray && !repeats(resolved):
		v.reportShape(primitive, key, foundArray, path, result)
		return nil, false
	case isArray:
		siblings, _ := data["_"+key].([]any)
		var reported map[string]bool // a primitive array's items are reported at the array: once per shape
		if primitive {
			reported = map[string]bool{}
		}
		for i, item := range items {
			var sibling any
			if i < len(siblings) {
				sibling = siblings[i]
			}
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if primitive {
				// HL7 names a primitive array's item by the array ("The JSON array for 'given' is
				// invalid at index 1"), and reports it once.
				if reported[foundShape(item)] {
					continue
				}
				itemPath = path
			}
			if v.checkItem(primitive, key, item, sibling, itemPath, result) && primitive {
				reported[foundShape(item)] = true
			}
		}
		return value, true
	case repeats(resolved):
		result.AddErrorWithID(issue.DiagStructureArrayExpected,
			map[string]any{"element": key, "found": foundShape(value)}, path)
		if m, ok := value.(map[string]any); ok {
			return m, true
		}
		return nil, false
	}
	// A single value has no null: the "_key" sibling holds a primitive's id and extensions when the
	// value is absent, not null (json.html#primitive).
	v.checkItem(primitive, key, value, nil, path, result)
	return value, true
}

// checkItem reports value, one value of key at path, that is not of its element's kind: a primitive
// is a simple value, or a null where sibling, its "_key" item, holds its id and extensions; another
// type's value is an object. It reports whether it reported.
func (v *Validator) checkItem(primitive bool, key string, value, sibling any, path string, result *issue.Result) bool {
	found := foundShape(value)
	switch {
	case primitive && (found == foundObject || found == foundArray):
	case primitive && found == foundNull:
		if _, extended := sibling.(map[string]any); extended {
			return false
		}
	case !primitive && found != foundObject:
	default:
		return false
	}
	v.reportShape(primitive, key, found, path, result)
	return true
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

// checkShadowShape reports a primitive's "_key" sibling (value, at path, its element's) written
// otherwise than json.html#primitive says: an array of objects or nulls, aligned with the values,
// when the element repeats; an object otherwise. It reports whether value has that shape.
func (v *Validator) checkShadowShape(key string, value any, resolved *resolvedElement, path string, result *issue.Result) bool {
	sibling := "_" + key
	items, isArray := value.([]any)
	if !repeats(resolved) {
		if _, ok := value.(map[string]any); !ok {
			result.AddErrorWithID(issue.DiagStructureObjectExpected,
				map[string]any{"element": sibling, "found": foundShape(value)}, path)
			return false
		}
		return true
	}
	if !isArray {
		result.AddErrorWithID(issue.DiagStructureArrayExpected,
			map[string]any{"element": sibling, "found": foundShape(value)}, path)
		return false
	}
	reported := map[string]bool{}
	for _, item := range items {
		if f := foundShape(item); f != foundObject && f != foundNull && !reported[f] {
			// at the element, once per shape, as HL7 reports it
			reported[f] = true
			result.AddErrorWithID(issue.DiagStructureObjectExpected,
				map[string]any{"element": sibling, "found": f}, path)
		}
	}
	return true
}
