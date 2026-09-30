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
	// Containment reports whether a resource is contained in another, for %rootResource.
	Containment func(container, resource map[string]any) bool
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
		run.scope = slicematch.Scope{Resource: resource, RootResource: resource, Container: resource}
	}

	if root := sd.Tree().Root(); root != nil {
		v.walk(run, run.scope, sd, root, resource, resourceType, result)
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

// childNamed returns node's child with this name, or nil.
func childNamed(node *registry.ElementNode, name string) *registry.ElementNode {
	if node == nil {
		return nil
	}
	for _, c := range governedChildren(node) {
		if c.Name() == name {
			return c
		}
	}
	return nil
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
	if psd == nil || v.registry.EnsureSnapshot(context.Background(), psd) != nil {
		return slice
	}
	if root := psd.Tree().Root(); root != nil {
		return root
	}
	return slice
}

// checkChildren checks the cardinality of node's children in one instance of node, and recurses
// into the children that are present.
//
// The cardinality phase already checks every instance against the unsliced element's definition
// (base), so a child is checked here only where the slice constrains it further: a missing child
// is reported once, not once per definition that requires it.
func (v *Validator) checkChildren(node, base *registry.ElementNode, inst map[string]any, instPath, defPath string, result *issue.Result) {
	for _, child := range node.Children {
		name := child.Name()
		childPath := instPath + "." + name
		childDef := defPath + "." + name
		values := v.childValues(child, inst)
		count := len(values)
		baseChild := childNamed(base, name)
		sameMin := baseChild != nil && baseChild.Def.Min == child.Def.Min
		sameMax := baseChild != nil && baseChild.Def.Max == child.Def.Max

		if !sameMin && count < int(child.Def.Min) {
			result.AddErrorWithID(issue.DiagSlicingCardinalityMin, map[string]any{
				"path": childDef, "min": child.Def.Min, "count": count,
			}, childPath)
		}
		if !sameMax && child.Def.Max != "" && child.Def.Max != "*" {
			if maxInt, err := strconv.Atoi(child.Def.Max); err == nil && count > maxInt {
				result.AddErrorWithID(issue.DiagSlicingCardinalityMax, map[string]any{
					"path": childDef, "max": maxInt, "count": count,
				}, childPath)
			}
		}

		if len(child.Children) == 0 {
			continue
		}
		for _, cv := range values {
			m, ok := cv.value.(map[string]any)
			if !ok {
				continue
			}
			p := instPath + "." + cv.key
			if cv.array {
				p = fmt.Sprintf("%s[%d]", p, cv.index)
			}
			v.checkChildren(child, baseChild, m, p, childDef, result)
		}
	}
}

type childValue struct {
	key   string
	value any
	array bool
	index int // position in the array, when array
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
		root := containedSD.Tree().Root()
		if root == nil {
			continue
		}
		containedFhirPath := fmt.Sprintf("%s.contained[%d]", baseFhirPath, i)
		scope := slicematch.Scope{Resource: resourceMap, RootResource: run.scope.RootResource, Container: run.scope.Container}
		v.walk(run, scope, containedSD, root, resourceMap, containedFhirPath, result)
	}
}
