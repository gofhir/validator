package slicing

import (
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
func (v *Validator) walk(run *validation, scope slicematch.Scope, sd *registry.StructureDefinition, node *registry.ElementNode, inst map[string]any, fhirPath string, result *issue.Result) {
	for _, child := range governedChildren(node) {
		values := v.childValues(child, inst)
		governing := make([]*registry.ElementNode, len(values))
		for i := range values {
			governing[i] = child
		}
		if child.Def.Slicing != nil && len(child.Slices) > 0 {
			governing = v.checkSlicing(run, scope, sd, child, values, fhirPath, result)
		}
		for i, cv := range values {
			m, ok := cv.value.(map[string]any)
			if !ok {
				continue
			}
			// A resource inside an element (Bundle.entry.resource, contained) is validated against
			// its own definition, not the element's.
			if _, isResource := m[resourceTypeKey]; isResource {
				continue
			}
			v.walk(run, scope, sd, governing[i], m, cv.path(fhirPath), result)
		}
	}
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
// in this parent instance: closed, ordered, openAtEnd, and each slice's (and reslice's)
// cardinality. It returns the definition governing each value.
func (v *Validator) checkSlicing(run *validation, scope slicematch.Scope, sd *registry.StructureDefinition, node *registry.ElementNode, values []childValue, fhirPath string, result *issue.Result) []*registry.ElementNode {
	governing := make([]*registry.ElementNode, len(values))
	counts := map[*registry.ElementNode]int{}
	slicing := node.Def.Slicing
	order := map[*registry.ElementNode]int{}
	for i, s := range node.Slices {
		order[s] = i
	}
	lastOrder := -1
	firstOutside := -1 // the first value in no slice, for openAtEnd

	for i, cv := range values {
		governing[i] = node
		itemPath := cv.path(fhirPath)
		slice := v.matchValue(run, scope, sd, node, cv, itemPath, result)
		if slice == nil {
			if slicing.Rules == rulesClosed {
				result.AddErrorWithID(issue.DiagSlicingNoMatch, nil, itemPath)
			}
			if firstOutside < 0 {
				firstOutside = i
			}
			continue
		}
		governing[i] = slice
		top := slice
		for s := slice; s != nil && s != node; s = s.SliceOf {
			counts[s]++
			top = s
		}
		if slicing.Ordered {
			if o := order[top]; o < lastOrder {
				result.AddErrorWithID(issue.DiagSlicingOrder, map[string]any{"path": node.Def.ID}, itemPath)
			} else {
				lastOrder = o
			}
		}
		if slicing.Rules == rulesOpenAtEnd && firstOutside >= 0 {
			result.AddErrorWithID(issue.DiagSlicingOpenAtEnd, map[string]any{"path": node.Def.ID},
				values[firstOutside].path(fhirPath))
			firstOutside = len(values) // report once
		}
		v.checkMember(node, slice, cv, itemPath, result)
	}

	checkSliceCounts(node, fhirPath+"."+node.Name(), counts, result)
	return governing
}

// checkSliceCounts checks each slice's (and reslice's) cardinality against the values assigned to
// it in one parent instance.
func checkSliceCounts(n *registry.ElementNode, elementPath string, counts map[*registry.ElementNode]int, result *issue.Result) {
	for _, s := range n.Slices {
		slicePath := elementPath + ":" + sliceNameOf(s)
		count := counts[s]
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
		checkSliceCounts(s, elementPath, counts, result)
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
	name := child.Name()
	keys := []string{name}
	if base, ok := strings.CutSuffix(name, "[x]"); ok {
		keys = keys[:0]
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
		keys = append(keys, others...)
	}
	var out []childValue
	for _, k := range keys {
		val, ok := inst[k]
		if !ok {
			switch ext := inst["_"+k].(type) {
			case nil:
			case []any:
				for i := range ext {
					out = append(out, childValue{key: k, array: true, index: i})
				}
			default:
				out = append(out, childValue{key: k})
			}
			continue
		}
		if arr, isArr := val.([]any); isArr {
			for i, item := range arr {
				out = append(out, childValue{key: k, value: item, array: true, index: i})
			}
			continue
		}
		out = append(out, childValue{key: k, value: val})
	}
	return out
}

func sliceNameOf(n *registry.ElementNode) string {
	if n.Def.SliceName != nil {
		return *n.Def.SliceName
	}
	return ""
}
