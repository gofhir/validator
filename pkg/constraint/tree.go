package constraint

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/internal/elementvalues"
	"github.com/gofhir/validator/internal/fhirpathcache"
	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// The constraint phase walks the instance and the StructureDefinition's element tree together
// (registry.ElementTree), as cardinality and slicing do. Every value is checked against every
// definition that governs it (plan B, definition layering):
//
//   - the element itself;
//   - the element its contentReference points to;
//   - the one profile its type declares for the value (type.profile), else the type's definition.
//
// A rule that two of them share (the same key and expression) is evaluated once per value, under
// the most specific. Slices are left to a later step: a value is checked against the unsliced
// element.
//
// A resource held in an element (Bundle.entry.resource, contained, Parameters.parameter.resource)
// is checked as a resource of its own, against the profiles its meta.profile declares or its type's
// definition, with %resource set to it.

// kindPrimitive is the StructureDefinition.kind of primitive types.
const kindPrimitive = "primitive-type"

// resourceTypeKey is the FHIR JSON property that names a resource's type (json.html#resources).
const resourceTypeKey = "resourceType"

// containedBase is the element every resource's contained derives from (ElementDefinition.base):
// a resource it holds takes its container as %rootResource (fhirpath.html#variables).
const containedBase = "DomainResource.contained"

// maxContentReferenceHops bounds a chain of contentReferences (a cycle in a malformed definition).
const maxContentReferenceHops = 8

// The walk checks one value of node, held at fhirPath, and recurses into its children. The value's
// type is typeCode (a choice element's type, or its only type).
func (v *Validator) walk(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, value any, raw json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	seen := map[string]bool{}
	v.evaluate(node, choiceElementPath(node.Def.Path, typeCode), value, &raw, fhirPath, seen, opts, result)

	layerSD, layer, hops := sd, node, 0
	for layer.Def.ContentReference != nil && hops < maxContentReferenceHops {
		target, tsd := v.contentTarget(layerSD, layer)
		if target == nil {
			break
		}
		v.evaluate(target, target.Def.Path, value, &raw, fhirPath, seen, opts, result)
		layerSD, layer, hops = tsd, target, hops+1
	}

	typeSD, typeRoot := v.typeLayer(opts.ctx, layer, typeCode)
	if typeRoot != nil && typeSD.Kind != kindPrimitive {
		v.evaluate(typeRoot, typeRoot.Def.Path, value, &raw, fhirPath, seen, opts, result)
	}

	if obj, ok := value.(map[string]any); ok {
		childSD, children := layerSD, layer.Children
		if len(children) == 0 && typeRoot != nil {
			childSD, children = typeSD, typeRoot.Children
		}
		v.walkChildren(childSD, children, obj, fhirPath, opts, result)
	}
}

// walkPrimitiveExtensions walks a primitive's "_key" sibling (its id and extensions) against the
// children of the primitive's type definition.
func (v *Validator) walkPrimitiveExtensions(node *registry.ElementNode, typeCode string, ext map[string]any, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	typeSD, typeRoot := v.typeLayer(opts.ctx, node, typeCode)
	if typeRoot == nil || typeSD.Kind != kindPrimitive {
		return
	}
	v.walkChildren(typeSD, typeRoot.Children, ext, fhirPath, opts, result)
}

// walkChildren walks the values inst holds for each of children.
func (v *Validator) walkChildren(sd *registry.StructureDefinition, children []*registry.ElementNode, inst map[string]any, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	for _, child := range children {
		for _, cv := range elementvalues.Of(child, inst, v.registry.ChoiceType) {
			path := cv.Path(fhirPath)
			if m, ok := cv.Value.(map[string]any); ok {
				if _, isResource := m[resourceTypeKey]; isResource {
					v.validateNested(child, m, path, opts, result)
					continue
				}
			}
			if cv.Value != nil {
				v.walk(sd, child, cv.TypeCode, cv.Value, nil, path, opts, result)
			}
			if cv.Ext != nil {
				v.walkPrimitiveExtensions(child, cv.TypeCode, cv.Ext, path, opts, result)
			}
		}
	}
}

// validateNested checks a resource held in an element as a resource of its own, against each
// profile its meta.profile declares that resolves, or else its type's definition.
func (v *Validator) validateNested(holder *registry.ElementNode, res map[string]any, fhirPath string, parent *constraintEvalOpts, result *issue.Result) {
	raw, err := json.Marshal(res)
	if err != nil {
		return
	}
	col, err := types.JSONToCollection(raw)
	if err != nil {
		return
	}
	fhirpathcache.Enable(col)

	opts := *parent
	opts.resourceCol, opts.rootResourceCol = col, col
	if holder.Def.Base != nil && holder.Def.Base.Path == containedBase {
		opts.rootResourceCol = parent.resourceCol
	}

	for _, sd := range v.nestedDefinitions(opts.ctx, res) {
		if root := sd.Tree().Root(); root != nil {
			v.walk(sd, root, "", res, raw, fhirPath, &opts, result)
		}
	}
}

// nestedDefinitions are the definitions a nested resource is checked against: the profiles its
// meta.profile declares that resolve, or else its type's definition.
func (v *Validator) nestedDefinitions(ctx context.Context, res map[string]any) []*registry.StructureDefinition {
	var out []*registry.StructureDefinition
	if meta, ok := res["meta"].(map[string]any); ok {
		if profiles, ok := meta["profile"].([]any); ok {
			for _, p := range profiles {
				if url, ok := p.(string); ok {
					if sd, _ := v.registry.ResolveCanonical(url); sd != nil && v.registry.EnsureSnapshot(ctx, sd) == nil {
						out = append(out, sd)
					}
				}
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	if rt, _ := res[resourceTypeKey].(string); rt != "" {
		if sd := v.registry.GetByType(rt); sd != nil && sd.Snapshot != nil {
			out = append(out, sd)
		}
	}
	return out
}

// typeLayer is the definition that governs a value of node through its type: the one profile its
// type declares for typeCode, else the type's own definition, with its root element. A declared
// profile that does not resolve (reported by the cardinality phase) leaves the value under its
// type's definition, as the HL7 validator checks it.
func (v *Validator) typeLayer(ctx context.Context, node *registry.ElementNode, typeCode string) (*registry.StructureDefinition, *registry.ElementNode) {
	if tp := v.registry.TypeProfile(ctx, node, typeCode); tp.SD != nil {
		return tp.SD, tp.SD.Tree().Root()
	}
	code := typeCode
	if code == "" && len(node.Def.Type) == 1 {
		code = node.Def.Type[0].Code
	}
	if code == "" {
		return nil, nil
	}
	sd := v.registry.GetByType(code)
	if sd == nil || sd.Snapshot == nil {
		return nil, nil
	}
	root := sd.Tree().Root()
	if root == nil {
		return nil, nil
	}
	return sd, root
}

// contentTarget is the element node's contentReference points to, with the definition it is in.
func (v *Validator) contentTarget(sd *registry.StructureDefinition, node *registry.ElementNode) (*registry.ElementNode, *registry.StructureDefinition) {
	target, _ := v.registry.ContentReference(sd, node)
	if target == nil {
		return nil, nil
	}
	tsd := sd
	if url, _, ok := registry.SplitContentReference(*node.Def.ContentReference); ok && url != "" && url != sd.URL {
		if s, _ := v.registry.ResolveCanonical(url); s != nil {
			tsd = s
		}
	}
	return target, tsd
}

// evaluate checks the constraints of one definition on a value, skipping those already evaluated
// on it (seen). The value's JSON is built once, on the first constraint that needs it.
func (v *Validator) evaluate(def *registry.ElementNode, defPath string, value any, raw *json.RawMessage, fhirPath string, seen map[string]bool, opts *constraintEvalOpts, result *issue.Result) {
	var pending []registry.Constraint
	for _, c := range def.Def.Constraint {
		k := c.Key + "\x00" + c.Expression
		if c.Expression == "" || seen[k] {
			continue
		}
		seen[k] = true
		pending = append(pending, c)
	}
	if len(pending) == 0 {
		return
	}
	if *raw == nil {
		b, err := json.Marshal(value)
		if err != nil {
			return
		}
		*raw = b
	}
	v.evaluateConstraintsWithCtx(*raw, pending, fhirPath, defPath, opts, result)
}

// choiceElementPath renders the concrete path of a choice element for the type a value has:
// "Observation.value[x]" with Quantity is "Observation.valueQuantity" (the property name is the
// base name followed by the type, capitalized). Any other path is returned unchanged.
func choiceElementPath(elemPath, typeCode string) string {
	base, ok := strings.CutSuffix(elemPath, "[x]")
	if !ok || typeCode == "" {
		return elemPath
	}
	return base + strings.ToUpper(typeCode[:1]) + typeCode[1:]
}

// rootCollection reads a resource's JSON as the FHIRPath collection %resource and %rootResource
// are bound to.
func rootCollection(raw json.RawMessage) fhirpath.Collection {
	col, err := types.JSONToCollection(raw)
	if err != nil {
		return nil
	}
	fhirpathcache.Enable(col)
	return col
}
