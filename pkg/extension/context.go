package extension

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gofhir/validator/internal/elementvalues"
	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// The context of use of an extension (defining-extensions.html#context): "Extensions SHALL only
// be used on a target that appears in their context list". The target is the element that holds
// the extension, and what it is comes from the definitions the instance is walked with, never from
// its JSON path:
//
//   - an element context is an element id: the target matches the id of the element it
//     instantiates and the element that one is based on (base.path), the element a contentReference
//     points to, the root of its type and of each of the type's ancestors, and its path from the
//     resource through the data types it is in (Patient.name.given), which the extensions packages
//     write for elements of data types (decisions B-D2 to B-D5);
//   - an extension context is the url of the extension that holds it, or url#code for a
//     sub-extension of a complex extension;
//   - a fhirpath context selects the target: it is one of the nodes the expression returns,
//     evaluated from the root of the resource the target is in, the same node and not an equal one;
//   - every context invariant holds, evaluated on the element that holds the extension.
//
// An extension whose definition declares no context is used on no target.

// FHIRPathEvaluator evaluates the FHIRPath expressions of a context of use. Implemented by the
// constraint phase, which evaluates the profiles' invariants the same way.
type FHIRPathEvaluator interface {
	// Scope returns what evaluates expressions on root's resource.
	Scope(ctx context.Context, root ScopeRoot) FHIRPathScope
}

// ScopeRoot is the resource a FHIRPathScope evaluates expressions on.
type ScopeRoot struct {
	// Resource is the resource, as parsed.
	Resource map[string]any
	// Raw is the JSON Resource was parsed from, which the expressions read; nil reads Resource.
	Raw []byte
	// Bundle is the Bundle being validated, which resolve() finds references in, when Resource is
	// in one (a slice's conformance check); a Bundle is its own.
	Bundle map[string]any
	// Exact returns an object of Resource, or of Bundle, with its numbers as the JSON spells them, or
	// nil when it has none (see Data.Exact).
	Exact func(map[string]any) map[string]any
}

// FHIRPathScope evaluates expressions on one resource (FHIRPathEvaluator.Scope). Places are its
// root's: "Patient.name[0]", "Patient.name[0].given[1]" for a primitive's element,
// "Observation.contained[0].name[0]". An evaluation stopped at the validator's time limit is an
// error that wraps context.DeadlineExceeded.
type FHIRPathScope interface {
	// Holds reports whether expression is true on value, an element whose definition path, as the
	// definitions type it, is focusPath: an object, or a primitive's value, with element its "_key"
	// sibling.
	Holds(expression, focusPath string, value any, element map[string]any) (bool, error)
	// Selects reports whether expression, evaluated from the resource, returns the node at
	// location.
	Selects(expression, location string) (bool, error)
	// Within returns the scope of resource, at at, one the resource holds: contained, its
	// %rootResource is the resource; otherwise its own. When resource is a Bundle, resolve() looks
	// in it first, before the Bundles that hold it.
	Within(at string, resource map[string]any, contained bool) FHIRPathScope
}

// SetFHIRPathEvaluator sets what evaluates fhirpath contexts and context invariants. Without it,
// neither is evaluated: a fhirpath context allows no target, and context invariants are not
// checked.
func (v *Validator) SetFHIRPathEvaluator(e FHIRPathEvaluator) {
	v.fhirpath = e
}

// target is an element that may hold extensions: where it is in the instance and in the
// definitions.
type target struct {
	sd       *registry.StructureDefinition // the definition node is in
	node     *registry.ElementNode
	typeCode string // the value's type, for a choice element; "" for its only type
	path     string // its path from the resource, through the data types it is in
	location string // where it is in the instance (FHIRPath)
	// local is where it is in root: "Patient.name[0]", "Observation.contained[0].name[0]".
	local string
	// value is the target: an object, or a primitive's value, with element its "_key" sibling;
	// holder holds it, under key, at index in an array (-1 for none).
	value   any
	element map[string]any
	holder  map[string]any
	key     string
	index   int
	// exact returns an object with its numbers as the JSON spells them (Data.Exact); nil for none.
	exact func(map[string]any) map[string]any
	// scope evaluates expressions on the resource it is in; nil without a FHIRPathEvaluator.
	scope FHIRPathScope
	// extensions are the extension contexts it is: the url of an extension, url#code of a
	// sub-extension.
	extensions []string
}

// place is where a resource is: for one another holds, local, its place in the root, the scope of
// the resource that holds it, and whether that one contains it; for the root, what its scope reads.
// Its exact function returns an object with its numbers as the JSON spells them.
type place struct {
	local     string
	parent    FHIRPathScope
	contained bool
	root      ScopeRoot
	exact     func(map[string]any) map[string]any
}

// checkContexts checks the context of use of every extension in resource, an instance of its
// type's definition, and in the resources it holds, resource being where in says.
func (v *Validator) checkContexts(ctx context.Context, resource map[string]any, location string, in place, result *issue.Result) {
	rt, _ := resource[resourceTypeKey].(string)
	sd := v.registry.GetByType(rt)
	if sd == nil || sd.Snapshot == nil {
		return
	}
	rootNode := sd.Tree().Root()
	if rootNode == nil {
		return
	}
	t := target{sd: sd, node: rootNode, path: rt, location: location, local: in.local, value: resource, index: -1, exact: in.exact}
	switch {
	case in.parent != nil:
		t.scope = in.parent.Within(in.local, resource, in.contained)
	case v.fhirpath != nil:
		root := in.root
		root.Resource = resource
		t.local, t.scope = rt, v.fhirpath.Scope(ctx, root)
	default:
		t.local = rt
	}
	v.walkTargets(ctx, t, resource, result)
}

// walkTargets checks the extensions t holds, held in inst (t's object, or a primitive's "_key"
// sibling), and walks into its children.
func (v *Validator) walkTargets(ctx context.Context, t target, inst map[string]any, result *issue.Result) {
	children := v.registry.ChildrenOf(ctx, t.sd, t.node, t.typeCode, func(typeCode string) *registry.StructureDefinition {
		return v.DefinitionOf(ctx, typeCode, inst)
	})
	for _, child := range children.Nodes {
		for _, cv := range elementvalues.Of(child, inst, v.registry.ChoiceType) {
			location := cv.Path(t.location)
			m, isObject := cv.Value.(map[string]any)
			if _, isResource := m[resourceTypeKey]; isObject && isResource {
				v.checkHeld(ctx, t, child, cv, m, result)
				continue
			}
			next := target{
				sd: children.SD, node: child, typeCode: cv.TypeCode, path: t.path + "." + child.Name(),
				location: location, local: cv.Path(t.local), value: cv.Value, element: cv.Ext, scope: t.scope,
				holder: inst, key: cv.Key, index: -1, exact: t.exact,
			}
			if cv.Array {
				next.index = cv.Index
			}
			if isObject {
				next.extensions = v.extensionsOf(ctx, t, valueTypeOf(child, cv.TypeCode), m, result)
				v.walkTargets(ctx, next, m, result)
			}
			if cv.Ext != nil {
				v.walkTargets(ctx, next, cv.Ext, result)
			}
		}
	}
}

// checkHeld checks the contexts in res, a resource t holds in child, read as a node of the root:
// a contained resource with its container as %rootResource, any other with itself.
func (v *Validator) checkHeld(ctx context.Context, t target, child *registry.ElementNode, cv elementvalues.Value, res map[string]any, result *issue.Result) {
	location := cv.Path(t.location)
	if t.scope == nil {
		v.checkContexts(ctx, res, location, place{exact: t.exact}, result)
		return
	}
	contained := child.Def.Base != nil && child.Def.Base.Path == containedBase
	v.checkContexts(ctx, res, location, place{local: cv.Path(t.local), parent: t.scope, contained: contained, exact: t.exact}, result)
}

// extensionsOf checks the context of m, a value of typeCode t holds, when m is an extension whose
// definition is loaded, and returns the extension contexts m is (target.extensions).
func (v *Validator) extensionsOf(ctx context.Context, t target, typeCode string, m map[string]any, result *issue.Result) []string {
	url, _ := m[keyURL].(string)
	if typeCode == extensionType && absoluteURLDefect(url) == "" {
		// The definition may be one only the external resolver has: resolving it keeps it in the
		// registry, where DefinitionOf finds it.
		v.registry.ResolveByCanonical(ctx, url, "")
	}
	switch def := v.DefinitionOf(ctx, typeCode, m); {
	case def != nil:
		v.checkContext(ctx, def, t, result)
		return []string{def.URL}
	case typeCode == extensionType && absoluteURLDefect(url) == "":
		// An extension whose definition is not loaded is still the extension its url names, the one
		// an extension context names.
		return []string{url}
	case url != "":
		// A sub-extension of a complex extension is named by its code within it.
		names := make([]string, 0, len(t.extensions))
		for _, parent := range t.extensions {
			names = append(names, parent+"#"+url)
		}
		return names
	}
	return nil
}

// checkContext checks that def, the definition of an extension t holds, allows t as its target.
func (v *Validator) checkContext(ctx context.Context, def *registry.StructureDefinition, t target, result *issue.Result) {
	var names []string // the names of t, worked out when a context or the issue needs them
	named := func() []string {
		if names == nil {
			names = v.targetNames(t)
		}
		return names
	}
	allowed := false
	for _, c := range def.Context {
		switch c.Type {
		case contextElement:
			allowed = slices.Contains(named(), c.Expression)
		case contextExtension:
			allowed = v.isExtensionContext(t, c.Expression)
		case contextFHIRPath:
			selected, err := v.selects(t, c.Expression)
			if undecided(ctx, err, c.Expression, t, result) {
				return
			}
			allowed = selected
		}
		if allowed {
			break
		}
	}
	if !allowed {
		result.AddErrorWithID(issue.DiagExtensionInvalidContext, map[string]any{
			keyURL: def.URL, "context": strings.Join(named(), ", "),
		}, t.location)
		return
	}
	if t.scope == nil {
		return
	}
	focusPath := v.typedPath(t)
	for _, expression := range def.ContextInvariant {
		holds, err := t.scope.Holds(expression, focusPath, t.exactValue(), exactOf(t.exact, t.element))
		if undecided(ctx, err, expression, t, result) {
			return
		}
		if !holds {
			// The first that does not hold is reported, as the HL7 validator reports it.
			result.AddErrorWithID(issue.DiagExtensionContextInvariant, map[string]any{
				keyURL: def.URL, "expression": expression,
			}, t.location)
			return
		}
	}
}

// exactValue is t's value with its numbers as the JSON spells them, when t.exact knows it: the
// exact twin of an object, or the value its holder's exact twin holds.
func (t target) exactValue() any {
	if t.exact == nil {
		return t.value
	}
	if m, ok := t.value.(map[string]any); ok {
		return exactOf(t.exact, m)
	}
	h := t.exact(t.holder)
	if h == nil {
		return t.value
	}
	v, ok := h[t.key]
	if !ok {
		return t.value
	}
	if t.index < 0 {
		return v
	}
	if a, ok := v.([]any); ok && t.index < len(a) {
		return a[t.index]
	}
	return t.value
}

// exactOf is exact's twin of m, or m when there is none.
func exactOf(exact func(map[string]any) map[string]any, m map[string]any) map[string]any {
	if exact == nil || m == nil {
		return m
	}
	if e := exact(m); e != nil {
		return e
	}
	return m
}

// typedPath is the definition path that types t's value, as the constraint phase types a value: the
// element a contentReference points to, and for a choice element the name its value's type gives
// it (Observation.valueQuantity, formats.html#choice).
func (v *Validator) typedPath(t target) string {
	node := t.node
	if node.Def.ContentReference != nil {
		if ref, _ := v.registry.ContentReference(t.sd, node); ref != nil {
			node = ref
		}
	}
	base, choice := strings.CutSuffix(node.Def.Path, "[x]")
	if !choice || t.typeCode == "" {
		return node.Def.Path
	}
	return base + strings.ToUpper(t.typeCode[:1]) + t.typeCode[1:]
}

// undecided reports whether evaluating expression for t failed in a way that decides nothing: the
// validation was canceled, or the evaluation stopped at the validator's own time limit, which is
// reported as a processing notice, as the constraint phase reports it. Any other failure is an
// expression that does not hold, or selects nothing.
func undecided(ctx context.Context, err error, expression string, t target, result *issue.Result) bool {
	switch {
	case err == nil:
		return false
	case ctx.Err() != nil:
		return true
	case errors.Is(err, context.DeadlineExceeded):
		result.AddWarningWithID(issue.DiagConstraintEvalError, map[string]any{"key": expression, "error": err.Error()}, t.location)
		return true
	}
	return false
}

// containedBase is the element every resource's contained derives from (ElementDefinition.base): a
// resource it holds takes its container as %rootResource (fhirpath.html#variables).
const containedBase = "DomainResource.contained"

// extensionType is the type of an extension.
const extensionType = "Extension"

// The context types (StructureDefinition.context.type).
const (
	contextElement   = "element"
	contextExtension = "extension"
	contextFHIRPath  = "fhirpath"
)

// isExtensionContext reports whether t is the extension, or the sub-extension, expression names: a
// canonical, whatever version it pins (structuredefinition.html: "the context is a particular
// extension from a particular StructureDefinition"), followed by #code for a sub-extension.
func (v *Validator) isExtensionContext(t target, expression string) bool {
	canonical, code, _ := strings.Cut(expression, "#")
	url, _ := registry.ParseCanonical(canonical)
	if code != "" {
		url += "#" + code
	}
	return slices.Contains(t.extensions, url)
}

// selects reports whether expression, a fhirpath context, selects t: whether t is one of the nodes
// it returns, evaluated from the root of the resource t is in ("The FHIRPath statement always
// starts from the root of the resource", defining-extensions.html#context). An expression that
// does not compile, or whose evaluation fails, selects nothing.
func (v *Validator) selects(t target, expression string) (bool, error) {
	if t.scope == nil {
		return false, nil
	}
	return t.scope.Selects(expression, t.local)
}

// targetNames are the element ids and paths an element context may name t by.
func (v *Validator) targetNames(t target) []string {
	var names []string
	add := func(n string) {
		if n != "" && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	add(t.node.Def.ID)
	add(t.node.Def.Path)
	if t.node.Def.Base != nil {
		add(t.node.Def.Base.Path)
	}
	add(t.path)
	typed := t.node
	if t.node.Def.ContentReference != nil {
		if ref, _ := v.registry.ContentReference(t.sd, t.node); ref != nil {
			add(ref.Def.ID)
			typed = ref
		}
	}
	code := valueTypeOf(typed, t.typeCode)
	if code == "" && typed.Parent == nil {
		code = t.sd.Type // a resource's root element is its type
	}
	for _, ancestor := range v.typeAndAncestors(code) {
		add(ancestor)
	}
	// A resource is also named by the interfaces its type implements (CanonicalResource).
	for _, iface := range v.registry.Interfaces(v.registry.GetByType(code)) {
		add(iface)
	}
	return names
}

// typeAndAncestors are a type and the types it derives from, through baseDefinition: Patient,
// DomainResource, Resource.
func (v *Validator) typeAndAncestors(code string) []string {
	var out []string
	for sd := v.registry.GetByType(code); sd != nil; {
		if root := sd.Tree().Root(); root != nil {
			out = append(out, root.Def.ID)
		}
		if sd.BaseDefinition == "" {
			break
		}
		base, _ := v.registry.ResolveCanonical(sd.BaseDefinition)
		if base == nil || base == sd {
			break
		}
		sd = base
	}
	return out
}

// valueTypeOf is the type of a value of node: typeCode, or else the element's only type.
func valueTypeOf(node *registry.ElementNode, typeCode string) string {
	if typeCode == "" && len(node.Def.Type) == 1 {
		return node.Def.Type[0].Code
	}
	return typeCode
}
