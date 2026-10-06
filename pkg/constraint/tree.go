package constraint

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/internal/fhirpathcache"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
)

// The constraint phase walks the instance and the StructureDefinition's element tree together
// (registry.ElementTree), as cardinality and slicing do. Every value is checked against every
// definition that governs it (plan B, definition layering):
//
//   - the element itself;
//   - the element its contentReference points to;
//   - the one profile its type declares for the value (type.profile), else the type's definition.
//
// A value of a sliced element is governed first by the slice it belongs to, as slice matching
// assigns it (slicematch), then by each slice that slice reslices, and by the element itself; the
// type layer is the profile the most specific of them declares. A rule that two of them share (the
// same key and expression) is evaluated once per value, under the most specific.
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

// The walk checks one value of node, held at fhirPath, and recurses into its children. The node is
// the most specific definition of the value: the slice it belongs to, or the element. The value's
// type is typeCode (a choice element's type, or its only type).
func (v *Validator) walk(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, value any, raw, element json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	seen := map[string]bool{}
	base := node
	for n := node; n != nil; n = n.SliceOf {
		v.layer(n, choiceElementPath(n.Def.Path, typeCode), value, &raw, element, fhirPath, seen, opts, result)
		base = n
	}

	layerSD, layer, hops := sd, base, 0
	for layer.Def.ContentReference != nil && hops < maxContentReferenceHops {
		target, tsd := v.contentTarget(layerSD, layer)
		if target == nil {
			break
		}
		v.layer(target, target.Def.Path, value, &raw, element, fhirPath, seen, opts, result)
		layerSD, layer, hops = tsd, target, hops+1
	}

	self := v.selfDefinition(opts.ctx, layer, typeCode, value)
	typeSD, typeRoot := v.typeLayer(opts.ctx, node, layer, typeCode, self)
	if typeRoot != nil && typeSD.Kind != kindPrimitive {
		v.layer(typeRoot, typeRoot.Def.Path, value, &raw, element, fhirPath, seen, opts, result)
	}
	if self != nil && self != typeSD {
		// The slice's profile governs the value, and so does the definition it declares itself.
		if root := self.Tree().Root(); root != nil {
			v.layer(root, root.Def.Path, value, &raw, element, fhirPath, seen, opts, result)
		}
	}

	if obj, ok := value.(map[string]any); ok {
		childSD, children := sd, governedChildren(node)
		if len(children) == 0 {
			childSD, children = layerSD, layer.Children
		}
		if len(children) == 0 && typeRoot != nil {
			childSD, children = typeSD, typeRoot.Children
		}
		v.walkChildren(childSD, children, obj, raw, fhirPath, opts, result)
	}
}

// governedChildren are the children that govern a value of node in its own snapshot: node's, or,
// for a slice whose children the snapshot does not unroll, those of the element it slices.
func governedChildren(node *registry.ElementNode) []*registry.ElementNode {
	for n := node; n != nil; n = n.SliceOf {
		if len(n.Children) > 0 {
			return n.Children
		}
	}
	return nil
}

// governing is the most specific definition of one value of node: the slice slice matching assigns
// it to, or node itself when node is not sliced or the value is in none of its slices.
func (v *Validator) governing(sd *registry.StructureDefinition, node *registry.ElementNode, cv elementvalues.Value, opts *constraintEvalOpts) *registry.ElementNode {
	if node.Def.Slicing == nil || len(node.Slices) == 0 {
		return node
	}
	m := v.matcher.Resolve(opts.ctx, slicematch.Request{
		SD: sd, Node: node, Key: cv.Key, Value: cv.Value, Valueless: cv.Value == nil && cv.Ext != nil,
		Scope: opts.scope, Resolver: opts.sliceResolver, Containment: opts.containment,
	})
	if m.Matched {
		return m.Node
	}
	return node
}

// walkPrimitiveExtensions walks a primitive's "_key" sibling (its id and extensions), whose JSON
// is raw, against the children of the primitive's type definition.
func (v *Validator) walkPrimitiveExtensions(node *registry.ElementNode, typeCode string, ext map[string]any, raw json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	typeSD, typeRoot := v.typeLayer(opts.ctx, node, node, typeCode, nil)
	if typeRoot == nil || typeSD.Kind != kindPrimitive {
		return
	}
	v.walkChildren(typeSD, typeRoot.Children, ext, raw, fhirPath, opts, result)
}

// walkChildren walks the values inst holds for each of children. Given raw, inst's JSON as the
// resource spells it (nil when inst was not read from it), each value is evaluated on its own JSON,
// read from raw, so that a decimal keeps its text (1.50 is not 1.5, json.html#primitive).
func (v *Validator) walkChildren(sd *registry.StructureDefinition, children []*registry.ElementNode, inst map[string]any, raw json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	fields := rawFields(raw)
	for _, child := range children {
		for _, cv := range elementvalues.Of(child, inst, v.registry.ChoiceType) {
			path := cv.Path(fhirPath)
			valueRaw, extRaw := fields.of(cv.Key, cv), fields.of("_"+cv.Key, cv)
			if m, ok := cv.Value.(map[string]any); ok {
				if _, isResource := m[resourceTypeKey]; isResource {
					v.validateNested(child, m, valueRaw, path, opts, result)
					continue
				}
			}
			node := v.governing(sd, child, cv, opts)
			if cv.Value != nil {
				v.walk(sd, node, cv.TypeCode, cv.Value, valueRaw, extRaw, path, opts, result)
			} else if extRaw != nil {
				v.checkValueless(sd, node, extRaw, path, opts, result)
			}
			if cv.Ext != nil {
				v.walkPrimitiveExtensions(node, cv.TypeCode, cv.Ext, extRaw, path, opts, result)
			}
		}
	}
}

// jsonFields are the JSON of an object's properties as written, and of the items of those that are
// arrays, read once (objectSpans, arraySpans).
type jsonFields struct {
	fields map[string][]byte
	items  map[string][][]byte
}

// rawFields reads the properties of raw, a JSON object; none when raw is nil or not an object.
func rawFields(raw json.RawMessage) *jsonFields {
	if raw == nil {
		return nil
	}
	fields := objectSpans(raw)
	if fields == nil {
		return nil
	}
	return &jsonFields{fields: fields}
}

// of is the JSON of the value cv reads from the property key: the property's, or the item's at
// cv.Index when cv is read from an array (every position counts, nulls included). Nil when there
// is none.
func (f *jsonFields) of(key string, cv elementvalues.Value) json.RawMessage {
	if f == nil {
		return nil
	}
	raw, ok := f.fields[key]
	if !ok {
		return nil
	}
	if !cv.Array {
		return raw
	}
	items, ok := f.items[key]
	if !ok {
		items = arraySpans(raw)
		if f.items == nil {
			f.items = map[string][][]byte{}
		}
		f.items[key] = items
	}
	if cv.Index < 0 || cv.Index >= len(items) {
		return nil
	}
	return items[cv.Index]
}

// validateNested checks a resource held in an element as a resource of its own, against each
// profile its meta.profile declares that resolves, or else its type's definition. Its JSON, as the
// resource validated spells it, is raw, or nil when it was not read from it. A Bundle is where
// resolve() looks first, before the Bundles that hold it (bundle.html#references), as the HL7
// validator looks.
func (v *Validator) validateNested(holder *registry.ElementNode, res map[string]any, raw json.RawMessage, fhirPath string, parent *constraintEvalOpts, result *issue.Result) {
	if raw == nil {
		var err error
		if raw, err = json.Marshal(res); err != nil {
			return
		}
	}
	col, err := types.JSONToCollection(raw)
	if err != nil {
		return
	}
	fhirpathcache.Enable(col)

	opts := *parent
	opts.resourceCol, opts.rootResourceCol = col, col
	opts.scope = slicematch.Scope{Resource: res, RootResource: res, Container: parent.scope.Container}
	if holder.Def.Base != nil && holder.Def.Base.Path == containedBase {
		opts.rootResourceCol = parent.resourceCol
		opts.scope.RootResource = parent.scope.Resource
	}
	if rt, _ := res[resourceTypeKey].(string); rt == bundleType {
		opts.resolver = resolverInBundle(opts.resolver, res, opts.exact)
	}
	opts.resolver = resolverWithin(opts.resolver, opts.scope.RootResource, nil)

	for _, sd := range v.nestedDefinitions(opts.ctx, res, fhirPath, result) {
		if root := sd.Tree().Root(); root != nil {
			v.walk(sd, root, "", res, raw, nil, fhirPath, &opts, result)
		}
	}
}

// nestedDefinitions are the definitions a nested resource at fhirPath is checked against: the
// profiles its meta.profile declares that resolve (declaredProfile), each once, and its type's
// definition unless a profile of its type stands for it, as the HL7 validator checks a resource
// against its type's definition besides its profiles.
func (v *Validator) nestedDefinitions(ctx context.Context, res map[string]any, fhirPath string, result *issue.Result) []*registry.StructureDefinition {
	rt, _ := res[resourceTypeKey].(string)
	var out []*registry.StructureDefinition
	ofType := false
	if meta, ok := res["meta"].(map[string]any); ok {
		profiles, _ := meta["profile"].([]any)
		for i, p := range profiles {
			url, ok := p.(string)
			if !ok {
				continue
			}
			if sd := v.declaredProfile(ctx, url, fmt.Sprintf("%s.meta.profile[%d]", fhirPath, i), fhirPath, result); sd != nil && !slices.Contains(out, sd) {
				out = append(out, sd)
				ofType = ofType || sd.Type == rt
			}
		}
	}
	if !ofType && rt != "" {
		if sd := v.registry.GetByType(rt); sd != nil && sd.Snapshot != nil {
			out = append(out, sd)
		}
	}
	return out
}

// declaredProfile resolves the profile url a nested resource at fhirPath declares at its
// meta.profile entry at: the version a canonical pins, or the one an unversioned canonical resolves
// to (references.html#canonical). One that does not resolve is reported at the entry, and one whose
// snapshot cannot be generated at the resource, as the HL7 validator reports them; once per
// location.
func (v *Validator) declaredProfile(ctx context.Context, url, at, fhirPath string, result *issue.Result) *registry.StructureDefinition {
	sd, resolution, err := v.registry.ResolveProfile(ctx, url)
	switch {
	case sd != nil:
		return sd
	case err != nil:
		if firstReportKey(ctx, fhirPath+"\x00"+string(issue.DiagProfileSnapshotFailed)+"\x00"+url) {
			result.AddErrorWithID(issue.DiagProfileSnapshotFailed, map[string]any{"url": url, "reason": err.Error()}, fhirPath)
		}
	default:
		if firstReportKey(ctx, at+"\x00"+string(issue.DiagProfileNotFound)) {
			result.AddWarningWithID(issue.DiagProfileNotFound, map[string]any{"url": url, "reason": resolution.Reason()}, at)
		}
	}
	return nil
}

// typeLayer is the definition that governs a value through its type: the one profile that the type
// of node, or of an element node slices, declares for typeCode, the most specific first; else the
// definition the value declares itself (self); else the type's own definition, as layer (node, or
// the element its contentReference points to) declares it; with its root element. A declared
// profile that does not resolve (reported by the cardinality phase) leaves the value under its
// type's definition, as the HL7 validator checks it.
func (v *Validator) typeLayer(ctx context.Context, node, layer *registry.ElementNode, typeCode string, self *registry.StructureDefinition) (*registry.StructureDefinition, *registry.ElementNode) {
	for n := node; n != nil; n = n.SliceOf {
		if tp := v.registry.TypeProfile(ctx, n, typeCode); tp.SD != nil {
			return tp.SD, tp.SD.Tree().Root()
		}
	}
	if layer != node {
		if tp := v.registry.TypeProfile(ctx, layer, typeCode); tp.SD != nil {
			return tp.SD, tp.SD.Tree().Root()
		}
	}
	if self != nil {
		if root := self.Tree().Root(); root != nil {
			return self, root
		}
	}
	code := valueType(layer, typeCode)
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

// valueType is the type of a value of layer: typeCode, the type a choice element's value has, or
// else the element's only type.
func valueType(layer *registry.ElementNode, typeCode string) string {
	if typeCode == "" && len(layer.Def.Type) == 1 {
		return layer.Def.Type[0].Code
	}
	return typeCode
}

// selfDefinition is the definition a value of layer declares for itself, which governs it wherever
// it is used: an extension's url names the definition the extension conforms to
// (extensibility.html). The DefinitionSource knows which values declare one; without it, none
// does.
func (v *Validator) selfDefinition(ctx context.Context, layer *registry.ElementNode, typeCode string, value any) *registry.StructureDefinition {
	obj, ok := value.(map[string]any)
	if !ok || v.definitions == nil {
		return nil
	}
	return v.definitions.DefinitionOf(ctx, valueType(layer, typeCode), obj)
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

// layer checks value, at fhirPath, against def, one of the definitions that govern it: its
// invariants, and what the ValueChecker checks (its fixed and pattern values), element being a
// primitive's "_x" sibling. An issue is reported once per location, however many definitions or
// profiles find it.
func (v *Validator) layer(def *registry.ElementNode, defPath string, value any, raw *json.RawMessage, element json.RawMessage, fhirPath string, seen map[string]bool, opts *constraintEvalOpts, result *issue.Result) {
	v.evaluate(def, defPath, value, raw, fhirPath, seen, opts, result)
	if v.values == nil || !v.values.Governs(def.Def) {
		return
	}
	if *raw == nil {
		b, err := json.Marshal(value)
		if err != nil {
			return
		}
		*raw = b
	}
	v.checkValue(def, *raw, element, fhirPath, opts, result)
}

// checkValue checks a value at fhirPath, raw with its "_x" sibling element, against def's fixed
// and pattern values. An issue is reported once per location.
func (v *Validator) checkValue(def *registry.ElementNode, raw, element json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	if v.values == nil || !v.values.Governs(def.Def) {
		return
	}
	v.values.CheckValue(def.Def, raw, element, fhirPath, func(id issue.DiagnosticID, params map[string]any, at string) {
		if firstReportKey(opts.ctx, at+"\x00"+string(id)+"\x00"+fmt.Sprint(params)) {
			result.AddErrorWithID(id, params, at)
		}
	})
}

// checkValueless checks a primitive that has extensions and no value (json.html#primitive), at
// fhirPath with its "_x" sibling element, against the fixed and pattern values of the definitions
// that govern it, as walk layers them: node, the slices it reslices, and the elements a
// contentReference points to. A primitive type's definition has none.
func (v *Validator) checkValueless(sd *registry.StructureDefinition, node *registry.ElementNode, element json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	if v.values == nil {
		return
	}
	base := node
	for n := node; n != nil; n = n.SliceOf {
		v.checkValue(n, nil, element, fhirPath, opts, result)
		base = n
	}
	layerSD, layer := sd, base
	for hops := 0; layer.Def.ContentReference != nil && hops < maxContentReferenceHops; hops++ {
		target, tsd := v.contentTarget(layerSD, layer)
		if target == nil {
			break
		}
		v.checkValue(target, nil, element, fhirPath, opts, result)
		layerSD, layer = tsd, target
	}
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
