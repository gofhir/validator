// Package cardinality provides cardinality validation for FHIR resources.
//
// It walks the instance and the StructureDefinition's element tree together
// ([registry.ElementTree]): the children of an element come from the element itself, from the
// element it slices, from its contentReference, or from its type's definition. Slices are the
// slicing phase's; an instance is checked here against the unsliced element's definition.
package cardinality

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/walker"
)

// Validator performs cardinality validation of FHIR resources.
type Validator struct {
	registry *registry.Registry
	walker   *walker.Walker
}

// New creates a new cardinality Validator.
func New(reg *registry.Registry) *Validator {
	return &Validator{
		registry: reg,
		walker:   walker.New(reg),
	}
}

// Validate validates the cardinality of a FHIR resource against its StructureDefinition.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resource []byte, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

	// Parse JSON into a map
	var data map[string]any
	if err := json.Unmarshal(resource, &data); err != nil {
		result.AddErrorWithID(
			issue.DiagStructureInvalidJSON,
			map[string]any{"error": err.Error()},
		)
		return result
	}

	return v.ValidateData(data, sd)
}

// ValidateData validates the cardinality of a pre-parsed FHIR resource against its StructureDefinition.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(data map[string]any, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

	rootType := sd.Type
	if rootType == "" || sd.Snapshot == nil {
		return result
	}
	v.validateRoot(data, sd, rootType, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	// WalkWithProfiles validates against each declared profile in meta.profile.
	v.walker.WalkWithProfiles(data, rootType, rootType, func(ctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if ctx.FHIRPath == rootType {
			return true
		}
		if ctx.SD != nil && ctx.SD.Snapshot != nil {
			v.validateRoot(ctx.Data, ctx.SD, ctx.FHIRPath, result)
		}
		return true
	})

	return result
}

// validateRoot checks a resource (or a datatype value) against the root of sd's tree.
func (v *Validator) validateRoot(data map[string]any, sd *registry.StructureDefinition, fhirPath string, result *issue.Result) {
	root := sd.Tree().Root()
	if root == nil {
		return
	}
	v.validateNode(data, sd, root, "", fhirPath, result)
}

// validateNode checks the children of one instance of node: their counts against min and max, and
// recursively the children of those present.
func (v *Validator) validateNode(
	data map[string]any,
	sd *registry.StructureDefinition,
	node *registry.ElementNode,
	typeCode string,
	fhirPath string,
	result *issue.Result,
) {
	childSD, children := v.childrenOf(sd, node, typeCode)
	for _, child := range children {
		name := child.Name()
		childPath := fhirPath + "." + name
		values := childValues(child, data)
		count := len(values)

		if child.Def.Min > 0 && count < int(child.Def.Min) {
			result.AddErrorWithID(
				issue.DiagCardinalityMin,
				map[string]any{"path": childPath, "min": child.Def.Min, "count": count},
				childPath,
			)
		}
		if child.Def.Max != "" && child.Def.Max != "*" {
			if maxInt, err := strconv.Atoi(child.Def.Max); err == nil && count > maxInt {
				result.AddErrorWithID(
					issue.DiagCardinalityMax,
					map[string]any{"path": childPath, "max": maxInt, "count": count},
					childPath,
				)
			}
		}

		for i, cv := range values {
			m, ok := cv.value.(map[string]any)
			if !ok {
				continue
			}
			// A resource inside an element (Bundle.entry.resource, contained) is validated
			// against its own definition by the walker.
			if _, isResource := m[resourceTypeKey]; isResource {
				continue
			}
			p := fhirPath + "." + cv.key
			if cv.array {
				p = fmt.Sprintf("%s[%d]", p, i)
			}
			v.validateNode(m, childSD, child, cv.typeCode, p, result)
		}
	}
}

// resourceTypeKey is the FHIR JSON property that names a resource's type (json.html#resources).
const resourceTypeKey = "resourceType"

// childrenOf returns the child elements of an instance of node, and the StructureDefinition they
// belong to: node's own children in the snapshot; else those of the element it slices; else those
// its contentReference points to (D6); else those of its type's base definition. Following a
// type's profile instead is plan B (definition layering). The instance's type (typeCode) picks
// the type of a choice element.
func (v *Validator) childrenOf(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string) (*registry.StructureDefinition, []*registry.ElementNode) {
	if len(node.Children) > 0 {
		return sd, node.Children
	}
	for s := node.SliceOf; s != nil; s = s.SliceOf {
		if len(s.Children) > 0 {
			return sd, s.Children
		}
	}
	if ref := node.Def.ContentReference; ref != nil {
		target, _ := v.registry.ContentReference(sd, node)
		if target == nil {
			return nil, nil
		}
		tsd := sd
		if url, _, ok := registry.SplitContentReference(*ref); ok && url != "" && url != sd.URL {
			if s, _ := v.registry.ResolveCanonical(url); s != nil {
				tsd = s
			}
		}
		return v.childrenOf(tsd, target, typeCode)
	}

	code := typeCode
	if code == "" && len(node.Def.Type) == 1 {
		code = node.Def.Type[0].Code
	}
	if code == "" {
		return nil, nil
	}
	typeSD := v.registry.GetByType(code)
	if typeSD == nil || typeSD.Kind == kindPrimitive || typeSD == sd {
		return nil, nil
	}
	root := typeSD.Tree().Root()
	if root == nil {
		return nil, nil
	}
	return typeSD, root.Children
}

// kindPrimitive is the StructureDefinition.kind of primitive types, whose children (id,
// extension, value) are not properties of a JSON value.
const kindPrimitive = "primitive-type"

type childValue struct {
	key      string
	typeCode string
	value    any
	array    bool
}

// childValues returns the instance values of a child element: under its name, or for a choice
// element ("value[x]") under each name its types give it, the element name followed by the type
// code with its first letter capitalized (formats.html#choice). A primitive present only through
// its "_name" sibling (extensions without a value) is present (json.html#primitive).
func childValues(child *registry.ElementNode, data map[string]any) []childValue {
	name := child.Name()
	type key struct{ name, typeCode string }
	keys := []key{{name, ""}}
	if len(child.Def.Type) == 1 {
		keys[0].typeCode = child.Def.Type[0].Code
	}
	if base, ok := strings.CutSuffix(name, "[x]"); ok {
		keys = keys[:0]
		for _, t := range child.Def.Type {
			if t.Code != "" {
				keys = append(keys, key{base + strings.ToUpper(t.Code[:1]) + t.Code[1:], t.Code})
			}
		}
	}
	var out []childValue
	for _, k := range keys {
		val, ok := data[k.name]
		if !ok {
			if _, ext := data["_"+k.name]; ext {
				out = append(out, childValue{key: k.name, typeCode: k.typeCode})
			}
			continue
		}
		if arr, isArr := val.([]any); isArr {
			for _, item := range arr {
				out = append(out, childValue{key: k.name, typeCode: k.typeCode, value: item, array: true})
			}
			continue
		}
		out = append(out, childValue{key: k.name, typeCode: k.typeCode, value: val})
	}
	return out
}

// Extracts the element name from a path (e.g., "Patient.name" -> "name").
func getElementName(path string) string {
	lastDot := strings.LastIndex(path, ".")
	if lastDot == -1 {
		return ""
	}
	return path[lastDot+1:]
}
