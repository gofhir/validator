package slicing

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/slicematch"
)

// Slicing rules (ElementDefinition.slicing.rules).
const (
	rulesClosed    = "closed"
	rulesOpenAtEnd = "openAtEnd"
)

// walk checks every slicing of node's children in one instance of node, and recurses into each
// child value under the definition that governs it: the slice it matched, or the element itself.
// Slicing is evaluated per parent instance, so the slices of one component never count another
// component's values, and a sliced element whose parent is absent is not checked at all.
//
// A primitive is walked through its "_key" sibling, which holds its id and extensions
// (json.html#primitive), so the slicing of a primitive's extensions is checked too.
//
// The instance's type, typeCode, picks the profile a choice element's type declares; it is ""
// for any other element.
func (v *Validator) walk(run *validation, scope slicematch.Scope, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, inst map[string]any, fhirPath string, result *issue.Result) {
	childSD, children := v.walkChildren(sd, node, typeCode, 0)
	for _, child := range children {
		values := v.childValues(child, inst)
		governing := make([]*registry.ElementNode, len(values))
		for i := range values {
			governing[i] = child
		}
		if child.Def.Slicing != nil && len(child.Slices) > 0 {
			governing = v.checkSlicing(run, scope, childSD, child, values, fhirPath, result)
		}
		for i, cv := range values {
			m, ok := cv.value.(map[string]any)
			if !ok {
				m = cv.ext
			}
			if m == nil {
				continue
			}
			// A resource inside an element (Bundle.entry.resource, contained) is validated against
			// its own definition, not the element's.
			if _, isResource := m[resourceTypeKey]; isResource {
				continue
			}
			nextSD, next := v.memberTree(childSD, governing[i])
			v.walk(run, scope, nextSD, next, v.valueType(child, cv.key), m, cv.path(fhirPath), result)
		}
	}
}

// memberTree returns where the walk continues under a value governed by node: the root of the one
// profile its type declares, when node is a slice whose snapshot unrolls no children (an extension
// slice is defined by its extension's StructureDefinition, sub-extension slices included), else
// node itself.
func (v *Validator) memberTree(sd *registry.StructureDefinition, node *registry.ElementNode) (*registry.StructureDefinition, *registry.ElementNode) {
	if node.SliceOf == nil {
		return sd, node
	}
	member := v.memberDefinition(node)
	if member == node {
		return sd, node
	}
	psd, _ := v.registry.ResolveCanonical(node.Def.Type[0].Profile[0])
	if psd == nil {
		return sd, node
	}
	return psd, member
}

// maxContentReferenceHops bounds a chain of contentReferences (a cycle in a malformed
// StructureDefinition).
const maxContentReferenceHops = 8

// walkChildren returns the children that govern an instance of node and the StructureDefinition
// they belong to: node's own in the snapshot; those of the element it slices, when the snapshot
// does not unroll the slice; those its contentReference points to; or those of the one profile its
// type declares (type.profile, plan B L1), whose slicing applies to the value. A type's base
// definition has no slicing of its own to check, and a profile that does not resolve is reported
// by the cardinality phase.
func (v *Validator) walkChildren(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, hops int) (*registry.StructureDefinition, []*registry.ElementNode) {
	if c := governedChildren(node); len(c) > 0 {
		return sd, c
	}
	ref := node.Def.ContentReference
	if ref == nil {
		if tp := v.registry.TypeProfile(context.Background(), node, typeCode); tp.SD != nil {
			return tp.SD, tp.SD.Tree().Root().Children
		}
		return sd, nil
	}
	if hops >= maxContentReferenceHops {
		return sd, nil
	}
	target, _ := v.registry.ContentReference(sd, node)
	if target == nil {
		return sd, nil
	}
	tsd := sd
	if url, _, ok := registry.SplitContentReference(*ref); ok && url != "" && url != sd.URL {
		if s, _ := v.registry.ResolveCanonical(url); s != nil {
			tsd = s
		}
	}
	return v.walkChildren(tsd, target, typeCode, hops+1)
}

// valueType is the type of a value of child read from the JSON property key: for a choice element
// ("value[x]"), the type the property names (valueQuantity: Quantity); "" otherwise.
func (v *Validator) valueType(child *registry.ElementNode, key string) string {
	base, ok := strings.CutSuffix(child.Name(), "[x]")
	if !ok {
		return ""
	}
	return v.registry.ChoiceType(base, key)
}

// resourceTypeKey is the FHIR JSON property that names a resource's type (json.html#resources).
const resourceTypeKey = "resourceType"

// governedChildren returns the children that govern an instance of node: its own in the snapshot,
// or those of the element it slices when the snapshot does not unroll the slice.
func governedChildren(node *registry.ElementNode) []*registry.ElementNode {
	if len(node.Children) > 0 {
		return node.Children
	}
	for s := node.SliceOf; s != nil; s = s.SliceOf {
		if len(s.Children) > 0 {
			return s.Children
		}
	}
	return nil
}

// checkSlicing assigns each value of a sliced element to its slice and checks the slicing rules
// in this parent instance, at every level: the element's slicing, and the slicing of each slice
// that is resliced, over the values assigned to it. It returns the definition governing each
// value.
func (v *Validator) checkSlicing(run *validation, scope slicematch.Scope, sd *registry.StructureDefinition, node *registry.ElementNode, values []childValue, fhirPath string, result *issue.Result) []*registry.ElementNode {
	governing := make([]*registry.ElementNode, len(values))
	for i, cv := range values {
		governing[i] = node
		if slice := v.matchValue(run, scope, sd, node, cv, cv.path(fhirPath), result); slice != nil {
			governing[i] = slice
			v.checkMember(node, slice, cv, cv.path(fhirPath), result)
		}
	}
	all := make([]int, len(values))
	for i := range values {
		all[i] = i
	}
	checkLevel(node, all, fhirPath+"."+node.Name(), values, governing, fhirPath, result)
	return governing
}

// checkLevel checks one level of slicing, level being the sliced element or a resliced slice, over
// the values assigned to it (members, by index): closed, ordered, openAtEnd, and the cardinality of
// each of its slices; then each resliced slice, over its own members.
func checkLevel(level *registry.ElementNode, members []int, elementPath string, values []childValue, governing []*registry.ElementNode, fhirPath string, result *issue.Result) {
	bySlice := map[*registry.ElementNode][]int{}
	for _, i := range members {
		if s := sliceAt(level, governing[i]); s != nil {
			bySlice[s] = append(bySlice[s], i)
		}
	}
	if level.Def.Slicing != nil {
		checkRules(level, members, values, governing, fhirPath, result)
	}
	for _, s := range level.Slices {
		checkSliceCount(s, len(bySlice[s]), elementPath, result)
		// A reslice's rules and cardinality apply within its slice (profiling.html#reslicing):
		// a slice with no members has none to constrain.
		if len(bySlice[s]) > 0 {
			checkLevel(s, bySlice[s], elementPath, values, governing, fhirPath, result)
		}
	}
}

// checkRules checks closed, ordered and openAtEnd over the members of one level, in order.
func checkRules(level *registry.ElementNode, members []int, values []childValue, governing []*registry.ElementNode, fhirPath string, result *issue.Result) {
	slicing := level.Def.Slicing
	position := map[*registry.ElementNode]int{}
	for i, s := range level.Slices {
		position[s] = i
	}
	// Each element is compared with the one before it, and one in no slice restarts the
	// comparison, as the HL7 validator orders slices.
	previous := -1
	firstOutside, reported := -1, false
	for _, i := range members {
		itemPath := values[i].path(fhirPath)
		s := sliceAt(level, governing[i])
		if s == nil {
			if slicing.Rules == rulesClosed {
				result.AddErrorWithID(issue.DiagSlicingNoMatch, nil, itemPath)
			}
			if firstOutside < 0 {
				firstOutside = i
			}
			previous = -1
			continue
		}
		if slicing.Ordered {
			if position[s] < previous {
				result.AddErrorWithID(issue.DiagSlicingOrder, map[string]any{"path": level.Def.ID}, itemPath)
			}
			previous = position[s]
		}
		if slicing.Rules == rulesOpenAtEnd && firstOutside >= 0 && !reported {
			result.AddErrorWithID(issue.DiagSlicingOpenAtEnd, map[string]any{"path": level.Def.ID},
				values[firstOutside].path(fhirPath))
			reported = true
		}
	}
}

// sliceAt returns the slice of level that a value governed by n belongs to: n itself or the
// slice of level it reslices, or nil when n is not under level.
func sliceAt(level, n *registry.ElementNode) *registry.ElementNode {
	for ; n != nil; n = n.SliceOf {
		if n.SliceOf == level {
			return n
		}
	}
	return nil
}

// checkSliceCount checks a slice's cardinality in one parent instance. It is reported at the
// sliced element of that instance with the slice's name (Patient.maritalStatus.coding:inset).
func checkSliceCount(s *registry.ElementNode, count int, elementPath string, result *issue.Result) {
	slicePath := elementPath + ":" + sliceNameOf(s)
	if count < int(s.Def.Min) {
		result.AddErrorWithID(issue.DiagSlicingCardinalityMin, map[string]any{
			"path": slicePath, "min": s.Def.Min, "count": count,
		}, slicePath)
	}
	if s.Def.Max != "" && s.Def.Max != "*" {
		if maxInt, err := strconv.Atoi(s.Def.Max); err == nil && count > maxInt {
			result.AddErrorWithID(issue.DiagSlicingCardinalityMax, map[string]any{
				"path": slicePath, "max": maxInt, "count": count,
			}, slicePath)
		}
	}
}

// checkMember checks the children of one value assigned to a slice. The unsliced element is the
// base the cardinality phase checks against; a slice defined by its type's profile has no such
// base here.
func (v *Validator) checkMember(node, slice *registry.ElementNode, cv childValue, itemPath string, result *issue.Result) {
	m, ok := cv.value.(map[string]any)
	if !ok {
		return
	}
	member := v.memberDefinition(slice)
	base := node
	if member != slice {
		base = nil
	}
	v.checkChildren(member, base, m, itemPath, node.Def.ID+":"+sliceNameOf(slice), result)
}

// matchValue returns the slice that governs one value, or nil, reporting what the matcher found on
// the way: several matching slices (D-1), a discriminator that could not be evaluated (D-2, D-3),
// and unknown ValueSet membership (D-6).
func (v *Validator) matchValue(run *validation, scope slicematch.Scope, sd *registry.StructureDefinition, node *registry.ElementNode, cv childValue, itemPath string, result *issue.Result) *registry.ElementNode {
	m := v.matcher.Resolve(run.ctx, slicematch.Request{
		SD: sd, Node: node, Key: cv.key, Value: cv.value, Scope: scope,
		Resolver: run.opts.Resolver, Containment: run.opts.Containment,
	})
	for _, n := range m.Notes {
		switch n.Kind {
		case slicematch.NoteMembershipUnknown:
			result.AddInfoWithID(issue.DiagSlicingMembershipUnknown, map[string]any{"detail": n.Message}, itemPath)
		default:
			result.AddErrorWithID(issue.DiagSlicingCannotEvaluate, map[string]any{"detail": n.Message}, itemPath)
		}
	}
	if !m.Matched {
		return nil
	}
	// One issue per further slice, naming the slice the value is assigned to and that one, as the
	// HL7 validator reports it.
	for _, a := range m.AlsoMatch {
		result.AddErrorWithID(issue.DiagSlicingMultipleMatch, map[string]any{
			"path": node.Def.ID, "slices": sliceNameOf(m.Node) + ", " + sliceNameOf(a),
		}, itemPath)
	}
	return m.Node
}

// path returns the location of a child value inside the instance at parent.
func (cv childValue) path(parent string) string {
	p := parent + "." + cv.key
	if cv.array {
		p = fmt.Sprintf("%s[%d]", p, cv.index)
	}
	return p
}

// childValues returns the instance values of a child element: under its name, or for a choice
// element ("value[x]") under each name its types give it, the element name followed by the type
// code with its first letter capitalized (formats.html#choice). A primitive present only through
// its "_name" sibling (extensions without a value) is present, once per entry of a repeating one
// (json.html#primitive).
func (v *Validator) childValues(child *registry.ElementNode, inst map[string]any) []childValue {
	return choiceValues(child, inst, v.registry.ChoiceType)
}

// choiceValues is childValues with the lookup that tells which properties name a type: a value of
// a type the element does not allow is present, with the wrong type, not absent.
func choiceValues(child *registry.ElementNode, inst map[string]any, choiceType func(base, key string) string) []childValue {
	keys := propertyNames(child, inst, choiceType)
	out := make([]childValue, 0, len(keys))
	for _, k := range keys {
		out = append(out, valuesOf(inst, k)...)
	}
	return out
}

// propertyNames returns the JSON properties an element can be read from: its name, or for a
// choice element the name of each type it allows followed by any other property in inst that
// names a type (formats.html#choice).
func propertyNames(child *registry.ElementNode, inst map[string]any, choiceType func(base, key string) string) []string {
	name := child.Name()
	base, ok := strings.CutSuffix(name, "[x]")
	if !ok {
		return []string{name}
	}
	var keys []string
	seen := map[string]bool{}
	for _, t := range child.Def.Type {
		if t.Code != "" {
			k := base + strings.ToUpper(t.Code[:1]) + t.Code[1:]
			keys = append(keys, k)
			seen[k] = true
		}
	}
	var others []string
	for k := range inst {
		if !seen[k] && choiceType(base, k) != "" {
			others = append(others, k)
		}
	}
	sort.Strings(others)
	return append(keys, others...)
}

// valuesOf returns the values of one property, each with its "_key" sibling (a primitive's id and
// extensions); a primitive present only through that sibling is present, once per entry of a
// repeating one (json.html#primitive).
func valuesOf(inst map[string]any, k string) []childValue {
	val, ok := inst[k]
	ext := inst["_"+k]
	if !ok {
		switch e := ext.(type) {
		case nil:
			return nil
		case []any:
			out := make([]childValue, 0, len(e))
			for i, item := range e {
				out = append(out, childValue{key: k, array: true, index: i, ext: asMap(item)})
			}
			return out
		default:
			return []childValue{{key: k, ext: asMap(e)}}
		}
	}
	arr, isArr := val.([]any)
	if !isArr {
		return []childValue{{key: k, value: val, ext: asMap(ext)}}
	}
	extArr, _ := ext.([]any)
	out := make([]childValue, 0, len(arr))
	for i, item := range arr {
		cv := childValue{key: k, value: item, array: true, index: i}
		if i < len(extArr) {
			cv.ext = asMap(extArr[i])
		}
		out = append(out, cv)
	}
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func sliceNameOf(n *registry.ElementNode) string {
	if n.Def.SliceName != nil {
		return *n.Def.SliceName
	}
	return ""
}
