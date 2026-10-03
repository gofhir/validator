// Package elementvalues reads the values an instance holds for an element of a StructureDefinition
// tree, the same way for every phase that walks the tree, so that each names a value by the same
// path.
package elementvalues

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gofhir/validator/pkg/registry"
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

// Of returns the values inst holds for child: under its name, or for a choice element
// ("value[x]") under each name its types give it, the element name followed by the type code with
// its first letter capitalized (formats.html#choice), then under any other property that
// choiceType says names a type. A value of a type the element does not allow is present, with the
// wrong type, not absent. A primitive present only through its "_name" sibling (extensions without
// a value) is present, once per entry of a repeating one (json.html#primitive).
func Of(child *registry.ElementNode, inst map[string]any, choiceType func(base, key string) string) []Value {
	props := properties(child, inst, choiceType)
	out := make([]Value, 0, len(props))
	for _, k := range props {
		out = append(out, valuesOf(inst, k[0], k[1])...)
	}
	return out
}

// properties returns the JSON properties an element can be read from, each with the type it
// names.
func properties(child *registry.ElementNode, inst map[string]any, choiceType func(base, key string) string) [][2]string {
	name := child.Name()
	base, ok := strings.CutSuffix(name, "[x]")
	if !ok {
		typeCode := ""
		if len(child.Def.Type) == 1 {
			typeCode = child.Def.Type[0].Code
		}
		return [][2]string{{name, typeCode}}
	}
	var out [][2]string
	seen := map[string]bool{}
	for _, t := range child.Def.Type {
		if t.Code != "" {
			k := base + strings.ToUpper(t.Code[:1]) + t.Code[1:]
			out = append(out, [2]string{k, t.Code})
			seen[k] = true
		}
	}
	var others [][2]string
	for k := range inst {
		if !seen[k] {
			if code := choiceType(base, k); code != "" {
				others = append(others, [2]string{k, code})
			}
		}
	}
	sort.Slice(others, func(i, j int) bool { return others[i][0] < others[j][0] })
	return append(out, others...)
}

// valuesOf returns the values of one property, each with its "_key" sibling.
func valuesOf(inst map[string]any, k, typeCode string) []Value {
	val, ok := inst[k]
	ext := inst["_"+k]
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
	out := make([]Value, 0, len(arr))
	for i, item := range arr {
		v := Value{Key: k, TypeCode: typeCode, Value: item, Array: true, Index: i}
		if i < len(extArr) {
			v.Ext = asMap(extArr[i])
		}
		out = append(out, v)
	}
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
