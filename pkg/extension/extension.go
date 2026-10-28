// Package extension validates FHIR extensions against their StructureDefinitions.
package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/internal/exactjson"
	"github.com/gofhir/validator/v2/internal/reslang"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/primitive"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/terminology"
	"github.com/gofhir/validator/v2/pkg/walker"
)

// Constants for commonly used string values.
const (
	keyExtension         = "extension"
	keyModifierExtension = "modifierExtension"
	strengthRequired     = "required"
	strengthExtensible   = "extensible"

	// The FHIR JSON property that names a resource's type (json.html#resources).
	resourceTypeKey = "resourceType"

	// The url key doubles as the JSON key of Extension.url and as the name of
	// the {url} placeholder in the diagnostic templates. They coincide on
	// purpose: the placeholder is named after the field it carries.
	keyURL = "url"
)

// Validator validates extensions against their StructureDefinitions.
type Validator struct {
	registry      *registry.Registry
	walker        *walker.Walker
	termRegistry  *terminology.Registry
	bindValidator BindingValidator
	primValidator *primitive.Validator
	fhirpath      FHIRPathEvaluator
}

// BindingValidator validates a value against an element binding. Implemented by
// *binding.Validator.
//
// Deprecated: the binding phase checks extension values, under their definitions; the extension
// phase no longer calls it.
type BindingValidator interface {
	ValidateValueBinding(ctx context.Context, value any, typeName string, binding *registry.Binding, fhirPath string, result *issue.Result)

	// ValidateCodedValue checks a Coding against the CodeSystem it declares, which is
	// independent of any binding — see its implementation in pkg/binding.
	ValidateCodedValue(ctx context.Context, value any, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result)
}

// New creates a new extension Validator.
func New(reg *registry.Registry, termReg *terminology.Registry, primVal *primitive.Validator) *Validator {
	return &Validator{
		registry:      reg,
		walker:        walker.New(reg),
		termRegistry:  termReg,
		primValidator: primVal,
	}
}

// SetBindingValidator supplies the validator once used for bindings on extension values.
//
// Deprecated: the binding phase checks extension values, under their definitions (see
// binding.Validator.SetDefinitions); the extension phase no longer uses it.
func (v *Validator) SetBindingValidator(b BindingValidator) {
	v.bindValidator = b
}

// Validate validates all extensions in a resource.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(ctx context.Context, resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Type == "" {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateDataWith(ctx, Data{Resource: resource, Exact: exactjson.New(resource, resourceData).Of, Raw: resourceData}, sd, result)
}

// checkResourceContexts checks the contexts of use in d's resource.
func (v *Validator) checkResourceContexts(ctx context.Context, d Data, resourceType string, result *issue.Result) {
	in := place{root: ScopeRoot{Raw: d.Raw, Bundle: d.Bundle, Outer: d.Outer, Exact: d.Exact}, exact: d.Exact}
	if d.Container != nil && d.At != "" && v.fhirpath != nil {
		container := v.fhirpath.Scope(ctx, ScopeRoot{Resource: d.Container, Bundle: d.Bundle, Outer: d.Outer, Exact: d.Exact})
		in = place{local: d.At, parent: container, contained: true, exact: d.Exact}
	}
	v.checkContexts(ctx, d.Resource, resourceType, in, result)
}

// ValidateData validates all extensions in a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing. The
// FHIRPath expressions of contexts of use read resource's numbers as it holds them; ValidateDataWith
// takes the resource as the JSON spells them too.
func (v *Validator) ValidateData(ctx context.Context, resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	v.ValidateDataWith(ctx, Data{Resource: resource}, sd, result)
}

// Data is a resource whose extensions are validated, and where it is.
type Data struct {
	// Resource is the resource, as parsed.
	Resource map[string]any
	// Exact returns an object of Resource (or of Bundle or Container) decoded with its numbers as the
	// JSON spells them (json.Decoder.UseNumber), or nil when it has none: the FHIRPath expressions
	// of contexts of use read it, so that a decimal keeps its precision (1.50 is not 1.5,
	// json.html#primitive). It is asked only when an expression is evaluated. Nil reads Resource.
	Exact func(map[string]any) map[string]any
	// Raw is the JSON Resource was parsed from, when it is the resource validated: the expressions
	// read it once, and the resources it holds as nodes of it. Nil reads Resource, as Exact gives it.
	Raw []byte
	// Bundle is the Bundle being validated, which resolve() finds references in, when Resource is
	// in one; a Bundle validated is its own.
	Bundle map[string]any
	// Outer are the Bundles that hold Bundle, the innermost first, where resolve() looks after it.
	Outer []map[string]any
	// Container is the resource that contains Resource, and At where Resource is in it
	// ("Observation.contained[0]"), when Resource is a contained resource validated on its own (a
	// slice's conformance check): its %rootResource.
	Container map[string]any
	At        string
}

// ValidateDataWith validates all extensions in d's resource.
func (v *Validator) ValidateDataWith(ctx context.Context, d Data, sd *registry.StructureDefinition, result *issue.Result) {
	resource := d.Resource
	if sd == nil || sd.Type == "" {
		return
	}

	resourceType := sd.RootName(resource)
	if resourceType == "" {
		return
	}

	// Each extension's context of use, in this resource and the resources it holds.
	v.checkResourceContexts(ctx, d, resourceType, result)

	// Validate extensions at root level and recursively, a coded value's display in the
	// resource's language, over the definitions: each resource it holds is its own.
	ctx = reslang.With(ctx, resource)
	v.validateResource(ctx, v.registry.GetByType(resourceType), resource, resourceType, result)
	languages := reslang.NewWalk(ctx, resourceType)

	// Each resource it holds, at any depth (contained resources, Bundle entries, the resources
	// other elements hold), in its language or else its holder's.
	v.walker.WalkAllContext(ctx, resource, resourceType, resourceType, func(wctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if wctx.FHIRPath == resourceType {
			return true
		}
		v.validateResource(languages.Context(ctx, wctx), wctx.SD, wctx.Data, wctx.FHIRPath, result)
		return true
	})
}

// validateResource validates the extensions of data, a resource of definition sd at path, and of
// its elements, following the definitions.
func (v *Validator) validateResource(ctx context.Context, sd *registry.StructureDefinition, data map[string]any, path string, result *issue.Result) {
	if sd == nil {
		return
	}
	if root := sd.Tree().Root(); root != nil {
		v.validateNode(ctx, sd, root, "", data, path, result)
	}
}

// validateNode validates the extensions of inst, an instance of node at path, then those of the
// values it holds for node's children, following the definitions (registry.ChildrenOf: a backbone's
// children inline, a contentReference's target, a datatype's own definition), a primitive's in its
// "_key" sibling. It does not go into a resource an element holds (the walk visits it), a value
// written as an array where one goes (HL7 validates nothing in it), nor an element the definitions
// do not have (the structural phase reports it), nor a primitive written otherwise than as a
// primitive.
func (v *Validator) validateNode(ctx context.Context, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, inst map[string]any, path string, result *issue.Result) {
	children := v.registry.ChildrenOf(ctx, sd, node, typeCode, nil)
	if children.Unresolved.Canonical != "" {
		// El perfil que declara su tipo no se puede usar (otra fase lo informa): el valor sigue siendo
		// de su tipo, cuya definición lo gobierna, y HL7 lo valida así (#178).
		children = v.registry.TypeChildrenOf(node, typeCode)
	}
	v.validateChildren(ctx, children, inst, path, result)
}

// validateChildren valida lo que children declara como extensión en inst, una instancia en path, y
// después lo que tienen adentro los valores de cada uno de los otros hijos.
func (v *Validator) validateChildren(ctx context.Context, children registry.InstanceChildren, inst map[string]any, path string, result *issue.Result) {
	v.validateDeclaredExtensions(ctx, children, inst, path, result)
	for _, child := range children.Nodes {
		if name := child.Name(); name == keyExtension || name == keyModifierExtension || v.registry.HoldsResource(child.Def) {
			continue
		}
		for _, cv := range elementvalues.Of(child, inst, v.registry) {
			if !elementvalues.Written(child, cv, v.registry) {
				// Not written as the element says (an array where one value goes, an object for a
				// primitive, a single null): the structural phase reports it, and HL7 drops it.
				continue
			}
			if cv.TypeCode != "" && v.registry.IsPrimitiveTypeCode(cv.TypeCode) {
				// A primitive's extensions are in its "_key" sibling (json.html#primitive), an instance
				// of its type; a single value written where an array goes is not checked, as HL7 does
				// not.
				if cv.Ext != nil && elementvalues.ShapeOf(child, cv) == elementvalues.RightShape {
					sibling := path + "._" + cv.Key
					if cv.Array {
						sibling = fmt.Sprintf("%s[%d]", sibling, cv.Index)
					}
					v.validateNode(ctx, children.SD, child, cv.TypeCode, cv.Ext, sibling, result)
				}
				continue
			}
			if m, ok := cv.Value.(map[string]any); ok {
				// A complex element's "_key" sibling is no property of it (HL7: unrecognized).
				v.validateNode(ctx, children.SD, child, cv.TypeCode, m, elementvalues.ItemPath(child, cv, path), result)
			}
		}
	}
}

// validateExtensionArray validates an array of extensions.
func (v *Validator) validateExtensionArray(ctx context.Context, extensions any, basePath string, isModifier bool, result *issue.Result) {
	extArray, ok := extensions.([]any)
	if !ok {
		return
	}

	for i, ext := range extArray {
		extMap, ok := ext.(map[string]any)
		if !ok {
			continue
		}

		extPath := fmt.Sprintf("%s[%d]", basePath, i)
		v.validateSingleExtension(ctx, extMap, extPath, isModifier, result)
	}
}

// absoluteURLDefect names why an Extension.url is not an absolute URL, or ""
// when it is. The reason travels into the diagnostic, because a message that
// lists every possible cause misnames three of them each time it fires.
//
// The rule is FHIR R4 §2.5.0.1:
//
//	"The url SHALL be a URL, not a URN (e.g. not an OID or a UUID), and it SHALL
//	 be the canonical URL of a StructureDefinition that defines the extension."
//	"Except for child extensions defined within complex extensions, the URL SHALL
//	 be an absolute URL."
//
// The bar is an absolute URL, not merely an absolute URI: `urn:uuid:…` is a
// perfectly valid absolute URI under RFC 3986 and is exactly the case the
// sentence above names to exclude.
//
// Three things have to hold, and testing for "://" alone covers none of them
// properly — `urn:uuid://x` contains it and is still a URN:
//
//  1. a scheme, spelled as RFC 3986 §3.1 requires;
//  2. that scheme is not `urn`;
//  3. a hierarchical part, i.e. `//` after the colon. This is what separates a
//     URL from an opaque URI: `ex:createdAt` has a scheme and is not a URL.
//
// The scheme itself is deliberately not restricted to http(s): the
// specification asks for a URL, not for a resolvable one, and an unresolvable
// extension is a separate (warning-level) matter — see docs/VALIDATION-GAPS.md.
//
// Child extensions inside a complex extension are the documented exception and
// never reach this function: they are resolved by name against the parent's
// definition in validateNestedExtensions.
func absoluteURLDefect(url string) string {
	scheme, rest, found := strings.Cut(url, ":")
	switch {
	case !found, !isValidURIScheme(scheme):
		// Both are the same defect seen from two angles. Per RFC 3986 a colon
		// only delimits a scheme when what precedes it is spelled like one, so
		// `StructureDefinition/my:ext` has no scheme either — calling that an
		// invalid scheme would name a cause its author never wrote.
		return "no scheme, so this is a relative reference"
	case strings.EqualFold(scheme, "urn"):
		return "a URN is not a URL"
	case !strings.HasPrefix(rest, "//"):
		return "an opaque URI is not a URL"
	default:
		return ""
	}
}

// isValidURIScheme applies RFC 3986 §3.1: ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ).
func isValidURIScheme(scheme string) bool {
	if scheme == "" {
		return false
	}
	for i := 0; i < len(scheme); i++ {
		c := scheme[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return true
}

// validateSingleExtension validates a single extension.
// Per FHIR R4 §2.1.0.1, unknown modifier extensions produce an ERROR (not a warning),
// because a system SHALL refuse to process a resource with an unrecognized modifier extension.
func (v *Validator) validateSingleExtension(ctx context.Context, ext map[string]any, extPath string, isModifier bool, result *issue.Result) {
	// Get extension URL
	url, ok := ext[keyURL].(string)
	if !ok || url == "" {
		result.AddErrorWithID(
			issue.DiagExtensionNoURL,
			nil,
			extPath,
		)
		return
	}

	// Validate that the URL is an absolute URL (FHIR R4 §2.5.0.1). The rule lives
	// in the prose specification and is not expressible via the
	// StructureDefinition: Extension.url is typed as System.String, with no regex
	// or constraint carrying it.
	if defect := absoluteURLDefect(url); defect != "" {
		result.AddErrorWithID(
			issue.DiagExtensionInvalidURL,
			map[string]any{keyURL: url, "reason": defect},
			extPath,
		)
		return
	}

	// Resolve extension StructureDefinition.
	// Use ResolveByCanonical (not GetByURL) so the ProfileResolver fallback runs —
	// this enables on-demand loading of SDs from external sources (DB, IG packages).
	extSD := v.registry.ResolveByCanonical(ctx, url, "")
	if extSD == nil {
		params := map[string]any{keyURL: url}
		if isModifier {
			result.AddErrorWithID(issue.DiagModifierExtensionUnknown, params, extPath)
		} else {
			result.AddWarningWithID(issue.DiagExtensionUnknown, params, extPath)
		}
		// Can't validate further without SD
		return
	}

	if root := extSD.Tree().Root(); root != nil {
		v.validateExtension(ctx, extSD, root, ext, extPath, result)
	}
}

// DefinitionOf returns the definition a value declares for itself, which governs it wherever it is
// used (implements constraint.DefinitionSource): the one an extension's url names, which the
// extension conforms to (extensibility.html). A value declares one when its url names a
// StructureDefinition of the value's own type, typeCode, that fixes the url to itself, as an
// extension's definition fixes Extension.url; no other url does: an Attachment's url is where its
// data is, even when it names a profile of Attachment, and a canonical resource's names one of
// another type.
//
// The lookup is in memory: this phase resolves an extension's url, through the external resolver
// too, before it asks, and the registry keeps what it resolves. A relative url (a sub-extension's)
// names none.
func (v *Validator) DefinitionOf(ctx context.Context, typeCode string, value map[string]any) *registry.StructureDefinition {
	url, _ := value[keyURL].(string)
	if url == "" || typeCode == "" {
		return nil
	}
	sd, _ := v.registry.ResolveCanonical(url)
	if sd == nil || sd.Type != typeCode {
		return nil
	}
	if err := v.registry.EnsureSnapshot(ctx, sd); err != nil || !fixesURL(sd, url) {
		return nil
	}
	return sd
}

// fixesURL reports whether sd's snapshot fixes its type's url element to url.
func fixesURL(sd *registry.StructureDefinition, url string) bool {
	path := sd.Type + "." + keyURL
	for i := range sd.Snapshot.Element {
		e := &sd.Snapshot.Element[i]
		if e.Path != path || e.SliceName != nil {
			continue
		}
		raw, _, ok := e.GetFixed()
		var fixed string
		return ok && json.Unmarshal(raw, &fixed) == nil && fixed == url
	}
	return false
}

// validateDeclaredExtensions valida cada extensión de inst, solo donde la definición la tiene: un
// modifierExtension en un tipo que no lo tiene es un elemento desconocido, que informa la fase
// estructural, y HL7 informa solo eso (#178).
func (v *Validator) validateDeclaredExtensions(ctx context.Context, children registry.InstanceChildren, inst map[string]any, path string, result *issue.Result) {
	for _, child := range children.Nodes {
		if name := child.Name(); name == keyExtension || name == keyModifierExtension {
			if extensions, ok := inst[name]; ok {
				v.validateExtensionArray(ctx, extensions, path+"."+name, name == keyModifierExtension, result)
			}
		}
	}
}

// validateExtension valida ext, una instancia en extPath de node en sd (la raíz de la definición de
// una extensión, o la slice de una subextensión que declara): su valor y sus subextensiones, cada uno
// contra el elemento de node que lo gobierna. El valor es el elemento de elección de node, el de
// nombre terminado en "[x]" (elementdefinition.html#id): Extension tiene uno, su value[x] (#178).
func (v *Validator) validateExtension(ctx context.Context, sd *registry.StructureDefinition, node *registry.ElementNode, ext map[string]any, extPath string, result *issue.Result) {
	children := v.registry.ChildrenOf(ctx, sd, node, "", nil)
	var nested *registry.ElementNode
	for _, child := range children.Nodes {
		switch name := child.Name(); {
		case strings.HasSuffix(name, "[x]"):
			v.validateExtensionValue(ctx, children.SD, child, ext, sd.URL, extPath, result)
		case name == keyExtension:
			nested = child
		}
	}
	if nestedExts, ok := ext[keyExtension]; ok {
		v.validateNestedExtensions(ctx, children.SD, nested, nestedExts, sd.URL, extPath, result)
	}
}

// validateExtensionValue valida los valores que ext tiene para valueNode, su elemento value[x] en sd:
// si puede o debe tener uno, el tipo de cada uno y lo que cada uno tiene adentro (extensibility.html:
// el valor es un elemento como cualquier otro). El parámetro url nombra la definición de la extensión
// en los mensajes.
func (v *Validator) validateExtensionValue(ctx context.Context, sd *registry.StructureDefinition, valueNode *registry.ElementNode, ext map[string]any, url, extPath string, result *issue.Result) {
	// Los valores de este elemento son los de un tipo que permite su elemento en la definición del tipo
	// de la extensión (Extension, en la versión cargada). Una clave de otro tipo (valueMeta en R4B) es
	// una propiedad desconocida, que informa la fase estructural, como HL7 (#178).
	baseTypes := v.baseTypes(sd, valueNode)
	values := elementvalues.Of(valueNode, ext, v.registry)
	values = slices.DeleteFunc(values, func(cv elementvalues.Value) bool {
		return baseTypes != nil && !declares(baseTypes, cv.TypeCode)
	})
	// Un valor es uno escrito como dice su elemento: un null o un array no lo son, y HL7 informa que
	// falta (json.html). Un primitivo presente solo por su "_key" sí lo es (json.html#primitive).
	written := 0
	for _, cv := range values {
		if elementvalues.Written(valueNode, cv, v.registry) {
			written++
		}
	}
	if valueNode.Def.Max == "0" {
		// Una extensión compleja no tiene valor.
		if written > 0 {
			result.AddErrorWithID(issue.DiagExtensionValueNotAllowed, map[string]any{keyURL: url}, extPath)
		}
		return
	}
	if valueNode.Def.Min > 0 && written == 0 && ext[keyExtension] == nil {
		result.AddErrorWithID(issue.DiagExtensionValueRequired, map[string]any{keyURL: url}, extPath)
		return
	}
	// Cada valor, en el orden de elementvalues. Más de uno ya lo informan la cardinalidad y la
	// exclusión mutua; lo que se valida de cada uno no depende de los otros (#174).
	for _, cv := range values {
		if !elementvalues.Written(valueNode, cv, v.registry) {
			continue // lo informa la fase estructural
		}
		allowed := declares(valueNode.Def, cv.TypeCode)
		if !allowed {
			// El tipo es el código que nombra la clave (json.html#choice), como lo informa HL7.
			result.AddErrorWithID(
				issue.DiagExtensionInvalidValueType,
				map[string]any{
					keyURL:     url,
					"provided": cv.TypeCode,
					"allowed":  v.allowedTypesString(valueNode.Def.Type),
				},
				extPath+"."+cv.Key,
			)
		}
		// Lo que tiene adentro: un complejo, en su objeto; un primitivo, en su "_key".
		inst, at := cv.Ext, extPath+"._"+cv.Key
		if m, ok := cv.Value.(map[string]any); ok {
			inst, at = m, extPath+"."+cv.Key
		}
		if inst == nil {
			continue
		}
		if allowed {
			v.validateNode(ctx, sd, valueNode, cv.TypeCode, inst, at, result)
			continue
		}
		// Un valor de un tipo que el elemento no permite es de ese tipo, no del que el elemento
		// restringe: se valida con la definición del tipo, y HL7 informa lo que tiene adentro (#178).
		v.validateChildren(ctx, v.registry.TypeChildrenOf(valueNode, cv.TypeCode), inst, at, result)
	}
}

// baseTypes devuelve el elemento que corresponde a valueNode en la definición del tipo de sd, cuyos
// tipos son los que una clave puede nombrar, o nil si no está.
func (v *Validator) baseTypes(sd *registry.StructureDefinition, valueNode *registry.ElementNode) *registry.ElementDefinition {
	base := v.registry.GetByType(sd.Type)
	if base == nil {
		return nil
	}
	root := base.Tree().Root()
	if root == nil {
		return nil
	}
	for _, child := range root.Children {
		if child.Name() == valueNode.Name() {
			return child.Def
		}
	}
	return nil
}

// declares dice si def permite el tipo code, exacto: el tipo que nombra la clave de un elemento de
// elección es su código (json.html#choice).
func declares(def *registry.ElementDefinition, code string) bool {
	for _, t := range def.Type {
		if t.Code == code {
			return true
		}
	}
	return false
}

// allowedTypesString returns a comma-separated list of allowed types.
func (v *Validator) allowedTypesString(types []registry.Type) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Code
	}
	return strings.Join(names, ", ")
}

// validateNestedExtensions valida cada extensión dentro de una extensión compleja; nested es su
// elemento Extension.extension en sd (nil si la definición no lo tiene). Una url relativa nombra una
// parte de la definición del padre (extensibility.html: "the identity of the parts of the extension
// are local/relative to the reference to the extension definition"): tiene que ser una que la
// definición declara, como exige HL7 (Extension_EXT_SubExtension_Invalid), y se valida contra esa
// slice, con sus propias subextensiones. Una url absoluta nombra una extensión definida aparte, que el
// slicing abierto de Extension.extension permite: una que el padre no declara se valida como cualquier
// extensión, contra su propia definición. El parámetro parentURL nombra la definición del padre.
func (v *Validator) validateNestedExtensions(ctx context.Context, sd *registry.StructureDefinition, nested *registry.ElementNode, nestedExts any, parentURL, parentPath string, result *issue.Result) {
	extArray, ok := nestedExts.([]any)
	if !ok {
		return
	}

	for i, ext := range extArray {
		extMap, ok := ext.(map[string]any)
		if !ok {
			continue
		}

		extPath := fmt.Sprintf("%s.extension[%d]", parentPath, i)
		url, _ := extMap[keyURL].(string)

		if slice := declaredSlice(nested, url); slice != nil {
			v.validateExtension(ctx, sd, slice, extMap, extPath, result)
			continue
		}
		if url != "" && absoluteURLDefect(url) == "" {
			v.validateSingleExtension(ctx, extMap, extPath, false, result)
			continue
		}
		result.AddErrorWithID(
			issue.DiagExtensionSubExtensionInvalid,
			map[string]any{keyURL: url, "parent": parentURL},
			extPath,
		)
	}
}

// declaredSlice devuelve la slice de nested, un elemento Extension.extension, cuyo elemento url está
// fijado a url, o nil.
func declaredSlice(nested *registry.ElementNode, url string) *registry.ElementNode {
	if nested == nil || url == "" {
		return nil
	}
	for _, slice := range nested.Slices {
		for _, child := range slice.Children {
			if child.Name() != keyURL {
				continue
			}
			fixed, _, ok := child.Def.GetFixed()
			var fixedURI string
			if ok && json.Unmarshal(fixed, &fixedURI) == nil && fixedURI == url {
				return slice
			}
		}
	}
	return nil
}
