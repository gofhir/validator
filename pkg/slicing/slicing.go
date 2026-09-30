// Package slicing validates FHIR slicing constraints from StructureDefinitions.
// It handles discriminator evaluation, slice matching, and cardinality validation.
package slicing

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/slicematch"
)

// Validator validates slicing constraints for FHIR resources.
type Validator struct {
	registry *registry.Registry
	matcher  *slicematch.Matcher
}

// New creates a new slicing validator. Its matcher cannot check profile conformance or ValueSet
// membership; use NewWithMatcher to provide one that can.
func New(reg *registry.Registry) *Validator {
	return NewWithMatcher(reg, slicematch.New(reg))
}

// NewWithMatcher creates a slicing validator that assigns elements to slices with m.
func NewWithMatcher(reg *registry.Registry, m *slicematch.Matcher) *Validator {
	return &Validator{registry: reg, matcher: m}
}

// Options carries what one validation provides to slice matching.
type Options struct {
	// Resolver follows references for the resolve() discriminator function.
	Resolver slicematch.Resolver
	// Scope holds the resources the validated value sits in, when it is not itself the resource.
	Scope *slicematch.Scope
}

// SliceInfo contains information about a defined slice.
type SliceInfo struct {
	Name       string                        // sliceName
	Definition *registry.ElementDefinition   // The slice's ElementDefinition
	Children   []*registry.ElementDefinition // Child ElementDefinitions of this slice
	Min        uint32                        // Minimum cardinality for this slice
	Max        string                        // Maximum cardinality ("*" = unbounded)
}

// Context contains slicing information for an element path.
type Context struct {
	sd             *registry.StructureDefinition
	Path           string                      // The sliced element path (e.g., "Patient.extension")
	EntryDef       *registry.ElementDefinition // ElementDefinition with slicing definition
	Discriminators []registry.Discriminator    // How to match elements to slices
	Rules          string                      // open | closed | openAtEnd
	Ordered        bool                        // Whether slice order matters
	Slices         []SliceInfo                 // Defined slices
}

// Validate validates slicing constraints for a FHIR resource.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateData(resource, sd, result)
}

// ValidateData validates slicing constraints for a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	v.ValidateDataContext(context.Background(), resource, sd, Options{}, result)
}

// ValidateDataContext validates slicing constraints of a pre-parsed value against sd: a resource,
// or a datatype or extension value validated against its own profile.
func (v *Validator) ValidateDataContext(goCtx context.Context, resource map[string]any, sd *registry.StructureDefinition, opts Options, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType := sd.RootName(resource)
	if resourceType == "" {
		return
	}

	run := &validation{ctx: goCtx, opts: opts}
	if opts.Scope != nil {
		run.scope = *opts.Scope
	} else {
		run.scope = slicematch.Scope{Resource: resource, RootResource: resource}
	}

	// Extract all slicing contexts from the StructureDefinition
	contexts := v.extractContexts(sd)

	// Validate each slicing context against the resource
	for _, ctx := range contexts {
		v.validateContext(run, run.scope, resource, resourceType, resourceType, ctx, result)
	}

	// Also validate contained resources
	v.validateContained(run, resource, resourceType, result)
}

// validation is one ValidateDataContext call.
type validation struct {
	ctx   context.Context
	opts  Options
	scope slicematch.Scope
}

// extractContexts extracts all slicing definitions from a StructureDefinition.
func (v *Validator) extractContexts(sd *registry.StructureDefinition) []Context {
	contexts := make([]Context, 0, 8)

	// Map to group elements by their sliced parent path
	slicesByPath := make(map[string][]SliceInfo)
	entryByPath := make(map[string]*registry.ElementDefinition)

	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]

		// Check if this element defines slicing
		if elem.Slicing != nil {
			entryByPath[elem.Path] = elem
		}

		// Check if this element is a slice (has sliceName)
		if elem.SliceName != nil && *elem.SliceName != "" {
			sliceName := *elem.SliceName
			// Find children of this slice
			children := v.findSliceChildren(sd, elem.ID)

			sliceInfo := SliceInfo{
				Name:       sliceName,
				Definition: elem,
				Children:   children,
				Min:        elem.Min,
				Max:        elem.Max,
			}
			slicesByPath[elem.Path] = append(slicesByPath[elem.Path], sliceInfo)
		}
	}

	// Build Contexts from entries and their slices
	for path, entry := range entryByPath {
		ctx := Context{
			sd:       sd,
			Path:     path,
			EntryDef: entry,
			Rules:    entry.Slicing.Rules,
			Slices:   slicesByPath[path],
		}

		if entry.Slicing.Discriminator != nil {
			ctx.Discriminators = entry.Slicing.Discriminator
		}

		contexts = append(contexts, ctx)
	}

	return contexts
}

// findSliceChildren finds ElementDefinitions that are children of a slice.
func (v *Validator) findSliceChildren(sd *registry.StructureDefinition, sliceID string) []*registry.ElementDefinition {
	var children []*registry.ElementDefinition

	prefix := sliceID + "."
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]
		if strings.HasPrefix(elem.ID, prefix) {
			children = append(children, elem)
		}
	}

	return children
}

// validateContext validates a single slicing context against resource data.
func (v *Validator) validateContext(
	run *validation,
	scope slicematch.Scope,
	resource map[string]any,
	sdPath string,
	fhirPath string,
	ctx Context,
	result *issue.Result,
) {
	// Navigate to the sliced element in the resource
	elements := v.getElementsAtPath(resource, ctx.Path, sdPath)
	if elements == nil {
		return // Element not present, cardinality validator handles this
	}

	// Track which slice each element matches
	sliceMatches := make(map[int]string) // element index -> slice name
	sliceCounts := make(map[string]int)  // slice name -> count

	// Match each element to a slice
	for i, elem := range elements {
		elemMap, ok := elem.(map[string]any)
		if !ok {
			continue
		}

		elemPath := fmt.Sprintf("%s.%s[%d]", fhirPath, v.lastPathSegment(ctx.Path), i)
		matchedSlice := v.matchElement(run, scope, ctx, elemMap, elemPath, result)
		if matchedSlice != "" {
			sliceMatches[i] = matchedSlice
			sliceCounts[matchedSlice]++
		} else if ctx.Rules == rulesClosed {
			// Element doesn't match any slice in closed slicing
			result.AddErrorWithID(issue.DiagSlicingNoMatch, nil, elemPath)
		}
	}

	// Validate cardinality for each slice
	for _, slice := range ctx.Slices {
		count := sliceCounts[slice.Name]
		slicePath := fmt.Sprintf("%s.%s:%s", fhirPath, v.lastPathSegment(ctx.Path), slice.Name)

		// Check minimum (safe comparison avoiding overflow)
		if count < 0 || count < int(slice.Min) {
			result.AddErrorWithID(issue.DiagSlicingCardinalityMin, map[string]any{
				"path": slicePath, "min": slice.Min, "count": count,
			}, slicePath)
		}

		// Check maximum
		if slice.Max != "*" {
			maxInt, err := strconv.Atoi(slice.Max)
			if err == nil && count > maxInt {
				result.AddErrorWithID(issue.DiagSlicingCardinalityMax, map[string]any{
					"path": slicePath, "max": maxInt, "count": count,
				}, slicePath)
			}
		}
	}

	// Validate cardinality of child elements within matched slices
	v.validateSliceChildren(elements, sliceMatches, ctx, fhirPath, result)
}

// rulesClosed is the slicing rule that allows no content outside the slices (slicing.rules).
const rulesClosed = "closed"

// matchElement returns the name of the slice that governs one instance, reporting what the
// matcher found on the way: an instance matching several slices (D-1), a discriminator that could
// not be evaluated (D-2, D-3), and unknown ValueSet membership (D-6).
func (v *Validator) matchElement(run *validation, scope slicematch.Scope, ctx Context, elem map[string]any, elemPath string, result *issue.Result) string {
	if ctx.sd == nil || ctx.EntryDef == nil {
		return ""
	}
	node := ctx.sd.Tree().ByID(ctx.EntryDef.ID)
	if node == nil {
		return ""
	}
	m := v.matcher.Resolve(run.ctx, slicematch.Request{
		SD: ctx.sd, Node: node, Key: node.Name(), Value: elem, Scope: scope, Resolver: run.opts.Resolver,
	})
	for _, n := range m.Notes {
		switch n.Kind {
		case slicematch.NoteMembershipUnknown:
			result.AddInfoWithID(issue.DiagSlicingMembershipUnknown, map[string]any{"detail": n.Message}, elemPath)
		default:
			result.AddErrorWithID(issue.DiagSlicingCannotEvaluate, map[string]any{"detail": n.Message}, elemPath)
		}
	}
	if !m.Matched {
		return ""
	}
	// One issue per further slice, naming the slice the element is assigned to and that one, as
	// the HL7 validator reports it.
	for _, a := range m.AlsoMatch {
		result.AddErrorWithID(issue.DiagSlicingMultipleMatch, map[string]any{
			"path": ctx.EntryDef.ID, "slices": sliceNameOf(m.Node) + ", " + sliceNameOf(a),
		}, elemPath)
	}
	return sliceNameOf(m.Node)
}

func sliceNameOf(n *registry.ElementNode) string {
	if n.Def.SliceName != nil {
		return *n.Def.SliceName
	}
	return ""
}

// validateSliceChildren validates the cardinality of the elements inside each matched slice
// instance, from the slice's definition in the element tree. A child is checked only where its
// parent is present in the instance: the children of an optional element that is absent are not
// required (D1). A choice element is counted under each of its JSON names, the element name
// followed by a type code with its first letter capitalized (D1b).
func (v *Validator) validateSliceChildren(
	elements []any,
	sliceMatches map[int]string,
	ctx Context,
	fhirPath string,
	result *issue.Result,
) {
	if ctx.sd == nil {
		return
	}
	tree := ctx.sd.Tree()
	sliceByName := make(map[string]*registry.ElementNode, len(ctx.Slices))
	for i := range ctx.Slices {
		if def := ctx.Slices[i].Definition; def != nil {
			if n := tree.ByID(def.ID); n != nil {
				sliceByName[ctx.Slices[i].Name] = n
			}
		}
	}

	pathSegment := v.lastPathSegment(ctx.Path)
	for elemIdx, sliceName := range sliceMatches {
		node := sliceByName[sliceName]
		elemMap, ok := elements[elemIdx].(map[string]any)
		if node == nil || !ok {
			continue
		}
		elemPath := fmt.Sprintf("%s.%s[%d]", fhirPath, pathSegment, elemIdx)
		checkChildren(v.memberDefinition(node), elemMap, elemPath, ctx.Path+":"+sliceName, result)
	}
}

// memberDefinition returns the definition whose children govern an instance of a slice: the slice
// itself when the snapshot unrolls its children, else the root of the one profile its type
// declares (an extension slice is defined by its extension's StructureDefinition). A type with no
// profile, or several, leaves the slice's own (empty) children.
func (v *Validator) memberDefinition(slice *registry.ElementNode) *registry.ElementNode {
	if len(slice.Children) > 0 || len(slice.Def.Type) != 1 || len(slice.Def.Type[0].Profile) != 1 {
		return slice
	}
	psd, _ := v.registry.ResolveCanonical(slice.Def.Type[0].Profile[0])
	if psd == nil || psd.Snapshot == nil {
		return slice
	}
	if root := psd.Tree().Root(); root != nil {
		return root
	}
	return slice
}

// checkChildren checks the cardinality of node's children in one instance of node, and recurses
// into the children that are present.
func checkChildren(node *registry.ElementNode, inst map[string]any, instPath, defPath string, result *issue.Result) {
	for _, child := range node.Children {
		name := child.Name()
		childPath := instPath + "." + name
		childDef := defPath + "." + name
		values := childValues(child, inst)
		count := len(values)

		if count < int(child.Def.Min) {
			result.AddErrorWithID(issue.DiagSlicingCardinalityMin, map[string]any{
				"path": childDef, "min": child.Def.Min, "count": count,
			}, childPath)
		}
		if child.Def.Max != "" && child.Def.Max != "*" {
			if maxInt, err := strconv.Atoi(child.Def.Max); err == nil && count > maxInt {
				result.AddErrorWithID(issue.DiagSlicingCardinalityMax, map[string]any{
					"path": childDef, "max": maxInt, "count": count,
				}, childPath)
			}
		}

		if len(child.Children) == 0 {
			continue
		}
		for i, v := range values {
			m, ok := v.value.(map[string]any)
			if !ok {
				continue
			}
			p := instPath + "." + v.key
			if v.array {
				p = fmt.Sprintf("%s[%d]", p, i)
			}
			checkChildren(child, m, p, childDef, result)
		}
	}
}

type childValue struct {
	key   string
	value any
	array bool
}

// childValues returns the instance values of a child element: under its name, or for a choice
// element ("value[x]") under each name its types give it. A primitive present only through its
// "_name" sibling (extensions without a value) is present (json.html#primitive).
func childValues(child *registry.ElementNode, inst map[string]any) []childValue {
	name := child.Name()
	keys := []string{name}
	if base, ok := strings.CutSuffix(name, "[x]"); ok {
		keys = keys[:0]
		for _, t := range child.Def.Type {
			if t.Code != "" {
				keys = append(keys, base+strings.ToUpper(t.Code[:1])+t.Code[1:])
			}
		}
	}
	var out []childValue
	for _, k := range keys {
		v, ok := inst[k]
		if !ok {
			if _, ext := inst["_"+k]; ext {
				out = append(out, childValue{key: k})
			}
			continue
		}
		if arr, isArr := v.([]any); isArr {
			for _, item := range arr {
				out = append(out, childValue{key: k, value: item, array: true})
			}
			continue
		}
		out = append(out, childValue{key: k, value: v})
	}
	return out
}

// getElementsAtPath extracts elements at a given SD path from the resource.
func (v *Validator) getElementsAtPath(resource map[string]any, sdPath, resourceType string) []any {
	// Remove resourceType prefix from path
	relativePath := strings.TrimPrefix(sdPath, resourceType+".")

	parts := strings.Split(relativePath, ".")
	current := any(resource)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case []any:
			// Flatten array elements and continue
			var results []any
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					if val := m[part]; val != nil {
						results = append(results, val)
					}
				}
			}
			current = results
		default:
			return nil
		}
	}

	// Ensure we return a slice
	if arr, ok := current.([]any); ok {
		return arr
	}
	if current != nil {
		return []any{current}
	}
	return nil
}

// lastPathSegment returns the last segment of a path.
func (v *Validator) lastPathSegment(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// validateContained validates slicing in contained resources.
func (v *Validator) validateContained(run *validation, resource map[string]any, baseFhirPath string, result *issue.Result) {
	containedRaw, ok := resource["contained"]
	if !ok {
		return
	}

	contained, ok := containedRaw.([]any)
	if !ok {
		return
	}

	for i, item := range contained {
		resourceMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		if resourceType == "" {
			continue
		}

		containedSD := v.registry.GetByType(resourceType)
		if containedSD == nil || containedSD.Snapshot == nil {
			continue
		}

		containedFhirPath := fmt.Sprintf("%s.contained[%d]", baseFhirPath, i)

		// Extract and validate slicing contexts for contained resource
		contexts := v.extractContexts(containedSD)
		scope := slicematch.Scope{Resource: resourceMap, RootResource: run.scope.RootResource}
		for _, ctx := range contexts {
			v.validateContext(run, scope, resourceMap, resourceType, containedFhirPath, ctx, result)
		}
	}
}
