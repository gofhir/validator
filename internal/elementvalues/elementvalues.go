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
	Value    any            // nil for a primitive present only through its "_key" sibling
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

// Types tells what the values' types are: the type a choice property names (Registry.ChoiceType)
// and whether a type is primitive (Registry.IsPrimitiveTypeCode). *registry.Registry is one.
type Types interface {
	ChoiceType(base, key string) string
	IsPrimitiveTypeCode(code string) bool
}

// Of returns the values inst holds for child: under its name, or for a choice element
// ("value[x]") under each name its types give it, the element name followed by the type code with
// its first letter capitalized (formats.html#choice), then under any other property that
// types.ChoiceType says names a type. A value of a type the element does not allow is present,
// with the wrong type, not absent. Only a primitive has a "_name" sibling (json.html#primitive): a
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
	props := choiceProperties(base, child.Def.Type, inst, types.ChoiceType)
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

// choiceProperties returns the properties of inst a choice element of base and types is read from:
// those the types name (or their "_key" siblings name), in the types' order, then those choiceType
// says name another type, by key. Only inst's own keys are looked at, not every name the types
// give.
func choiceProperties(base string, types []registry.Type, inst map[string]any, choiceType func(base, key string) string) []property {
	var out []property
	for k := range inst {
		key, underscored := strings.CutPrefix(k, "_")
		if len(key) <= len(base) || !strings.HasPrefix(key, base) {
			continue
		}
		if underscored {
			if _, hasValue := inst[key]; hasValue {
				continue // the value's key stands for both
			}
		}
		if rank := declared(types, key[len(base):]); rank >= 0 {
			out = append(out, property{key: key, code: types[rank].Code, rank: rank})
		} else if !underscored {
			if code := choiceType(base, key); code != "" {
				out = append(out, property{key: key, code: code, rank: len(types)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].rank != out[j].rank {
			return out[i].rank < out[j].rank
		}
		return out[i].key < out[j].key
	})
	return out
}

// declared returns the position among types of the one a choice property's suffix names, the type
// code with its first letter capitalized (formats.html#choice), or -1.
func declared(types []registry.Type, suffix string) int {
	for i, t := range types {
		c := t.Code
		if c == "" || len(c) != len(suffix) || c[1:] != suffix[1:] {
			continue
		}
		first := c[0]
		if first >= 'a' && first <= 'z' {
			first -= 'a' - 'A'
		}
		if first == suffix[0] {
			return i
		}
	}
	return -1
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
		return []Value{{Key: k, TypeCode: typeCode, Value: val, Ext: asMap(ext)}}
	}
	extArr, _ := ext.([]any)
	// The items are those of both arrays, aligned by position (json.html#primitive): a "_key" item
	// past the values is a primitive with no value, as the HL7 validator reads it.
	n := max(len(arr), len(extArr))
	out := make([]Value, 0, n)
	for i := range n {
		v := Value{Key: k, TypeCode: typeCode, Array: true, Index: i}
		if i < len(arr) {
			v.Value = arr[i]
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
	most := child.Def.Max
	if child.Def.Base != nil && child.Def.Base.Max != "" {
		most = child.Def.Base.Max
	}
	switch repeats := most != "1" && most != "0"; {
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
