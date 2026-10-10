package slicing

import (
	"context"
	"strconv"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
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
	childSD, children := v.walkChildren(run.ctx, sd, node, typeCode, inst)
	for _, child := range children {
		values := elementvalues.Of(child, inst, v.registry.ChoiceType)
		governing := make([]*registry.ElementNode, len(values))
		for i := range values {
			governing[i] = child
		}
		if child.Def.Slicing != nil && len(child.Slices) > 0 {
			governing = v.checkSlicing(run, scope, childSD, child, values, fhirPath, result)
		}
		for i, cv := range values {
			m, ok := cv.Value.(map[string]any)
			if !ok {
				m = cv.Ext
				// A primitive whose element the snapshot unrolls (a profile that slices its
				// extensions) is checked even with no "_key" sibling: its slices may be required.
				if m == nil && len(governedChildren(governing[i])) > 0 {
					m = map[string]any{}
				}
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
			v.walk(run, scope, nextSD, next, cv.TypeCode, m, cv.Path(fhirPath), result)
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

// walkChildren returns the children that govern an instance of node, inst, and the
// StructureDefinition they belong to (Registry.ChildrenOf, with the definition the instance
// declares for itself): their slicing applies to the value. A type's base definition has no
// slicing of its own, but the walk goes through it to the extensions it holds, a primitive's
// included, each checked against its definition. A profile that does not resolve is reported by
// the cardinality phase.
func (v *Validator) walkChildren(ctx context.Context, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, inst map[string]any) (*registry.StructureDefinition, []*registry.ElementNode) {
	c := v.registry.ChildrenOf(ctx, sd, node, typeCode, v.selfDefinitions(ctx, inst))
	return c.SD, c.Nodes
}

// selfDefinitions returns, for an instance, the definition it declares for itself for a type.
func (v *Validator) selfDefinitions(ctx context.Context, inst map[string]any) func(string) *registry.StructureDefinition {
	if v.definitions == nil {
		return nil
	}
	return func(typeCode string) *registry.StructureDefinition {
		return v.definitions.DefinitionOf(ctx, typeCode, inst)
	}
}

// resourceTypeKey is the FHIR JSON property that names a resource's type (json.html#resources).
const resourceTypeKey = "resourceType"

// bundleType is the type of the resource that holds others as its entries (bundle.html).
const bundleType = "Bundle"

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
		if slice := v.matchValue(run, scope, sd, node, cv, cv.Path(fhirPath), result); slice != nil {
			governing[i] = slice
			v.checkMember(run.ctx, sd, node, slice, cv, cv.Path(fhirPath), result)
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
		itemPath := values[i].Path(fhirPath)
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
				values[firstOutside].Path(fhirPath))
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

// checkMember checks the children of one value assigned to a slice, where the slice constrains
// them further than the definitions the cardinality phase checks the value against: the children
// that govern a value of the unsliced element (Registry.ChildrenOf), which the cardinality phase
// takes from the same place.
func (v *Validator) checkMember(ctx context.Context, sd *registry.StructureDefinition, node, slice *registry.ElementNode, cv childValue, itemPath string, result *issue.Result) {
	m, ok := cv.Value.(map[string]any)
	if !ok {
		return
	}
	base := v.registry.ChildrenOf(ctx, sd, node, cv.TypeCode, v.selfDefinitions(ctx, m))
	v.checkChildren(ctx, v.memberDefinition(slice), base, m, itemPath, node.Def.ID+":"+sliceNameOf(slice), result)
}

// matchValue returns the slice that governs one value, or nil, reporting what the matcher found on
// the way: several matching slices (D-1), a discriminator that could not be evaluated (D-2, D-3),
// and unknown ValueSet membership (D-6).
func (v *Validator) matchValue(run *validation, scope slicematch.Scope, sd *registry.StructureDefinition, node *registry.ElementNode, cv childValue, itemPath string, result *issue.Result) *registry.ElementNode {
	m := v.matcher.Resolve(run.ctx, slicematch.Request{
		SD: sd, Node: node, Key: cv.Key, Value: cv.Value, Valueless: cv.Value == nil && cv.Ext != nil, Scope: scope,
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

func sliceNameOf(n *registry.ElementNode) string {
	if n.Def.SliceName != nil {
		return *n.Def.SliceName
	}
	return ""
}
