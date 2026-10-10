// Package cardinality provides cardinality validation for FHIR resources.
//
// It walks the instance and the StructureDefinition's element tree together
// ([registry.ElementTree]): the children of an element come from the element itself, from the
// element it slices, from its contentReference, or from its type's definition. Slices are the
// slicing phase's; an instance is checked here against the unsliced element's definition.
package cardinality

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/walker"
)

// Validator performs cardinality validation of FHIR resources.
type Validator struct {
	registry    *registry.Registry
	walker      *walker.Walker
	definitions DefinitionSource
	shapes      sync.Map // *registry.StructureDefinition -> primitiveShape
}

// DefinitionSource names the definition a value declares for itself, which governs the value
// wherever it is used: an extension's url names the definition the extension conforms to
// (extensibility.html). It returns nil for a value that declares none. The definition has a
// snapshot.
type DefinitionSource interface {
	DefinitionOf(ctx context.Context, typeCode string, value map[string]any) *registry.StructureDefinition
}

// Option configures a Validator.
type Option func(*Validator)

// WithDefinitions sets the source of the definitions values declare for themselves. Without it, a
// value is checked against the definitions of the element that holds it only.
func WithDefinitions(d DefinitionSource) Option { return func(v *Validator) { v.definitions = d } }

// New creates a new cardinality Validator.
func New(reg *registry.Registry, opts ...Option) *Validator {
	v := &Validator{
		registry: reg,
		walker:   walker.New(reg),
	}
	for _, o := range opts {
		o(v)
	}
	return v
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
	return v.ValidateDataContext(context.Background(), data, sd)
}

// ValidateDataContext is ValidateData, resolving the profiles nested resources declare with ctx.
func (v *Validator) ValidateDataContext(ctx context.Context, data map[string]any, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

	rootType := sd.Type
	if rootType == "" || sd.Snapshot == nil {
		return result
	}
	v.validateRoot(data, sd, rootType, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	// WalkWithProfiles validates against each declared profile in meta.profile.
	v.walker.WalkWithProfilesContext(ctx, data, rootType, rootType, func(ctx *walker.ResourceContext) bool {
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
	childSD, children, missing := v.childrenOf(sd, node, typeCode, data)
	if missing.canonical != "" {
		// The value's type declares a profile that cannot be resolved: it cannot be checked, and is
		// not checked against the base type instead (as in the HL7 validator, which reports the
		// profile as unknown). Reported here only, not again by the slicing phase.
		result.AddErrorWithID(issue.DiagTypeProfileNotFound,
			map[string]any{"profile": missing.canonical, "type": missing.typeCode, "reason": missing.reason},
			fhirPath)
		return
	}
	v.validateChildren(data, childSD, children, fhirPath, result)
}

// validateChildren checks children in one instance, data, held at fhirPath: their counts against
// min and max, and recursively the children of those present.
func (v *Validator) validateChildren(data map[string]any, childSD *registry.StructureDefinition, children []*registry.ElementNode, fhirPath string, result *issue.Result) {
	for _, child := range children {
		name := child.Name()
		childPath := fhirPath + "." + name
		values := elementvalues.Of(child, data, v.registry.ChoiceType)
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

		for _, cv := range values {
			m, ok := cv.Value.(map[string]any)
			if !ok {
				v.validatePrimitiveElement(cv.Value, cv.Ext, childSD, child, cv.TypeCode, cv.Path(fhirPath), result)
				continue
			}
			// A resource held in the element (Bundle.entry.resource, contained) is validated
			// against its own definition by the walker.
			if v.registry.HoldsResource(child.Def) {
				continue
			}
			v.validateNode(m, childSD, child, cv.TypeCode, cv.Path(fhirPath), result)
		}
	}
}

// validatePrimitiveElement checks one primitive value against the children of its element: those
// the snapshot unrolls for it (a profile that requires an extension on it), else those of the
// primitive's type definition. In JSON a primitive's value is its own property, and its id and
// extensions are in its "_key" sibling, ext (json.html#primitive): the value is checked as the child
// the primitive type defines for it, the others against the sibling, even when there is none.
func (v *Validator) validatePrimitiveElement(value any, ext map[string]any, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode, fhirPath string, result *issue.Result) {
	code := typeCode
	if code == "" && len(node.Def.Type) == 1 {
		code = node.Def.Type[0].Code
	}
	typeSD := v.registry.GetByType(code)
	if typeSD == nil || typeSD.Kind != kindPrimitive {
		return
	}
	typeRoot := typeSD.Tree().Root()
	if typeRoot == nil {
		return
	}
	childSD, children := sd, node.Children
	if len(children) == 0 {
		if ext == nil && v.primitiveShape(typeSD).holdsAnyValue {
			return // the type's definition requires nothing of a value with no sibling
		}
		childSD, children = typeSD, typeRoot.Children
	}
	inst := make(map[string]any, len(ext)+1)
	for k, x := range ext {
		inst[k] = x
	}
	if value != nil {
		for _, name := range v.primitiveShape(typeSD).valueNames {
			inst[name] = value
		}
	}
	v.validateChildren(inst, childSD, children, fhirPath, result)
}

// primitiveShape is what a primitive type's definition says about the JSON of its values.
type primitiveShape struct {
	// valueNames are the children that hold the primitive's value: those a primitive type defines
	// (their base is a primitive type's element: date.value, or string.value for code), not those
	// every element has (Element.id, Element.extension), which the "_key" sibling holds.
	valueNames []string
	// holdsAnyValue is whether every value with no "_key" sibling meets the children's
	// cardinality: none requires an id or an extension, and none forbids the value.
	holdsAnyValue bool
}

// primitiveShape returns the shape of primitive type typeSD's values, read from its definition.
func (v *Validator) primitiveShape(typeSD *registry.StructureDefinition) primitiveShape {
	if cached, ok := v.shapes.Load(typeSD); ok {
		if shape, ok := cached.(primitiveShape); ok {
			return shape
		}
	}
	shape := primitiveShape{holdsAnyValue: true}
	if root := typeSD.Tree().Root(); root != nil {
		for _, child := range root.Children {
			if v.definedByPrimitive(child) {
				shape.valueNames = append(shape.valueNames, child.Name())
				if child.Def.Max == "0" {
					shape.holdsAnyValue = false
				}
			} else if child.Def.Min > 0 {
				shape.holdsAnyValue = false
			}
		}
	}
	v.shapes.Store(typeSD, shape)
	return shape
}

// definedByPrimitive reports whether an element is first defined by a primitive type: its base
// is an element of a StructureDefinition whose kind is primitive-type.
func (v *Validator) definedByPrimitive(n *registry.ElementNode) bool {
	if n.Def.Base == nil {
		return false
	}
	typeName, _, _ := strings.Cut(n.Def.Base.Path, ".")
	base := v.registry.GetByType(typeName)
	return base != nil && base.Kind == kindPrimitive
}

// unresolvedProfile is the profile an instance's type declares when it cannot be resolved.
type unresolvedProfile struct {
	canonical string
	typeCode  string
	reason    string
}

// childrenOf returns the child elements of an instance of node, inst, and the StructureDefinition
// they belong to (Registry.ChildrenOf), with the definition the instance declares for itself (an
// extension's url, plan B B4c). A declared type profile that does not resolve is returned as
// missing, with no children. The node is never a slice: slices are not children.
func (v *Validator) childrenOf(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, inst map[string]any) (*registry.StructureDefinition, []*registry.ElementNode, unresolvedProfile) {
	c := v.registry.ChildrenOf(context.Background(), sd, node, typeCode, v.selfDefinitions(inst))
	if tp := c.Unresolved; tp.Canonical != "" {
		return nil, nil, unresolvedProfile{tp.Canonical, tp.TypeCode, tp.Reason}
	}
	return c.SD, c.Nodes, unresolvedProfile{}
}

// selfDefinitions returns, for an instance, the definition it declares for itself for a type.
func (v *Validator) selfDefinitions(inst map[string]any) func(string) *registry.StructureDefinition {
	if v.definitions == nil {
		return nil
	}
	return func(typeCode string) *registry.StructureDefinition {
		return v.definitions.DefinitionOf(context.Background(), typeCode, inst)
	}
}

// kindPrimitive is the StructureDefinition.kind of primitive types, whose children (id,
// extension, value) are not properties of a JSON value.
const kindPrimitive = "primitive-type"

// Extracts the element name from a path (e.g., "Patient.name" -> "name").
func getElementName(path string) string {
	lastDot := strings.LastIndex(path, ".")
	if lastDot == -1 {
		return ""
	}
	return path[lastDot+1:]
}
