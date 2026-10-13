package structural

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/gofhir/validator/v2/internal/exactjson"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// Normalized is a resource as the HL7 validator's parser leaves it (Normalize).
type Normalized struct {
	// Issues are the values written otherwise than their elements say, at their paths as written.
	Issues *issue.Result
	// Raw is the JSON of the resource as left, numbers as the source spells them: the source
	// itself when nothing was dropped.
	Raw []byte

	edits *edits
	// empty are the objects written empty, reported; hadContent the objects that, as written, held
	// one or a value that was dropped. Both by address, holding them.
	empty, hadContent objects
}

// objects is a set of objects by address, which holds them (an address is not reused while it
// does).
type objects map[uintptr]map[string]any

func (o objects) has(m map[string]any) bool {
	_, ok := o[reflect.ValueOf(m).Pointer()]
	return m != nil && ok
}

func (o *objects) add(m map[string]any) {
	if *o == nil {
		*o = objects{}
	}
	(*o)[reflect.ValueOf(m).Pointer()] = m
}

// Written is the path, as the source writes it, of a value at path in the resource as left: the
// items after a dropped one are renumbered, and the one item of an array made of a single value
// written where the array goes has no index in the source.
func (n *Normalized) Written(path string) string {
	if n == nil || n.edits == nil || len(n.edits.arrays) == 0 {
		return path
	}
	return n.edits.written(path)
}

// ContentJudged reports whether the content of m, an object of the resource as left, was judged on
// the source, as HL7's parser judges it, and is not to be judged again on m: an empty object
// (reported), or one that had content as written (HadContent). An element has a value or children
// (ele-1): the source says whether it has.
func (n *Normalized) ContentJudged(m map[string]any) bool {
	return n != nil && (n.empty.has(m) || n.hadContent.has(m))
}

// HadContent reports whether m, an object of the resource as left, held as written content that it
// does not hold for FHIRPath: an empty object, or a value written otherwise than its element says,
// dropped. Judged on its content as written, it has content.
func (n *Normalized) HadContent(m map[string]any) bool {
	return n != nil && n.hadContent.has(m)
}

// Normalize checks the shape of every value of data, a resource of definition sd parsed from raw,
// as the HL7 validator's parser does (json.html), and leaves data as that parser leaves it: a value
// written otherwise than its element says is reported, with its path as written, and dropped, an
// array's item with it (the items after it renumbered; a primitive's "_key" item with it); a single
// object written where an array goes becomes the array's one item. The shape depends on the base
// definitions alone, not on a profile: it is checked once. The phases that validate data afterwards
// see what the HL7 validator sees: no invariant, cardinality or binding reads a value it dropped.
func (v *Validator) Normalize(data map[string]any, raw []byte, sd *registry.StructureDefinition) *Normalized {
	n := &Normalized{Issues: issue.GetPooledResult(), Raw: raw}
	if sd == nil || sd.Type == "" {
		return n
	}
	idx := v.getOrBuildIndex(sd)
	ctx := &validationContext{rootSD: sd, rootIdx: idx, edits: &edits{}}
	all := issue.GetPooledResult()
	defer issue.ReleaseResult(all)
	v.validateElement(data, sd.Type, sd.Type, idx, ctx, all)
	n.empty, n.hadContent = ctx.edits.empties, ctx.edits.holders
	applied := len(ctx.edits.byElement) == 0
	if !applied {
		var left []byte
		if left, applied = ctx.edits.apply(data, raw); applied {
			n.Raw, n.edits = left, ctx.edits
			for _, x := range ctx.edits.byElement {
				if !x.wrapped {
					n.hadContent.add(x.parent)
				}
			}
		}
	}
	for _, is := range all.Issues {
		switch issue.DiagnosticID(is.MessageID) {
		case issue.DiagStructureEmptyObject:
			n.Issues.AddIssue(is)
		case issue.DiagStructureArrayExpected, issue.DiagStructureObjectExpected, issue.DiagStructureValueExpected:
			if applied { // else data is as written, and the structural phase reports them on it
				n.Issues.AddIssue(is)
			}
		}
	}
	return n
}

// edits are how a parser that drops misshapen values leaves an instance, by element: its parent
// object and key. A nil *edits records nothing.
type edits struct {
	byElement map[elementKey]*edit
	order     []elementKey
	// arrays are the arrays whose items are renumbered or made of a single value, by their path as
	// written: how a path in the instance as left is written (written)
	arrays map[string]*arrayEdit
	// empties are the objects written empty, and holders the objects that hold one
	empties, holders objects
}

type elementKey struct {
	parent uintptr
	key    string
}

// edit is what becomes of one element of parent: dropped, made an array of its one value, or some
// of its items dropped (by index).
type edit struct {
	parent  map[string]any
	key     string
	dropped bool
	wrapped bool
	drop    []int
}

// arrayEdit is how the items of an array as left are written: the indices dropped from it, or
// wrapped when the array is a single value written where the array goes.
type arrayEdit struct {
	dropped []int
	wrapped bool
}

func (e *edits) at(parent map[string]any, key string) *edit {
	k := elementKey{reflect.ValueOf(parent).Pointer(), key}
	if x, ok := e.byElement[k]; ok {
		return x
	}
	if e.byElement == nil {
		e.byElement = map[elementKey]*edit{}
	}
	x := &edit{parent: parent, key: key}
	e.byElement[k] = x
	e.order = append(e.order, k)
	return x
}

func (e *edits) array(path string) *arrayEdit {
	if e.arrays == nil {
		e.arrays = map[string]*arrayEdit{}
	}
	a, ok := e.arrays[path]
	if !ok {
		a = &arrayEdit{}
		e.arrays[path] = a
	}
	return a
}

// drop drops key from parent, and its "_key" sibling with it when it is a primitive's.
func (e *edits) drop(parent map[string]any, key string, primitive bool) {
	if e == nil {
		return
	}
	e.at(parent, key).dropped = true
	if _, ok := parent["_"+key]; primitive && ok {
		e.at(parent, "_"+key).dropped = true
	}
}

// dropItem drops item i of key's array, at path as written, and the "_key" item aligned with it when
// it is a primitive's. An item is dropped once, whichever of the two is misshapen.
func (e *edits) dropItem(parent map[string]any, key string, i int, primitive bool, path string) {
	if e == nil {
		return
	}
	x := e.at(parent, key)
	if slices.Contains(x.drop, i) {
		return
	}
	x.drop = append(x.drop, i)
	a := e.array(path)
	a.dropped = append(a.dropped, i)
	if siblings, ok := parent["_"+key].([]any); primitive && ok && i < len(siblings) {
		y := e.at(parent, "_"+key)
		y.drop = append(y.drop, i)
		s := e.array(path[:strings.LastIndex(path, ".")+1] + "_" + key)
		s.dropped = append(s.dropped, i)
	}
}

// empty records m, an empty object, and parent, the object that holds it, as judged on the source.
func (e *edits) empty(parent, m map[string]any) {
	if e == nil {
		return
	}
	e.empties.add(m)
	e.holders.add(parent)
}

// wrap makes key's single value, at path as written, the one item of an array.
func (e *edits) wrap(parent map[string]any, key, path string) {
	if e == nil {
		return
	}
	e.at(parent, key).wrapped = true
	e.array(path).wrapped = true
}

// apply makes the edits on data and on its exact twin decoded from raw, and returns the twin's
// JSON; data is left as it is when the twin cannot be read or written.
func (e *edits) apply(data map[string]any, raw []byte) ([]byte, bool) {
	twins := exactjson.New(data, raw)
	root := twins.Of(data) // pairs every object before any is changed
	if root == nil {
		return raw, false
	}
	type pair struct{ parsed, exact map[string]any }
	targets := make([]pair, 0, len(e.order))
	for _, k := range e.order {
		x := e.byElement[k]
		twin := twins.Of(x.parent)
		if twin == nil {
			return raw, false
		}
		targets = append(targets, pair{x.parent, twin})
	}
	for i, k := range e.order {
		x := e.byElement[k]
		x.on(targets[i].parsed)
		x.on(targets[i].exact)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if enc.Encode(root) != nil {
		return raw, false
	}
	return bytes.TrimRight(out.Bytes(), "\n"), true
}

// on makes the edit on parent, the parsed object or its exact twin. An array left with no item is
// dropped: an empty array is no value (json.html).
func (x *edit) on(parent map[string]any) {
	switch value := parent[x.key]; {
	case x.dropped:
		delete(parent, x.key)
	case x.wrapped:
		parent[x.key] = []any{value}
	default:
		items, ok := value.([]any)
		if !ok {
			return
		}
		items = slices.Clone(items)
		drop := slices.Compact(slices.Sorted(slices.Values(x.drop)))
		for _, i := range slices.Backward(drop) {
			if i < len(items) {
				items = slices.Delete(items, i, i+1)
			}
		}
		if len(items) == 0 {
			delete(parent, x.key)
			return
		}
		parent[x.key] = items
	}
}

// written is the path, as the source writes it, of path in the instance as left (Normalized.Written).
func (e *edits) written(path string) string {
	segments := strings.Split(path, ".")
	var out strings.Builder
	out.WriteString(segments[0])
	element := segments[0]
	for _, seg := range segments[1:] {
		name, index, indexed := splitIndex(seg)
		element += "." + name
		out.WriteString("." + name)
		if !indexed {
			continue
		}
		if a, ok := e.arrays[element]; ok {
			if a.wrapped && index == 0 {
				continue // a single value written where the array goes: no index as written
			}
			for _, d := range slices.Sorted(slices.Values(a.dropped)) {
				if d <= index {
					index++
				}
			}
		}
		item := fmt.Sprintf("[%d]", index)
		element += item
		out.WriteString(item)
	}
	return out.String()
}

// splitIndex splits a path segment "name[3]" into its name and index.
func splitIndex(seg string) (name string, index int, indexed bool) {
	open := strings.IndexByte(seg, '[')
	if open < 0 || !strings.HasSuffix(seg, "]") {
		return seg, 0, false
	}
	i, err := strconv.Atoi(seg[open+1 : len(seg)-1])
	if err != nil {
		return seg, 0, false
	}
	return seg[:open], i, true
}
