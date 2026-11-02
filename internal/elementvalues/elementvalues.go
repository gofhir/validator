// Package elementvalues reads the values an instance holds for an element of a StructureDefinition
// tree, the same way for every phase that walks the tree, so that each names a value by the same
// path.
package elementvalues

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gofhir/validator/v2/pkg/registry"
)

// Value is one value of an element in an instance.
type Value struct {
	Key      string         // the JSON property it is read from ("valueQuantity" for value[x])
	TypeCode string         // its type: the one a choice property names, or the element's only type
	Value    any            // nil for a primitive present only through its "_key" sibling, or a null
	Null     bool           // the property, or its array's item, is written as a JSON null
	Array    bool           // read from an array
	Index    int            // its position in the array, when Array
	Ext      map[string]any // a primitive's id and extensions, from its "_key" sibling
}

// Path is the value's path under the path of the instance that holds it.
func (v Value) Path(parent string) string {
	p := parent + "." + v.Key
	if v.Array {
		p = fmt.Sprintf("%s[%d]", p, v.Index)
	}
	return p
}

// Types tells what the values' types are: the type a choice property names among those of the
// element's base (Registry.ChoiceType) and whether a type is primitive (Registry.IsPrimitiveTypeCode).
// *registry.Registry is one.
type Types interface {
	ChoiceType(node *registry.ElementNode, key string) string
	IsPrimitiveTypeCode(code string) bool
}

// Of returns the values inst holds for child: under its name, or for a choice element
// ("value[x]") under each name its types give it, the element name followed by the type code with
// its first letter capitalized (formats.html#choice), then under any other property that
// types.ChoiceType says names a type. A value of a type the element does not allow is present,
// with the wrong type, not absent; one the element's base does not declare is no value of it. Only a primitive has a "_name" sibling (json.html#primitive): a
// primitive present only through it (extensions without a value) is present, once per entry of a
// repeating one; another type's "_name" is no property of it, and is not read.
func Of(child *registry.ElementNode, inst map[string]any, types Types) []Value {
	name := child.Name()
	base, isChoice := strings.CutSuffix(name, "[x]")
	if !isChoice {
		typeCode := ""
		if len(child.Def.Type) == 1 {
			typeCode = child.Def.Type[0].Code
		}
		return valuesOf(inst, name, typeCode, primitive(types, typeCode))
	}
	props := choiceProperties(base, child, inst, types)
	if len(props) == 0 {
		return nil
	}
	out := make([]Value, 0, len(props))
	for _, p := range props {
		out = append(out, valuesOf(inst, p.key, p.code, primitive(types, p.code))...)
	}
	return out
}

// property is a JSON property a choice element is read from, with the type it names and its rank:
// a type's position among the element's, or past them for another type choiceType names.
type property struct {
	key, code string
	rank      int
}

// choiceProperties returns the properties of inst a choice element node of base is read from, those
// whose type Types.ChoiceType finds among the base element's: the ones node's types name (or their
// "_key" siblings name), in the types' order, then the others (or, for a primitive one, their "_key"
// siblings), by key.
// Only inst's own keys are looked at, not every name the types give.
func choiceProperties(base string, node *registry.ElementNode, inst map[string]any, t Types) []property {
	types := node.Def.Type
	var out []property
	for k := range inst {
		key, code, ok := variantOf(base, node, k, inst, t)
		if !ok {
			continue
		}
		rank := len(types)
		if i := registry.ChoiceTypeIndex(types, key[len(base):]); i >= 0 {
			code, rank = types[i].Code, i
		}
		out = append(out, property{key: key, code: code, rank: rank})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].rank != out[j].rank {
			return out[i].rank < out[j].rank
		}
		return out[i].key < out[j].key
	})
	return out
}

// variantOf dice si k, una clave de inst, es una variante del elemento de elección node de nombre
// base: devuelve el nombre de la variante (k sin el "_" de un hermano) y su tipo. Es la única
// definición de variante, la que usan las fases estructural, de cardinalidad y de slicing: una
// propiedad que nombra un tipo que la base del elemento declara (formats.html#choice), o el hermano
// "_x" de un primitivo sin el valor (json.html#primitive); el hermano de otro tipo no es propiedad.
// Un tipo que solo declara el perfil no lo es (CodeableReference de un package de R5 cargado en R4,
// #180), y uno que el perfil no permite y la base sí es una variante, de tipo equivocado (#178).
func variantOf(base string, node *registry.ElementNode, k string, inst map[string]any, t Types) (key, code string, ok bool) {
	key, underscored := strings.CutPrefix(k, "_")
	if len(key) <= len(base) || !strings.HasPrefix(key, base) {
		return "", "", false
	}
	if underscored {
		if _, hasValue := inst[key]; hasValue {
			return "", "", false // the value's key stands for both
		}
	}
	code = t.ChoiceType(node, key)
	if code == "" || (underscored && !primitive(t, code)) {
		return "", "", false
	}
	return key, code, true
}

// CompetingVariants returns the names of the variants inst holds of node, a choice element, in
// name order, when there are two or more (they are the same fact as a repeated element,
// formats.html#choice): nil otherwise. The profile does not decide which is the value, so the
// result is the same for a resource validated against its profile or against its base type. It
// allocates only when it finds a conflict.
func CompetingVariants(node *registry.ElementNode, inst map[string]any, types Types) []string {
	base, isChoice := strings.CutSuffix(node.Def.Path[strings.LastIndex(node.Def.Path, ".")+1:], "[x]")
	if !isChoice {
		return nil
	}
	var first string
	var out []string
	for k := range inst {
		key, _, ok := variantOf(base, node, k, inst, types)
		switch {
		case !ok:
		case first == "":
			first = key
		default:
			if out == nil {
				out = append(make([]string, 0, 3), first)
			}
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// Occurrences returns how many times child occurs given its values, those Of returned (or the
// ones of them that count): for a choice element the variants present are one occurrence
// (formats.html#choice), not one per variant; of an element that repeats, the items of its variant
// with the most. Cardinality and slicing count with it, so that two variants are the one issue the
// structural phase reports (CompetingVariants) and not also a cardinality one. It assumes the values
// of one key are contiguous, as Of returns them (by key, in name order).
func Occurrences(child *registry.ElementNode, values []Value) int {
	if !strings.HasSuffix(child.Def.Path, "[x]") {
		return len(values)
	}
	most, run := 0, 0
	for i, cv := range values {
		if i > 0 && cv.Key == values[i-1].Key {
			run++
		} else {
			run = 1
		}
		most = max(most, run)
	}
	return most
}

// primitive reports whether typeCode is a primitive type's, which a "_key" sibling may extend.
func primitive(types Types, typeCode string) bool {
	return typeCode != "" && types.IsPrimitiveTypeCode(typeCode)
}

// valuesOf returns the values of one property, each with its "_key" sibling when the property is
// a primitive.
func valuesOf(inst map[string]any, k, typeCode string, primitive bool) []Value {
	val, ok := inst[k]
	var ext any
	if primitive {
		ext = inst["_"+k]
	}
	if !ok {
		switch e := ext.(type) {
		case nil:
			return nil
		case []any:
			out := make([]Value, 0, len(e))
			for i, item := range e {
				out = append(out, Value{Key: k, TypeCode: typeCode, Array: true, Index: i, Ext: asMap(item)})
			}
			return out
		default:
			return []Value{{Key: k, TypeCode: typeCode, Ext: asMap(e)}}
		}
	}
	arr, isArr := val.([]any)
	if !isArr {
		return []Value{{Key: k, TypeCode: typeCode, Value: val, Null: val == nil, Ext: asMap(ext)}}
	}
	extArr, _ := ext.([]any)
	// The items are those of both arrays, aligned by position (json.html#primitive): a "_key" item
	// past the values is a primitive with no value, as the HL7 validator reads it.
	n := max(len(arr), len(extArr))
	out := make([]Value, 0, n)
	for i := range n {
		v := Value{Key: k, TypeCode: typeCode, Array: true, Index: i}
		if i < len(arr) {
			v.Value, v.Null = arr[i], arr[i] == nil
		}
		if i < len(extArr) {
			v.Ext = asMap(extArr[i])
		}
		if v.Value == nil && v.Ext == nil && i >= len(arr) {
			continue // a null past the values pads nothing
		}
		out = append(out, v)
	}
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// Shape is how a value is written against its element's cardinality (json.html#arrays).
type Shape int

const (
	// RightShape is a value written as its element's cardinality says.
	RightShape Shape = iota
	// ArrayForOne is an item of an array written where one value goes: the HL7 validator reports
	// it ("must be an Object, not an Array") and validates nothing in it.
	ArrayForOne
	// SingleForMany is a single value written where an array goes: the HL7 validator reports it
	// ("must be a JSON Array") and validates it as the array's one item.
	SingleForMany
)

// ShapeOf tells how v, a value of child, is written: an array is written for an element whose
// base allows several values, whatever a profile narrows it to (json.html#arrays).
func ShapeOf(child *registry.ElementNode, v Value) Shape {
	switch repeats := child.Def.Repeats(); {
	case v.Array && !repeats:
		return ArrayForOne
	case !v.Array && repeats:
		return SingleForMany
	}
	return RightShape
}

// ItemPath is v's path under parent, the path of the instance that holds it: a single value
// written where an array goes is the array's one item ("contained[0]"), as HL7 names it.
func ItemPath(child *registry.ElementNode, v Value, parent string) string {
	if ShapeOf(child, v) == SingleForMany {
		return parent + "." + v.Key + "[0]"
	}
	return v.Path(parent)
}

// Written reports whether v, a value of child, is written as its element says (json.html): not an
// item of an array written where one value goes, an object for a complex type or a resource, a
// simple value for a primitive, a null only as an array's item of a primitive whose "_key" item
// holds its id and extensions (json.html#primitive); a primitive present only through its "_key"
// sibling is one. A value written otherwise is not one of the element's: the HL7 validator drops it
// (the structural phase reports it), so it is not counted nor evaluated. A value of an element
// whose kind is not known (several types, not a choice) is written unless it is a null.
func Written(child *registry.ElementNode, v Value, types Types) bool {
	if ShapeOf(child, v) == ArrayForOne {
		return false
	}
	known, prim := v.TypeCode != "" || child.Def.ContentReference != nil, primitive(types, v.TypeCode)
	switch v.Value.(type) {
	case nil:
		return prim && v.Ext != nil && (v.Array || !v.Null)
	case map[string]any:
		return !known || !prim
	case []any:
		return false
	}
	return !known || prim
}
