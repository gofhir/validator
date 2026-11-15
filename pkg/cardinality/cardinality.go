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
	"slices"
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
	v.validateRoot(ctx, data, sd, rootType, result)

	// Walk every resource it holds (contained resources, Bundle entries, the resources other
	// elements hold), each against each profile its meta.profile declares and its type.
	goCtx := ctx
	v.walker.WalkAllWithProfilesContext(ctx, data, rootType, rootType, func(ctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if ctx.FHIRPath == rootType {
			return true
		}
		if ctx.SD != nil && ctx.SD.Snapshot != nil {
			v.validateRoot(goCtx, ctx.Data, ctx.SD, ctx.FHIRPath, result)
		}
		return true
	})

	return result
}

// validateRoot checks a resource (or a datatype value) against the root of sd's tree.
func (v *Validator) validateRoot(ctx context.Context, data map[string]any, sd *registry.StructureDefinition, fhirPath string, result *issue.Result) {
	root := sd.Tree().Root()
	if root == nil {
		return
	}
	v.validateNode(ctx, registry.ResourceOwner(data), data, sd, root, "", fhirPath, nil, result)
}

// validateNode comprueba los hijos de una instancia de node: sus cuentas contra min y max, y,
// y, en cada valor presente, sus hijos. Los hijos son los de cada capa que gobierna el valor, y lo
// que dos capas informan igual se informa una vez. El dueño, owner, es el recurso que contiene el
// valor (registry.Instance).
func (v *Validator) validateNode(
	ctx context.Context,
	owner map[string]any,
	data map[string]any,
	sd *registry.StructureDefinition,
	node *registry.ElementNode,
	typeCode string,
	fhirPath string,
	done registry.Descended,
	result *issue.Result,
) {
	c := v.registry.ChildrenOf(ctx, sd, node, typeCode, v.instance(owner, data))
	// Los perfiles que no se resuelven se informan antes de quitar las capas ya recorridas: el valor que
	// llega por una segunda capa de su padre puede declarar otros perfiles que faltan. Lo que ya se informó
	// al caminarlo por la primera no se repite.
	before := len(result.Issues)
	for _, missing := range c.AllUnresolved {
		// El tipo del valor declara un perfil que no se resuelve: se informa aquí y no otra vez en
		// la fase de slicing, y el valor se comprueba contra el tipo base, como hace el validador
		// de HL7, que informa el perfil como desconocido y sigue.
		result.AddErrorWithID(issue.DiagTypeProfileNotFound,
			map[string]any{"profile": missing.Canonical, "type": missing.TypeCode, "reason": missing.Reason},
			fhirPath)
	}
	if done != nil {
		result.DropRepeated(0, before)
	}
	// Un valor que declaran dos capas de su padre se camina una vez, bajo la unión de las capas.
	c, ok := done.Fresh(data, c)
	if !ok {
		return
	}
	mine := done.Below(c)
	if c.NumLayers() < 2 {
		for i := range c.NumLayers() {
			layer := c.LayerAt(i)
			v.validateChildren(ctx, owner, data, layer.SD, layer.Nodes, fhirPath, true, true, mine, result)
		}
		return
	}
	// Each layer (a profile of the several the value's type declares) checks its children, and what
	// two of them report the same is reported once. The counts of the value's own children come first,
	// those of the values below them after.
	for _, below := range []bool{false, true} {
		start := len(result.Issues)
		for i := range c.NumLayers() {
			from := len(result.Issues)
			layer := c.LayerAt(i)
			v.validateChildren(ctx, owner, data, layer.SD, layer.Nodes, fhirPath, !below, below, mine, result)
			result.DropRepeated(start, from)
		}
	}
}

// validateChildren checks children in one instance, data, held at fhirPath: their counts against
// min and max (counts), and recursively the children of those present (below).
func (v *Validator) validateChildren(ctx context.Context, owner, data map[string]any, childSD *registry.StructureDefinition, children []*registry.ElementNode, fhirPath string, counts, below bool, done registry.Descended, result *issue.Result) {
	for _, child := range children {
		name := child.Name()
		childPath := fhirPath + "." + name
		// Only the values written as the element says are its values (elementvalues.Written).
		values := slices.DeleteFunc(elementvalues.Of(child, data, v.registry), func(cv elementvalues.Value) bool {
			return !elementvalues.Written(child, cv, v.registry)
		})
		count := elementvalues.Occurrences(child, values)

		if counts && child.Def.Min > 0 && count < int(child.Def.Min) {
			result.AddErrorWithID(
				issue.DiagCardinalityMin,
				map[string]any{"path": childPath, "min": child.Def.Min, "count": count},
				childPath,
			)
		}
		if counts && child.Def.Max != "" && child.Def.Max != "*" {
			if maxInt, err := strconv.Atoi(child.Def.Max); err == nil && count > maxInt {
				result.AddErrorWithID(
					issue.DiagCardinalityMax,
					map[string]any{"path": childPath, "max": maxInt, "count": count},
					childPath,
				)
			}
		}

		if !below {
			continue
		}
		for _, cv := range values {
			m, ok := cv.Value.(map[string]any)
			if !ok {
				v.validatePrimitiveElement(ctx, owner, cv.Value, cv.Ext, childSD, child, cv.TypeCode, cv.Path(fhirPath), result)
				continue
			}
			// A resource held in the element (Bundle.entry.resource, contained) is validated
			// against its own definition by the walker.
			if v.registry.HoldsResource(child.Def) {
				continue
			}
			v.validateNode(ctx, owner, m, childSD, child, cv.TypeCode, cv.Path(fhirPath), done, result)
		}
	}
}

// validatePrimitiveElement checks one primitive value against the children of its element: those
// the snapshot unrolls for it (a profile that requires an extension on it), else those of the
// primitive's type definition. In JSON a primitive's value is its own property, and its id and
// extensions are in its "_key" sibling, ext (json.html#primitive): the value is checked as the child
// the primitive type defines for it, the others against the sibling, even when there is none.
func (v *Validator) validatePrimitiveElement(ctx context.Context, owner map[string]any, value any, ext map[string]any, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode, fhirPath string, result *issue.Result) {
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
	v.validateChildren(ctx, owner, inst, childSD, children, fhirPath, true, true, nil, result)
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

// instance es la instancia inst, de dueño owner, con la definición que declara para sí (la url de una
// extensión, plan B B4c).
func (v *Validator) instance(owner, inst map[string]any) registry.Instance {
	return registry.Instance{Value: inst, Self: v.definitions, Owner: owner}
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
